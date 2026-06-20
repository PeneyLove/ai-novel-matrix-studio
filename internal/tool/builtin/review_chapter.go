package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(reviewChapter{})
}

type reviewChapter struct{}

type reviewChapterArgs struct {
	Source    string `json:"source"`     // 章节文本内容（先 read_file 读取后传入）
	ChapterID string `json:"chapter_id"` // 可选的章节标识（如 "第5章"），用于报告标题
}

func (r reviewChapter) Name() string { return "review_chapter" }

func (r reviewChapter) Description() string {
	return "章节审查引擎：对单章文本运行 SOP 17项审查清单中可自动化的项目（字数、段落、标点、句逗比、平坦段、标牌、问句句号），输出结构化审查报告。语义检查项（线程、支线、打斗等）标记为需要人工审查。先 read_file 读取章节内容，再将内容传入 source 参数。"
}

func (r reviewChapter) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {
				"type": "string",
				"description": "章节文本内容（从 read_file 读取后传入）"
			},
			"chapter_id": {
				"type": "string",
				"description": "可选的章节标识，如'第5章'，用于报告标题"
			}
		},
		"required": ["source"]
	}`)
}

func (r reviewChapter) ReadOnly() bool { return true }

func (r reviewChapter) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewChapterArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_chapter: invalid args: %w", err)
	}
	if p.Source == "" {
		return "", fmt.Errorf("review_chapter: source text is empty — read the chapter file first and pass its content")
	}

	report := runChapterReview(p.ChapterID, p.Source)
	return report, nil
}

// ---------- review engine ----------

type reviewResult struct {
	id     int
	name   string
	status string // "pass", "fail", "warn", "manual"
	detail string
	fix    string
}

func runChapterReview(chapterID, text string) string {
	var results []reviewResult

	// Normalise line endings and strip BOM.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")

	lines := strings.Split(text, "\n")

	// ---- Item 1: 纯汉字字数 ≥2300 ----
	cjkCount := countCJK(text)
	r1 := reviewResult{id: 1, name: "纯汉字≥2300"}
	if cjkCount >= 2300 {
		r1.status = "pass"
		r1.detail = fmt.Sprintf("纯汉字 %d 字", cjkCount)
	} else {
		r1.status = "fail"
		r1.detail = fmt.Sprintf("纯汉字仅 %d 字，差 %d 字", cjkCount, 2300-cjkCount)
		r1.fix = fmt.Sprintf("需补充约 %d 字内容（对话/描写/内心独白）", 2300-cjkCount)
	}
	results = append(results, r1)

	// ---- Item 2: 段落 ≤80汉字 ----
	nodes := paragraphNodes(lines)
	over80 := []int{}
	for _, nd := range nodes {
		cjk := countCJK(nd.text)
		if cjk > 80 {
			over80 = append(over80, nd.line)
		}
	}
	r2 := reviewResult{id: 2, name: "段落≤80汉字"}
	if len(over80) == 0 {
		r2.status = "pass"
		r2.detail = "无超限段落"
	} else {
		r2.status = "fail"
		r2.detail = fmt.Sprintf("%d 段超 80 汉字：行 %v", len(over80), formatLineNums(over80))
		r2.fix = "将超限段落拆分为 2-3 个短段，或压缩冗余描写"
	}
	results = append(results, r2)

	// ---- Item 3: 段间有空行 ----
	r3 := reviewResult{id: 3, name: "段间有空行"}
	consecutiveNoBlank := countConsecutiveNonBlank(nodes)
	if consecutiveNoBlank == 0 {
		r3.status = "pass"
		r3.detail = "段间空行正常"
	} else {
		r3.status = "warn"
		r3.detail = fmt.Sprintf("发现 %d 处连续段落无空行分隔", consecutiveNoBlank)
		r3.fix = "在连续段落间插入空行，保证手机阅读视觉分段"
	}
	results = append(results, r3)

	// ---- Item 4: 对话独立成段 ----
	r4 := reviewResult{id: 4, name: "对话独立成段"}
	dialogueIssues := checkDialogueParagraphing(lines)
	if len(dialogueIssues) == 0 {
		r4.status = "pass"
		r4.detail = "对话格式正常"
	} else {
		r4.status = "warn"
		r4.detail = fmt.Sprintf("%d 处对话未独立成段：行 %v", len(dialogueIssues), formatLineNums(dialogueIssues))
		r4.fix = "每句对话独占一行，对话后紧接的动作描写可同行或以新段承接"
	}
	results = append(results, r4)

	// ---- Item 10: 标点检测 ----
	r10 := reviewResult{id: 10, name: "标点正确（？/！/……）"}
	periodQuestion := findQuestionEndingWithPeriod(text)
	exclamCount := strings.Count(text, "！")
	var punctIssues []string
	if len(periodQuestion) > 0 {
		punctIssues = append(punctIssues, fmt.Sprintf("%d 处问句以。收尾", len(periodQuestion)))
	}
	if exclamCount > 5 {
		punctIssues = append(punctIssues, fmt.Sprintf("感叹号 %d 个（偏多，建议 ≤5）", exclamCount))
	}
	if len(punctIssues) == 0 {
		r10.status = "pass"
		r10.detail = "标点使用正常"
	} else {
		r10.status = "fail"
		r10.detail = strings.Join(punctIssues, "；")
		r10.fix = "问句必须用？收尾；感叹号控制在关键情绪点使用"
	}
	results = append(results, r10)

	// ---- Item 12: 时间地点标牌 ----
	r12 := reviewResult{id: 12, name: "无独立时间地点标牌"}
	signs := findTimePlaceSigns(lines)
	if len(signs) == 0 {
		r12.status = "pass"
		r12.detail = "未发现独立标牌"
	} else {
		r12.status = "fail"
		r12.detail = fmt.Sprintf("%d 处独立标牌：%v", len(signs), signs)
		r12.fix = "将时间/地点融入叙事句，不要独立成句"
	}
	results = append(results, r12)

	// ---- Item 16: 句逗比 ≤3:1 ----
	r16 := reviewResult{id: 16, name: "句逗比≤3:1"}
	periodCnt := strings.Count(text, "。")
	commaCnt := strings.Count(text, "，")
	ratio := 0.0
	if commaCnt > 0 {
		ratio = float64(periodCnt) / float64(commaCnt)
	}
	if ratio <= 3.0 {
		r16.status = "pass"
		r16.detail = fmt.Sprintf("句逗比 %.1f:1（句号 %d，逗号 %d）", ratio, periodCnt, commaCnt)
	} else {
		r16.status = "fail"
		r16.detail = fmt.Sprintf("句逗比 %.1f:1（句号 %d，逗号 %d），>3:1 进入干涩区", ratio, periodCnt, commaCnt)
		r16.fix = "连续动作用逗号串联，句号只在序列结束时用"
	}
	results = append(results, r16)

	// ---- Item 17: 平坦段（连续5句以上纯句号） ----
	r17 := reviewResult{id: 17, name: "无5句以上平坦段"}
	flatLines := findFlatSegments(nodes)
	if len(flatLines) == 0 {
		r17.status = "pass"
		r17.detail = "未发现平坦段"
	} else {
		r17.status = "fail"
		r17.detail = fmt.Sprintf("%d 处平坦段：行 %v", len(flatLines), formatLineNums(flatLines))
		r17.fix = "用拆段/标点破/长短撞三法打破平坦节奏"
	}
	results = append(results, r17)

	// ---- Item 8: 结尾检查（动作/场景收束，有钩子） ----
	r8 := reviewResult{id: 8, name: "结尾动作/场景收束"}
	endIssues := checkChapterEnding(nodes, lines)
	if len(endIssues) == 0 {
		r8.status = "pass"
		r8.detail = "结尾正常"
	} else {
		r8.status = "warn"
		r8.detail = fmt.Sprintf("结尾问题：%s", strings.Join(endIssues, "；"))
		r8.fix = "环境独白结尾→改为动作或对话收束；标牌结尾→融入叙事；无钩子→加入悬念/冲突/揭秘"
	}
	results = append(results, r8)

	// ---- Item 11: 乒乓球对话检测 ----
	r11 := reviewResult{id: 11, name: "无3连Q&A乒乓球"}
	pingpong := detectPingPongDialogue(nodes)
	if len(pingpong) == 0 {
		r11.status = "pass"
		r11.detail = "未发现乒乓球对话"
	} else {
		r11.status = "warn"
		r11.detail = fmt.Sprintf("%d 处疑似乒乓球对话：行 %v", len(pingpong), formatLineNums(pingpong))
		r11.fix = "在Q&A之间插入身体语言/动作描写/环境反应，打破审讯笔录感"
	}
	results = append(results, r11)

	// ---- AI 套话检测 ----
	aiSlop := detectAISlop(text)
	rAI := reviewResult{id: 0, name: "AI套话检测"}
	if len(aiSlop) == 0 {
		rAI.status = "pass"
		rAI.detail = "未发现AI套话"
	} else {
		rAI.status = "fail"
		rAI.detail = fmt.Sprintf("%d 处AI套话：%v", len(aiSlop), aiSlop)
		rAI.fix = "删除AI套话，用具体描写替代"
	}
	results = append(results, rAI)

	// ---- 语义检查项（标记为 manual） ----
	manualItems := []struct {
		id   int
		name string
	}{
		{5, "≥3条线程"},
		{6, "支线≥1处信息传递"},
		{7, "≥1处信息不对称"},
		{9, "下章衔接点已埋"},
		{13, "支线因果挂钩+锚定"},
		{14, "只推一条支线"},
		{15, "打斗四要素齐备"},
	}
	for _, mi := range manualItems {
		results = append(results, reviewResult{
			id:     mi.id,
			name:   mi.name,
			status: "manual",
			detail: "需要人工审查（语义检查）",
		})
	}

	// ---- Build report ----
	var sb strings.Builder
	title := chapterID
	if title == "" {
		title = "章节"
	}
	sb.WriteString(fmt.Sprintf("═══ 审查报告：%s ═══\n\n", title))

	pass, fail, warn, manual := 0, 0, 0, 0
	for _, r := range results {
		switch r.status {
		case "pass":
			pass++
		case "fail":
			fail++
		case "warn":
			warn++
		case "manual":
			manual++
		}
	}
	total := pass + fail + warn + manual
	sb.WriteString(fmt.Sprintf("总检查项：%d | ✅通过 %d | ❌违规 %d | ⚠警告 %d | 👁需人工 %d\n\n", total, pass, fail, warn, manual))

	// 违规项（fail）
	if fail > 0 {
		sb.WriteString("### 🔴 必须修复\n\n")
		for _, r := range results {
			if r.status == "fail" {
				sb.WriteString(fmt.Sprintf("**项%d %s**\n", r.id, r.name))
				sb.WriteString(fmt.Sprintf("> %s\n", r.detail))
				if r.fix != "" {
					sb.WriteString(fmt.Sprintf("💡 %s\n\n", r.fix))
				} else {
					sb.WriteString("\n")
				}
			}
		}
	}

	// 警告项
	if warn > 0 {
		sb.WriteString("### 🟡 建议修复\n\n")
		for _, r := range results {
			if r.status == "warn" {
				sb.WriteString(fmt.Sprintf("**项%d %s**\n", r.id, r.name))
				sb.WriteString(fmt.Sprintf("> %s\n", r.detail))
				if r.fix != "" {
					sb.WriteString(fmt.Sprintf("💡 %s\n\n", r.fix))
				} else {
					sb.WriteString("\n")
				}
			}
		}
	}

	// 需人工审查项
	if manual > 0 {
		sb.WriteString("### 👁 需人工审查\n\n")
		for _, r := range results {
			if r.status == "manual" {
				sb.WriteString(fmt.Sprintf("- **项%d %s**：%s\n", r.id, r.name, r.detail))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("═══ 报告结束 ═══\n")
	return sb.String()
}

// ---------- helper types & functions ----------

type paraNode struct {
	line int    // 1-indexed
	text string // trimmed, non-empty line content
}

// paragraphNodes extracts non-empty, non-heading content lines as paragraph nodes.
func paragraphNodes(lines []string) []paraNode {
	var out []paraNode
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Skip markdown headings and metadata lines.
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "- ") {
			continue
		}
		out = append(out, paraNode{line: i + 1, text: trimmed})
	}
	return out
}

// countCJK counts Chinese/Japanese/Korean characters (Unicode CJK ranges).
func countCJK(s string) int {
	n := 0
	for _, r := range s {
		if isCJK(r) {
			n++
		}
	}
	return n
}

func isCJK(r rune) bool {
	if r >= 0x4E00 && r <= 0x9FFF {
		return true // CJK Unified Ideographs
	}
	if r >= 0x3400 && r <= 0x4DBF {
		return true // CJK Unified Ideographs Extension A
	}
	if r >= 0x20000 && r <= 0x2A6DF {
		return true // Extension B
	}
	if r >= 0xF900 && r <= 0xFAFF {
		return true // CJK Compatibility Ideographs
	}
	return false
}

// countConsecutiveNonBlank counts how many times two consecutive paragraph nodes
// appear without a blank line between them.
func countConsecutiveNonBlank(nodes []paraNode) int {
	if len(nodes) < 2 {
		return 0
	}
	count := 0
	for i := 1; i < len(nodes); i++ {
		if nodes[i].line-nodes[i-1].line == 1 {
			count++
		}
	}
	return count
}

// checkDialogueParagraphing finds lines where dialogue is not independently paragraphed.
// Heuristic: a line that contains quoted speech mixed with non-dialogue content
// (quotes not at the very start of the trimmed line) may be a formatting issue.
func checkDialogueParagraphing(lines []string) []int {
	var issues []int
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Find quoted speech in the line.
		// Try Chinese quotes (U+201C/U+201D) first, then ASCII double quotes.
		leftQuote := "\u201C"  // "
		rightQuote := "\u201D" // "
		left := strings.Index(trimmed, leftQuote)
		if left < 0 {
			// Fall back to ASCII double quote.
			leftQuote = "\""
			rightQuote = "\""
			left = strings.Index(trimmed, leftQuote)
		}
		if left < 0 {
			continue
		}
		// Find the matching closing quote.
		right := strings.Index(trimmed[left+1:], rightQuote)
		if right < 0 {
			continue
		}
		right += left + 1
		}
		if right < 0 {
			continue
		}
		right += left + 1
		// Check if there's text after the closing quote on the same line
		// that isn't just a dialogue tag.
		after := strings.TrimSpace(trimmed[right+1:])
		if len(after) > 15 && !strings.HasPrefix(after, "——") && !strings.HasPrefix(after, "…") {
			// Non-trivial text after closing quote — likely two speech segments.
			issues = append(issues, i+1)
		}
		// Also check for two separate quoted segments in one line.
		secondLeft := strings.Index(trimmed[right+1:], "\u201C")
		if secondLeft < 0 {
			secondLeft = strings.Index(trimmed[right+1:], "\"")
		}
		if secondLeft >= 0 {
			issues = append(issues, i+1)
		}
	}
	return dedupInts(issues)
}

// findQuestionEndingWithPeriod finds quoted questions that end with 。instead of ？
func findQuestionEndingWithPeriod(text string) []string {
	// Match quoted strings that contain question words and end with 。"
	// Use interpreted string (not raw) so \u escapes resolve to Unicode.
	re := regexp.MustCompile("[\"\u201C][^\"\u201D]{0,80}(\u4EC0\u4E48|\u600E\u4E48|\u54EA|\u8C01|\u5417|\u5462|\u5E72\u4EC0|\u80FD\u4E0D\u80FD|\u662F\u4E0D\u662F|\u5BF9\u4E0D\u5BF9)[^\"\u201D]{0,20}\u3002[\"\u201D]")
	matches := re.FindAllString(text, -1)
	// Truncate for display.
	var out []string
	for _, m := range matches {
		display := m
		if len(display) > 30 {
			display = display[:30] + "…\""
		}
		out = append(out, display)
	}
	return out
}

// findTimePlaceSigns looks for standalone time/place signboard lines.
func findTimePlaceSigns(lines []string) []string {
	patterns := []string{
		"傍晚", "木屋", "深夜", "第二天", "同一天", "几天之后",
		"清晨", "午后", "黄昏", "午夜", "翌日", "次日",
	}
	var hits []string
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		// Must be a short standalone line (≤10 CJK chars).
		cjk := countCJK(trimmed)
		if cjk == 0 || cjk > 10 {
			continue
		}
		// Skip lines that contain punctuation which suggests integration.
		if strings.Contains(trimmed, "，") || strings.Contains(trimmed, "。") {
			continue
		}
		for _, pat := range patterns {
			if strings.Contains(trimmed, pat) {
				hits = append(hits, fmt.Sprintf("行%d: %s", i+1, trimmed))
				break
			}
		}
	}
	return hits
}

// findFlatSegments finds paragraph lines with ≥5 consecutive 。-terminated clauses
// and no other sentence-ending punctuation (？！…).
func findFlatSegments(nodes []paraNode) []int {
	var issues []int
	for _, nd := range nodes {
		// Count 。 vs other sentence-ending punctuation.
		p := 0 // 。
		q := 0 // ？！…
		for _, r := range nd.text {
			switch r {
			case '\u3002': // 。
				p++
			case '\uFF1F', '\uFF01', '\u2026': // ？！…
				q++
			}
		}
		if p >= 5 && q == 0 && countCJK(nd.text) > 60 {
			issues = append(issues, nd.line)
		}
	}
	return issues
}

// checkChapterEnding examines the last ~300 CJK chars for forbidden ending patterns.
func checkChapterEnding(nodes []paraNode, lines []string) []string {
	var issues []string

	// Extract last significant content lines (skip metadata markers like ---, *, #).
	var contentLines []paraNode
	for i := len(nodes) - 1; i >= 0 && len(contentLines) < 5; i-- {
		contentLines = append([]paraNode{nodes[i]}, contentLines...)
	}
	if len(contentLines) == 0 {
		return nil
	}

	// Join last content lines for pattern matching.
	lastText := ""
	for _, nd := range contentLines {
		lastText += nd.text
	}

	// Forbidden ending: environment monologue.
	envMonologue := []string{
		"天黑了", "夜深了", "月光洒", "风停了", "雨停了",
		"太阳落山", "夜幕降临", "天色渐暗", "万籁俱寂",
		"一切归于平静", "世界安静下来",
	}
	for _, pat := range envMonologue {
		if strings.Contains(lastText, pat) {
			issues = append(issues, fmt.Sprintf("环境独白结尾「%s」", pat))
			break
		}
	}

	// Forbidden ending: standalone signboard at end of chapter.
	if len(contentLines) > 0 {
		lastLine := contentLines[len(contentLines)-1]
		cjk := countCJK(lastLine.text)
		if cjk <= 10 && cjk > 0 {
			signs := findTimePlaceSigns([]string{lastLine.text})
			if len(signs) > 0 {
				issues = append(issues, "标牌结尾")
			}
		}
	}

	// Weak ending: ends with passive description (no action/conflict hint).
	weakEndings := []string{"了。", "着。", "去。", "来。", "过。"}
	if len(contentLines) > 0 {
		lastLine := contentLines[len(contentLines)-1].text
		endsWeak := false
		for _, we := range weakEndings {
			if strings.HasSuffix(lastLine, we) {
				endsWeak = true
				break
			}
		}
		if endsWeak && !strings.Contains(lastLine, "？") && !strings.Contains(lastLine, "！") {
			issues = append(issues, "结尾偏弱，建议加钩子")
		}
	}

	return issues
}

// detectPingPongDialogue finds 3+ consecutive lines of pure Q&A without body language.
func detectPingPongDialogue(nodes []paraNode) []int {
	var issues []int
	// Find consecutive lines that are purely quoted speech.
	type lineType int
	const (
		typeOther lineType = iota
		typeQuote
	)
	classify := func(text string) lineType {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "\u201C") || strings.HasPrefix(trimmed, "\"") ||
			strings.HasPrefix(trimmed, "\u300C") {
			return typeQuote
		}
		// Also match lines that are entirely a quote with trailing dialogue tag.
		if len(trimmed) > 2 && (trimmed[0] == '\u201C' || trimmed[0] == '"') {
			return typeQuote
		}
		return typeOther
	}

	quoteRun := 0
	runStart := 0
	for i, nd := range nodes {
		if classify(nd.text) == typeQuote {
			if quoteRun == 0 {
				runStart = nd.line
			}
			quoteRun++
		} else {
			if quoteRun >= 3 {
				issues = append(issues, runStart)
			}
			quoteRun = 0
		}
	}
	if quoteRun >= 3 {
		issues = append(issues, runStart)
	}
	return issues
}

// detectAISlop finds common AI-generated filler phrases.
func detectAISlop(text string) []string {
	slopPhrases := []string{
		"不仅如此", "更重要的是", "总而言之", "综上所述",
		"在这个过程中", "从此以后", "毫无疑问",
		"显得格外", "透露出一种", "让人不禁", "令人感到",
		"不得不说", "显而易见", "不约而同",
	}
	var hits []string
	for _, phrase := range slopPhrases {
		if strings.Contains(text, phrase) {
			hits = append(hits, phrase)
		}
	}
	return hits
}

// helpers

func formatLineNums(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}

func dedupInts(nums []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, n := range nums {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
