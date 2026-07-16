package reviewagent

import (
	"fmt"
	"strings"
)

// ─── Verdict System (v4.0) ─────────────────────────────────────────────────

// Verdict is the pass/reject decision from the Review Agent.
type Verdict string

const (
	VerdictPass   Verdict = "PASS"
	VerdictReject Verdict = "REJECT"
)

// VerdictResult is the structured output of the chapter review.
// It includes the verdict, score, issue list, and rewrite guidance.
type VerdictResult struct {
	Verdict       Verdict     `json:"verdict"`         // PASS or REJECT
	Score         int         `json:"score"`           // 0-100, ≥80 = PASS
	Issues        []VerdictIssue `json:"issues"`        // problems found
	AutoFixable   bool        `json:"auto_fixable"`    // can the review agent fix this itself?
	RequireRewrite bool       `json:"require_rewrite"` // must go back to novel writer?
	Round         int         `json:"round"`           // which review round (1-3)
	Summary       string      `json:"summary"`         // one-line summary
}

// VerdictIssue is one problem found during review.
type VerdictIssue struct {
	Severity       string `json:"severity"`         // must_fix | suggest
	Category       string `json:"category"`         // 字数不足 | 流畅度 | 剧情遗漏 | AI痕迹 | 人设冲突 | 逻辑矛盾
	Location       string `json:"location"`         // paragraph or section reference
	Description    string `json:"description"`      // what's wrong
	Suggestion     string `json:"suggestion"`       // how to fix
	ReferenceScene string `json:"reference_scene"`  // original scene data for context
	DeductPoints   int    `json:"deduct_points"`    // points deducted from 100
}

// GenreWordLimits defines minimum word counts per genre.
var GenreWordLimits = map[string]int{
	"玄幻": 2000,
	"古言": 2000,
	"都市": 1800,
	"甜宠": 1800,
	"科幻": 1500,
	"悬疑": 1500,
}

// Evaluate runs the full review verdict on a chapter draft.
// It checks: word count, fluency heuristics, forbidden phrases,
// and computes a composite score.
func Evaluate(content string, genre string, round int) *VerdictResult {
	v := &VerdictResult{
		Verdict: VerdictPass,
		Score:   100,
		Round:   round,
	}

	runes := []rune(content)
	wordCount := len(runes)

	// 1. Word count check
	minWords := 1500
	if w, ok := GenreWordLimits[genre]; ok {
		minWords = w
	}
	if wordCount < minWords {
		deduct := 20
		v.Score -= deduct
		v.Issues = append(v.Issues, VerdictIssue{
			Severity:    "must_fix",
			Category:    "字数不足",
			Location:    "全文",
			Description: fmt.Sprintf("当前%d字，赛道最低要求%d字，不足%d字", wordCount, minWords, minWords-wordCount),
			Suggestion:  fmt.Sprintf("需扩充至少%d字。建议增加场景环境描写或角色内心活动。", minWords-wordCount),
			DeductPoints: deduct,
		})
	}

	// 2. Fluency heuristics (simple pattern checks)
	fluencyDeduct := checkFluency(content)
	v.Score -= fluencyDeduct

	// 3. Forbidden phrase scan
	forbidDeduct, forbidIssues := checkForbidPhrases(content)
	v.Score -= forbidDeduct
	v.Issues = append(v.Issues, forbidIssues...)

	// 4. AI pattern heuristics
	aiDeduct, aiIssues := checkAIPatternsQuick(content)
	v.Score -= aiDeduct
	v.Issues = append(v.Issues, aiIssues...)

	// Clamp score
	if v.Score < 0 {
		v.Score = 0
	}

	// Determine verdict
	if v.Score >= 80 && !hasMustFix(v.Issues) {
		v.Verdict = VerdictPass
		v.Summary = fmt.Sprintf("审核通过。综合评分%d分，字数%d，问题%d条。", v.Score, wordCount, len(v.Issues))
	} else {
		v.Verdict = VerdictReject
		v.RequireRewrite = hasMustFix(v.Issues)
		v.AutoFixable = !v.RequireRewrite

		// Round cap: force pass on 3rd round
		if round >= 3 {
			v.Verdict = VerdictPass
			v.Summary = fmt.Sprintf("已达最大修改轮次(3轮)，强制通过。综合评分%d分。剩余问题%d条需审核Agent自行修复。", v.Score, len(v.Issues))
			v.AutoFixable = true
			v.RequireRewrite = false
		} else {
			v.Summary = fmt.Sprintf("打回修改(第%d轮)。综合评分%d分，必须修复%d条。", round, v.Score, countMustFix(v.Issues))
		}
	}

	return v
}

func hasMustFix(issues []VerdictIssue) bool {
	for _, i := range issues {
		if i.Severity == "must_fix" {
			return true
		}
	}
	return false
}

func countMustFix(issues []VerdictIssue) int {
	c := 0
	for _, i := range issues {
		if i.Severity == "must_fix" {
			c++
		}
	}
	return c
}

// ─── Quick Heuristic Checks ─────────────────────────────────────────────────

func checkFluency(content string) int {
	deduct := 0
	lines := strings.Split(content, "\n")

	// Check for 3+ consecutive pure-dialogue lines
	dialogueStreak := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "「") || strings.HasPrefix(line, "\"") || strings.HasPrefix(line, "「") {
			dialogueStreak++
			if dialogueStreak >= 3 {
				deduct += 5
				break
			}
		} else {
			dialogueStreak = 0
		}
	}

	// Check for 3+ consecutive non-dialogue paragraphs (narration heavy)
	narrationStreak := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "「") || strings.HasPrefix(line, "\"") {
			narrationStreak = 0
		} else {
			narrationStreak++
			if narrationStreak >= 4 {
				deduct += 3
				break
			}
		}
	}

	if deduct > 10 {
		deduct = 10
	}
	return deduct
}

func checkForbidPhrases(content string) (int, []VerdictIssue) {
	forbid := []string{"与此同时", "值得一提的是", "不可否认的是", "总而言之"}
	var issues []VerdictIssue
	deduct := 0

	// Count occurrences more efficiently
	lower := strings.ToLower(content)
	for _, phrase := range forbid {
		count := strings.Count(lower, strings.ToLower(phrase))
		if count > 0 {
			deduct += count * 3
			issues = append(issues, VerdictIssue{
				Severity:    "must_fix",
				Category:    "AI痕迹",
				Location:    "全文多处",
				Description: fmt.Sprintf("检测到AI模板用语\"%s\"出现%d次", phrase, count),
				Suggestion:  fmt.Sprintf("将\"%s\"替换为具体动作、环境描写或直接删除。", phrase),
				DeductPoints: count * 3,
			})
		}
	}

	if deduct > 15 {
		deduct = 15
	}
	return deduct, issues
}

func checkAIPatternsQuick(content string) (int, []VerdictIssue) {
	var issues []VerdictIssue
	deduct := 0

	patterns := map[string]struct {
		keyword string
		desc    string
		sug     string
	}{
		"心中涌起":  {"心中涌起", "空洞情绪模板", "替换为具体生理反应+行为描述"},
		"难以言喻":  {"难以言喻", "万能情绪形容词", "使用具体比喻替代抽象形容"},
		"既…又…":  {"既", "对称句式（需结合上下文判断）", "打破对偶，改为长短句交替"},
		"这一刻":   {"这一刻", "公式化收束句", "用画面或动作结尾替代总结"},
		"先…然后…接着": {"然后", "规整动作链", "打断顺序，插入环境/心理描写"},
		"淡淡道":   {"淡淡道", "对话标签重复", "用动作描写替代标签"},
	}

	for name, p := range patterns {
		count := strings.Count(strings.ToLower(content), strings.ToLower(p.keyword))
		if count > 1 {
			deduct += count
			issues = append(issues, VerdictIssue{
				Severity:    "suggest",
				Category:    "AI痕迹",
				Location:    "全文",
				Description: fmt.Sprintf("疑似\"%s\"(%s)出现%d次", name, p.desc, count),
				Suggestion:  p.sug,
				DeductPoints: count,
			})
		}
	}

	if deduct > 15 {
		deduct = 15
	}
	return deduct, issues
}

// ─── Verdict Prompt Builder ─────────────────────────────────────────────────

const verdictSystemPrompt = `【角色锁定】你是网文审核Agent(Verdict模式)。你必须给出明确的PASS/REJECT判决。
判决标准:
- 字数<赛道最低要求 → 强制REJECT
- AI模板用语≥3处 → REJECT
- 综合评分<80 → REJECT
- 最高3轮修改，第3轮强制PASS`

// BuildVerdictPrompt builds the full verdict review prompt for LLM usage.
func BuildVerdictPrompt(content string, genre string, chapterNum int, round int) string {
	var b strings.Builder
	b.WriteString(verdictSystemPrompt)
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("## 审核参数\n- 章节: 第%d章\n- 赛道: %s\n- 轮次: 第%d轮\n", chapterNum, genre, round))

	minWords := GenreWordLimits[genre]
	if minWords == 0 {
		minWords = 1500
	}
	b.WriteString(fmt.Sprintf("- 最低字数: %d字\n", minWords))

	b.WriteString("\n## 审核清单\n")
	b.WriteString("1. 字数达标（当前字数 vs 最低要求）\n")
	b.WriteString("2. 流畅度（对话/叙述比例、段落节奏）\n")
	b.WriteString("3. AI痕迹（万能过渡句、对称句式、空洞情绪等8类）\n")
	b.WriteString("4. 人设与逻辑一致性\n\n")

	b.WriteString("## 输出格式（JSON）\n```json\n")
	b.WriteString(`{
  "verdict": "PASS|REJECT",
  "score": 85,
  "issues": [
    {
      "severity": "must_fix|suggest",
      "category": "字数不足|流畅度|剧情遗漏|AI痕迹|人设冲突|逻辑矛盾",
      "location": "第X段",
      "description": "问题描述",
      "suggestion": "修改建议"
    }
  ],
  "auto_fixable": true,
  "require_rewrite": true
}
`)
	b.WriteString("```\n\n")
	b.WriteString(fmt.Sprintf("## 正文（第%d章）\n\n", chapterNum))
	b.WriteString(content)

	return b.String()
}
