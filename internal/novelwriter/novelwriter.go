// Package novelwriter provides the Novel Writer Agent — the third stage in
// the v4.0 four-stage pipeline. It operates in two modes:
//
//   Generate mode (成文模式) — Takes the structured scene description from
//   the Recorder Agent and transforms it into fluent web-novel prose.
//   Injects character-specific dialogue styles, micro-expressions, 4-beat
//   rhythm control, genre-specific elements, and avoids AI-template phrasing.
//
//   Rewrite mode (修改模式) — Activated when the Review Agent rejects a
//   chapter draft. The Novel Writer Agent receives a structured issue list
//   and rewrites the chapter accordingly. During rewrite, it may query the
//   Recorder Agent (in consultant mode) for original deduction data to
//   ensure the rewrite stays true to the original scene intent.
//
// Design invariant: the Novel Writer Agent is responsible for ALL prose
// generation and editing. No other agent writes narrative text.
package novelwriter

import (
	"fmt"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/recorderagent"
)

// ─── Genre ──────────────────────────────────────────────────────────────────

// Genre is a web-novel track/category.
type Genre string

const (
	GenreXuanhuan  Genre = "玄幻"
	GenreDushi     Genre = "都市"
	GenreGuyan     Genre = "古言"
	GenreKehuan    Genre = "科幻"
	GenreTianchong Genre = "甜宠"
	GenreXuanyi    Genre = "悬疑"
)

// GenreSpec holds genre-specific writing parameters.
type GenreSpec struct {
	MinWords     int      // minimum words per chapter
	MaxWords     int      // maximum words per chapter
	StyleNotes   []string // genre-specific writing tips
	ForbidPhrases []string // phrases banned for this genre
}

// GenreSpecs maps each genre to its spec.
var GenreSpecs = map[Genre]GenreSpec{
	GenreXuanhuan: {
		MinWords: 2000, MaxWords: 5000,
		StyleNotes:   []string{"力量体系术语要统一", "战斗场景详写关键回合", "境界突破要有仪式感"},
		ForbidPhrases: []string{"与此同时", "值得一提的是"},
	},
	GenreDushi: {
		MinWords: 1800, MaxWords: 4000,
		StyleNotes:   []string{"行业细节准确", "人物关系复杂度高", "装逼打脸节奏快"},
		ForbidPhrases: []string{"值得一提的是", "不可否认的是"},
	},
	GenreGuyan: {
		MinWords: 2000, MaxWords: 4500,
		StyleNotes:   []string{"礼仪规范准确", "权谋逻辑严密", "对话潜台词丰富"},
		ForbidPhrases: []string{"总而言之", "与此同时"},
	},
	GenreKehuan: {
		MinWords: 1500, MaxWords: 4000,
		StyleNotes:   []string{"科学设定逻辑自洽", "世界观展开要有层次", "避免过度解释技术细节"},
		ForbidPhrases: []string{"值得一提的是"},
	},
	GenreTianchong: {
		MinWords: 1800, MaxWords: 3500,
		StyleNotes:   []string{"情感描写细腻", "互动甜而不腻", "冲突适度不狗血"},
		ForbidPhrases: []string{"与此同时", "总而言之"},
	},
	GenreXuanyi: {
		MinWords: 1500, MaxWords: 4000,
		StyleNotes:   []string{"氛围营造优先", "线索埋设隐蔽", "恐怖感渐进式"},
		ForbidPhrases: []string{"总而言之", "不可否认的是"},
	},
}

// ─── Agent ──────────────────────────────────────────────────────────────────

// Agent is the Novel Writer Agent.
type Agent struct {
	genre       Genre
	spec        GenreSpec
	styleGuide  string // optional additional style preferences from user
}

// New creates a Novel Writer Agent for the given genre.
func New(genre string) *Agent {
	g := Genre(genre)
	spec, ok := GenreSpecs[g]
	if !ok {
		g = GenreXuanhuan
		spec = GenreSpecs[g]
	}
	return &Agent{genre: g, spec: spec}
}

// SetStyleGuide sets optional user-provided style preferences.
func (a *Agent) SetStyleGuide(guide string) { a.styleGuide = guide }

// TargetWordRange returns the (min, max) word count for this agent's genre.
func (a *Agent) TargetWordRange() (int, int) { return a.spec.MinWords, a.spec.MaxWords }

// ─── Chapter Output ─────────────────────────────────────────────────────────

// Chapter is the output of the generate (or rewrite) mode.
type Chapter struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`       // full chapter prose
	WordCount int       `json:"word_count"` // Chinese character count
	Genre     Genre     `json:"genre"`
	Rhythm    string    `json:"rhythm"`     // which rhythm phases were used
	GeneratedAt time.Time `json:"generated_at"`
	Round     int       `json:"round"`      // 0=first draft, 1=first rewrite, etc.
}

// ─── Generate Mode ──────────────────────────────────────────────────────────

// GenerateInput is what the Novel Writer needs to produce a chapter.
type GenerateInput struct {
	ChapterNum int
	Scene      *recorderagent.Scene   // structured scene from Recorder Agent
	Profiles   map[string]*characteragent.CharacterProfile // character speech styles etc.
	Round      int                    // 0 = first draft
}

// Generate produces a chapter from the structured scene.
// It first builds a prompt, then the LLM fills in the body.
func (a *Agent) BuildGeneratePrompt(in GenerateInput) string {
	var b strings.Builder

	// System prompt: role lock
	b.WriteString(fmt.Sprintf("【角色锁定】你是专业的%s网文作家。\n", a.genre))
	b.WriteString("你的任务是将结构化场景描述转化为流畅的网文章节正文。\n\n")

	// Genre-specific notes
	b.WriteString("## 赛道规范\n")
	for _, note := range a.spec.StyleNotes {
		b.WriteString(fmt.Sprintf("- %s\n", note))
	}
	b.WriteString("\n")

	// Forbid phrases
	b.WriteString("## 禁用表达（AI痕迹）\n")
	b.WriteString("以下短语绝不出现在正文中：\n")
	for _, phrase := range a.spec.ForbidPhrases {
		b.WriteString(fmt.Sprintf("- \"%s\"\n", phrase))
	}
	b.WriteString("- 万能过渡句、对称句式、空洞情绪描述\n\n")

	// Style guide override
	if a.styleGuide != "" {
		b.WriteString(fmt.Sprintf("## 用户风格要求\n%s\n\n", a.styleGuide))
	}

	// Rhythm instructions
	b.WriteString("## 四拍节奏控制\n")
	b.WriteString("1. 铺垫（约20%）: 环境描写+人物位置+当前局势\n")
	b.WriteString("2. 冲突升级（约30%）: 短句+动作链+张力拉高\n")
	b.WriteString("3. 高潮/爽点（约30%）: 关键细节放大+慢镜头感+情绪爆发\n")
	b.WriteString("4. 悬念收尾（约20%）: 留白+钩子+戛然而止\n\n")

	// Character speech styles
	if len(in.Profiles) > 0 {
		b.WriteString("## 角色说话风格\n")
		for _, p := range in.Profiles {
			if p.SpeechStyle != "" {
				b.WriteString(fmt.Sprintf("- %s（%s）: %s\n", p.Name, p.Role, p.SpeechStyle))
			}
			if len(p.Habits) > 0 {
				b.WriteString(fmt.Sprintf("  习惯动作: %s\n", strings.Join(p.Habits, "、")))
			}
		}
		b.WriteString("\n")
	}

	// Scene data
	b.WriteString("## 场景数据\n")
	if in.Scene != nil {
		b.WriteString(fmt.Sprintf("场景: %s\n", in.Scene.Title))
		b.WriteString(fmt.Sprintf("地点: %s\n", in.Scene.Location))
		b.WriteString(fmt.Sprintf("第%d章\n\n", in.ChapterNum))

		b.WriteString("事件序列:\n")
		for _, ev := range in.Scene.EventSeq {
			b.WriteString(fmt.Sprintf("- #%d %s → %s [情绪: %s]\n", ev.Order, ev.Actor, ev.Action, ev.Emotion))
		}

		if len(in.Scene.Dialogues) > 0 {
			b.WriteString("\n关键对话:\n")
			for _, d := range in.Scene.Dialogues {
				b.WriteString(fmt.Sprintf("- %s: \"%s\"\n", d.Speaker, d.Line))
			}
		}

		b.WriteString("\n四拍提示:\n")
		b.WriteString(fmt.Sprintf("- 铺垫: %s\n", in.Scene.RhythmHints.Setup))
		b.WriteString(fmt.Sprintf("- 冲突升级: %s\n", in.Scene.RhythmHints.Conflict))
		b.WriteString(fmt.Sprintf("- 高潮: %s\n", in.Scene.RhythmHints.Climax))
		b.WriteString(fmt.Sprintf("- 悬念: %s\n", in.Scene.RhythmHints.Suspense))
	}

	if in.Round > 0 {
		b.WriteString(fmt.Sprintf("\n⚠️ 这是第%d轮修改稿，请更仔细地遵循以上规范。\n", in.Round))
	}

	b.WriteString(fmt.Sprintf("\n目标字数: %d-%d字\n", a.spec.MinWords, a.spec.MaxWords))
	b.WriteString("\n直接输出正文，不需要JSON包裹。开始：\n")

	return b.String()
}

// ─── Rewrite Mode ───────────────────────────────────────────────────────────

// IssueItem is one problem that the Review Agent found in the draft.
type IssueItem struct {
	Severity       string `json:"severity"`        // must_fix | suggest
	Category       string `json:"category"`        // 字数不足 | 流畅度 | 剧情遗漏 | AI痕迹 | 人设冲突
	Location       string `json:"location"`        // paragraph reference
	Description    string `json:"description"`     // what's wrong
	Suggestion     string `json:"suggestion"`      // how to fix it
	ReferenceScene string `json:"reference_scene"` // related original scene data
}

// RewriteInput is what the Novel Writer needs for a rewrite round.
type RewriteInput struct {
	ChapterNum   int
	CurrentDraft string                // the chapter text to rewrite
	Issues       []IssueItem           // problem list from review agent
	Scene        *recorderagent.Scene  // original scene for reference
	Round        int                   // which rewrite round (1 or 2)
	ConsultantCtx string              // consultant context from recorder agent
}

// BuildRewritePrompt builds the LLM prompt for rewrite mode.
func (a *Agent) BuildRewritePrompt(in RewriteInput) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("【角色锁定】你是专业的%s网文作家。当前是修改模式，第%d轮修改。\n\n", a.genre, in.Round))

	b.WriteString("## 审核Agent打回原因\n")
	for i, issue := range in.Issues {
		severityMark := "🔴"
		if issue.Severity == "suggest" {
			severityMark = "🟡"
		}
		b.WriteString(fmt.Sprintf("%s %d. [%s] %s\n", severityMark, i+1, issue.Category, issue.Description))
		if issue.Suggestion != "" {
			b.WriteString(fmt.Sprintf("   修改建议: %s\n", issue.Suggestion))
		}
		if issue.ReferenceScene != "" {
			b.WriteString(fmt.Sprintf("   原始场景参考: %s\n", issue.ReferenceScene))
		}
	}
	b.WriteString("\n")

	// Consultant context
	if in.ConsultantCtx != "" {
		b.WriteString("## 记录Agent顾问数据\n")
		b.WriteString(in.ConsultantCtx)
		b.WriteString("\n")
	}

	// Scene reference
	if in.Scene != nil {
		b.WriteString("## 原始场景（确保修改不偏离推演）\n")
		for _, ev := range in.Scene.EventSeq {
			b.WriteString(fmt.Sprintf("- #%d %s → %s\n", ev.Order, ev.Actor, ev.Action))
		}
		b.WriteString("\n")
	}

	// Forbid check
	b.WriteString("## 禁用表达（重点检查）\n")
	for _, phrase := range a.spec.ForbidPhrases {
		b.WriteString(fmt.Sprintf("- \"%s\" — 如出现必须替换\n", phrase))
	}
	b.WriteString("\n")

	b.WriteString("## 待修改正文\n\n")
	b.WriteString(in.CurrentDraft)
	b.WriteString("\n\n")
	b.WriteString("请输出修改后的完整正文，直接输出，不需要JSON包裹。")

	return b.String()
}

// ─── Consultant Query Helpers ───────────────────────────────────────────────

// BuildConsultQuery builds a consultant query for the Recorder Agent
// based on the issue the Novel Writer is trying to fix.
func BuildConsultQuery(issue IssueItem) recorderagent.ConsultantQuery {
	switch issue.Category {
	case "剧情遗漏":
		return recorderagent.ConsultantQuery{
			Type:     "scene_recall",
			Question: fmt.Sprintf("遗漏事件的详情是什么？%s", issue.ReferenceScene),
			Target:   issue.ReferenceScene,
		}
	case "人设冲突":
		return recorderagent.ConsultantQuery{
			Type:     "character_state",
			Question: fmt.Sprintf("角色在原始推演中的实际状态？%s", issue.Description),
			Target:   extractCharName(issue.Description),
		}
	default:
		return recorderagent.ConsultantQuery{
			Type:     "omission_check",
			Question: "修改稿是否覆盖了所有关键事件？",
			Target:   "all",
		}
	}
}

func extractCharName(desc string) string {
	// Simple heuristic: extract first Chinese name-like token
	for _, r := range []rune(desc) {
		if r >= 0x4e00 && r <= 0x9fff {
			return string(r)
		}
	}
	return desc
}

// ─── Quality Self-Check ─────────────────────────────────────────────────────

// SelfCheckResult is a quick internal check the Novel Writer runs
// before submitting to the Review Agent.
type SelfCheckResult struct {
	WordCount  int      `json:"word_count"`
	MeetsMin   bool     `json:"meets_min"`
	MeetsMax   bool     `json:"meets_max"`
	ForbidHits []string `json:"forbid_hits"` // which forbidden phrases were detected
}

// SelfCheck runs basic quality checks on a draft.
func (a *Agent) SelfCheck(draft string) SelfCheckResult {
	runes := []rune(draft)
	wc := len(runes)

	var forbidHits []string
	for _, phrase := range a.spec.ForbidPhrases {
		if strings.Contains(draft, phrase) {
			forbidHits = append(forbidHits, phrase)
		}
	}

	return SelfCheckResult{
		WordCount:  wc,
		MeetsMin:   wc >= a.spec.MinWords,
		MeetsMax:   wc <= a.spec.MaxWords,
		ForbidHits: forbidHits,
	}
}
