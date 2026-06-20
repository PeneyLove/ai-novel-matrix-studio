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
	tool.RegisterBuiltin(fixChapter{})
}

type fixChapter struct{}

type fixChapterArgs struct {
	Source          string `json:"source"`           // 原始章节文本
	Fixes           string `json:"fixes"`            // 修复模式：blanks/period_q/signs/slop/struct_mark/ending_type/all
	PrevEndingType  string `json:"prev_ending_type"` // 上一章结尾类型（用于 struct_mark 模式，可选）
	ChapterMeta     string `json:"chapter_meta"`     // 章节元数据行（struct_mark 模式插入章首，可选）
	ConfigJSON      string `json:"config_json"`      // 项目配置 JSON（用于读取 replace_map 等），可选
}

func (f fixChapter) Name() string { return "fix_chapter" }

func (f fixChapter) Description() string {
	return "章节自动修复+格式化工具：机械修复(blanks/period_q/signs/slop) + 结构标记(struct_mark:插入四段分隔符+元数据头) + 结尾检测(ending_type:分析结尾类型和钩子强度)。传入 fixes='all' 执行全部。先 read_file 读取后再传入 source。"
}

func (f fixChapter) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {
				"type": "string",
				"description": "原始章节文本（从 read_file 读取后传入）"
			},
			"fixes": {
				"type": "string",
				"description": "修复模式：blanks/period_q/signs/slop/replace_repeat/struct_mark/ending_type/all"
			},
			"config_json": {
				"type": "string",
				"description": "项目配置 JSON（read_file .novel-agent/novel-config.json 后传入），用于 replace_map"
			}
		},
		"required": ["source"]
	}`)
}

func (f fixChapter) ReadOnly() bool { return true }

func (f fixChapter) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p fixChapterArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("fix_chapter: invalid args: %w", err)
	}
	if p.Source == "" {
		return "", fmt.Errorf("fix_chapter: source text is empty")
	}

	fixes := parseFixes(p.Fixes)
	text := strings.ReplaceAll(p.Source, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Parse config for replace_map if provided.
	replaceMap := map[string]string{}
	if p.ConfigJSON != "" {
		var full struct {
			RepeatCtrl struct {
				ReplaceMap map[string]string `json:"replace_map"`
			} `json:"repeat_control"`
		}
		if err := json.Unmarshal([]byte(p.ConfigJSON), &full); err == nil {
			replaceMap = full.RepeatCtrl.ReplaceMap
		}
	}

	var changes []string
	fixed := text

	if fixes["blanks"] || fixes["all"] {
		var c []string
		fixed, c = fixBlankLines(fixed)
		changes = append(changes, c...)
	}
	if fixes["period_q"] || fixes["all"] {
		var c []string
		fixed, c = fixPeriodQuestions(fixed)
		changes = append(changes, c...)
	}
	if fixes["signs"] || fixes["all"] {
		var c []string
		fixed, c = fixSignboards(fixed)
		changes = append(changes, c...)
	}
	if fixes["slop"] || fixes["all"] {
		var c []string
		fixed, c = fixAISlop(fixed)
		changes = append(changes, c...)
	}
	if fixes["struct_mark"] || fixes["all"] {
		var c []string
		fixed, c = insertStructMarkers(fixed, p.ChapterMeta, p.PrevEndingType)
		changes = append(changes, c...)
	}
	if fixes["replace_repeat"] || fixes["all"] {
		var c []string
		fixed, c = applyReplaceMap(fixed, replaceMap)
		changes = append(changes, c...)
	}
	if fixes["ending_type"] || fixes["all"] {
		nodes := paragraphNodes(strings.Split(fixed, "\n"))
		ending := detectEndingType(nodes, nil)
		hasHook := detectChapterHook(nodes)
		hookStr := "有钩子"
		if !hasHook {
			hookStr = "⚠ 无钩子信号"
		}
		changes = append(changes, fmt.Sprintf("结尾类型：%s（%s）", ending, hookStr))
	}

	var sb strings.Builder
	sb.WriteString("═══ 自动修复报告 ═══\n\n")
	sb.WriteString(fmt.Sprintf("执行修复：%s\n", p.Fixes))
	sb.WriteString(fmt.Sprintf("变更数：%d\n\n", len(changes)))

	if len(changes) > 0 {
		sb.WriteString("### 变更明细\n\n")
		for i, c := range changes {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("无需修复 ✅\n\n")
	}

	sb.WriteString("### 修复后文本\n\n")
	sb.WriteString("```\n")
	sb.WriteString(fixed)
	sb.WriteString("\n```\n")
	sb.WriteString("\n═══ 报告结束 ═══\n")

	return sb.String(), nil
}

func parseFixes(raw string) map[string]bool {
	m := map[string]bool{}
	if raw == "" {
		raw = "all"
	}
	for _, f := range strings.Split(raw, ",") {
		m[strings.TrimSpace(f)] = true
	}
	return m
}

// ---------- fix functions ----------

// fixBlankLines inserts blank lines between consecutive non-empty content lines.
func fixBlankLines(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var out []string
	var changes []string
	prevContent := false

	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		isContent := trimmed != "" && !strings.HasPrefix(trimmed, "#") &&
			!strings.HasPrefix(trimmed, "---") && !strings.HasPrefix(trimmed, "===")

		if isContent && prevContent {
			// Previous line was content and this is content — need a blank line.
			out = append(out, "")
			changes = append(changes, fmt.Sprintf("行%d前行间插入空行", i+1))
		}

		out = append(out, l)
		prevContent = isContent
	}

	return strings.Join(out, "\n"), changes
}

// fixPeriodQuestions replaces 。" with ？" for question sentences.
func fixPeriodQuestions(text string) (string, []string) {
	var changes []string
	// Match quoted question sentences ending with 。"
	re := regexp.MustCompile("[\"\u201C]([^\"\u201D]{0,80}?)(\u4EC0\u4E48|\u600E\u4E48|\u54EA|\u8C01|\u5417|\u5462|\u5E72\u4EC0|\u80FD\u4E0D\u80FD|\u662F\u4E0D\u662F|\u5BF9\u4E0D\u5BF9)([^\"\u201D]{0,20}?)\u3002([\"\u201D])")

	fixed := re.ReplaceAllStringFunc(text, func(match string) string {
		// Replace the final 。with ？
		lastPeriod := strings.LastIndex(match, "\u3002")
		lastQuote := strings.LastIndexFunc(match, func(r rune) bool {
			return r == '"' || r == '\u201D'
		})
		if lastPeriod >= 0 && lastQuote > lastPeriod {
			fixed := match[:lastPeriod] + "\uFF1F" + match[lastPeriod+len("\u3002"):]
			// Truncate for display.
			display := match
			if len(display) > 25 {
				display = display[:25] + "…\""
			}
			changes = append(changes, fmt.Sprintf("修复问句句号：%s", display))
			return fixed
		}
		return match
	})

	return fixed, changes
}

// fixSignboards removes standalone time/place signboard lines.
func fixSignboards(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var out []string
	var changes []string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if isSignboardLine(trimmed) {
			changes = append(changes, fmt.Sprintf("删除标牌行：「%s」", trimmed))
			// Skip this line (remove it).
			continue
		}
		out = append(out, l)
	}

	return strings.Join(out, "\n"), changes
}

func isSignboardLine(s string) bool {
	cjk := 0
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk++
		}
	}
	if cjk == 0 || cjk > 10 {
		return false
	}
	// Skip lines with punctuation (integrated into narrative).
	if strings.Contains(s, "，") || strings.Contains(s, "。") ||
		strings.Contains(s, "！") || strings.Contains(s, "？") {
		return false
	}
	patterns := []string{"傍晚", "木屋", "深夜", "第二天", "同一天", "几天之后",
		"清晨", "午后", "黄昏", "午夜", "翌日", "次日"}
	for _, pat := range patterns {
		if strings.Contains(s, pat) {
			return true
		}
	}
	return false
}

// fixAISlop removes common AI slop phrases.
func fixAISlop(text string) (string, []string) {
	slopPhrases := []string{
		"不仅如此，", "更重要的是，", "总而言之，", "综上所述，",
		"在这个过程中，", "从此以后，", "毫无疑问，",
		"显得格外", "透露出一种", "让人不禁", "令人感到",
		"不得不说，", "显而易见，", "不约而同，",
	}
	var changes []string
	fixed := text
	for _, phrase := range slopPhrases {
		if strings.Contains(fixed, phrase) {
			before := fixed
			fixed = strings.ReplaceAll(fixed, phrase, "")
			if fixed != before {
				changes = append(changes, fmt.Sprintf("删除AI套话「%s」", phrase))
			}
		}
	}
	return fixed, changes
}

// applyReplaceMap replaces forbidden phrases with their configured alternatives.
func applyReplaceMap(text string, replaceMap map[string]string) (string, []string) {
	if len(replaceMap) == 0 {
		return text, nil
	}
	var changes []string
	fixed := text
	for old, new := range replaceMap {
		if strings.Contains(fixed, old) {
			fixed = strings.ReplaceAll(fixed, old, new)
			changes = append(changes, fmt.Sprintf("替换「%s」→「%s」", old, new))
		}
	}
	return fixed, changes
}
	}
	var changes []string
	fixed := text
	for _, phrase := range slopPhrases {
		if strings.Contains(fixed, phrase) {
			before := fixed
			fixed = strings.ReplaceAll(fixed, phrase, "")
			if fixed != before {
				changes = append(changes, fmt.Sprintf("删除AI套话「%s」", phrase))
			}
		}
	}
	return fixed, changes
}
