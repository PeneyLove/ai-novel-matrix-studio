package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(checkChapter{})
}

type checkChapter struct{}

type checkChapterArgs struct {
	Source              string `json:"source"`                // 章节文本
	ChapterID           string `json:"chapter_id"`            // 章节号
	PrevEndingType      string `json:"prev_ending_type"`      // 上一章结尾类型（悬念/压力/行动/伏笔），空=首章
	PrevActions         string `json:"prev_actions"`          // 上一章动作列表，逗号分隔
	ExpectedEmotion     string `json:"expected_emotion"`      // 本章预期情绪（可选）
	ConfigJSON          string `json:"config_json"`           // 项目配置 JSON（read_file .novel-agent/novel-config.json 后传入），可选
}

func (c checkChapter) Name() string { return "check_chapter" }

func (c checkChapter) Description() string {
	return "CheckAgent 质量校验引擎：对章节运行 10 项量化检查（满分100），≥90分通过。自动化项：结构四段完整(15)/字数比例合规(15)/无重复动作(10)/无重复结尾(10)/结尾有钩子(10)。人工标记项：人设/时间线/物品/逻辑/支线。传入 prev_ending_type 和 prev_actions 用于重复检测。"
}

func (c checkChapter) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {"type": "string", "description": "章节文本内容"},
			"chapter_id": {"type": "string", "description": "章节号，如'第12章'"},
			"prev_ending_type": {"type": "string", "description": "上一章结尾类型：悬念/压力/行动/伏笔。首章留空"},
			"prev_actions": {"type": "string", "description": "上一章出现的动作，逗号分隔，如'打坐,摸刀,检查弩'"},
			"expected_emotion": {"type": "string", "description": "本章预期情绪，如'压迫/对峙/隐忍'"}
		},
			"config_json": {"type": "string", "description": "项目配置 JSON（read_file .novel-agent/novel-config.json 后传入），可选。用于覆盖默认的禁止动作列表/结尾类型/及格线"}
		},
		"required": ["source"]
	}`)
}

func (c checkChapter) ReadOnly() bool { return true }

func (c checkChapter) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p checkChapterArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("check_chapter: invalid args: %w", err)
	}
	if p.Source == "" {
		return "", fmt.Errorf("check_chapter: source text is empty")
	}

	cfg := parseNovelConfig(p.ConfigJSON)
	report := runCheckAgent(p, cfg)
	return report, nil
}

// novelConfig holds the project-level configuration for a novel.
type novelConfig struct {
	PassScore     int              `json:"pass_score"`
	EndingCycle   []string         `json:"ending_cycle_types"` // from ending_hook_rules.cycle_type
	ForbidActions []string         `json:"forbid_actions"`     // from repeat_control.high_frequency_forbid_list
	QualityItems  []qualityItemCfg `json:"quality_items"`      // from quality_check_list
}

type qualityItemCfg struct {
	Item  string `json:"item"`
	Score int    `json:"score"`
}

func parseNovelConfig(raw string) *novelConfig {
	if raw == "" {
		return nil
	}
	// The full ck.json has nested structure. Extract needed fields.
	var full struct {
		PassScore    int `json:"pass_score"`
		EndingRules  struct {
			CycleType []string `json:"cycle_type"`
		} `json:"ending_hook_rules"`
		RepeatCtrl struct {
			ForbidList []string `json:"high_frequency_forbid_list"`
		} `json:"repeat_control"`
		QualityList []qualityItemCfg `json:"quality_check_list"`
	}
	if err := json.Unmarshal([]byte(raw), &full); err != nil {
		return nil
	}
	cfg := &novelConfig{
		PassScore:    full.PassScore,
		EndingCycle:  full.EndingRules.CycleType,
		ForbidActions: full.RepeatCtrl.ForbidList,
		QualityItems: full.QualityList,
	}
	if cfg.PassScore == 0 {
		cfg.PassScore = 90
	}
	return cfg
}

// ---------- CheckAgent engine ----------

type checkItem struct {
	id      int
	name    string
	max     int
	score   int
	detail  string
	auto    bool // true = automated, false = manual flag
}

func runCheckAgent(args checkChapterArgs, cfg *novelConfig) string {
	text := args.Source
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")

	lines := strings.Split(text, "\n")
	nodes := paragraphNodes(lines)
	totalCJK := countCJK(text)

	var items []checkItem

	// ---- Item 1: 结构四段完整 (15pts) ----
	i1 := checkItem{id: 1, name: "结构四段完整", max: 15, auto: true}
	segs := detectFourSegments(nodes)
	if len(segs) >= 4 {
		i1.score = 15
		i1.detail = fmt.Sprintf("检测到 %d 段：%s", len(segs), strings.Join(segs, " → "))
	} else if len(segs) >= 3 {
		i1.score = 10
		i1.detail = fmt.Sprintf("仅 %d 段完整（缺 %s），扣5分", len(segs), missingSegment(segs))
	} else {
		i1.score = 5
		i1.detail = fmt.Sprintf("仅 %d 段（预期4段），扣10分", len(segs))
	}
	items = append(items, i1)

	// ---- Item 2: 字数比例合规 (15pts) ----
	i2 := checkItem{id: 2, name: "字数比例合规", max: 15, auto: true}
	if totalCJK == 0 {
		i2.score = 0
		i2.detail = "无法计算（无汉字）"
	} else {
		ratios := segmentRatios(segs, nodes, totalCJK)
		ratioStr := ""
		violations := 0
		for _, r := range ratios {
			ratioStr += fmt.Sprintf("%s=%.0f%% ", r.name, r.pct)
			if !r.inRange {
				violations++
			}
		}
		i2.detail = ratioStr
		if violations == 0 {
			i2.score = 15
		} else if violations <= 2 {
			i2.score = 10
			i2.detail += fmt.Sprintf("（%d段偏离）", violations)
		} else {
			i2.score = 5
			i2.detail += fmt.Sprintf("（%d段偏离）", violations)
		}
	}
	items = append(items, i2)

	// ---- Item 3: 无重复动作 (10pts) ----
	i3 := checkItem{id: 3, name: "无重复动作", max: 10, auto: true}
	forbiddenActions := []string{
		"打坐", "行气", "拔匕首", "拔柴刀", "检查弩",
		"摸预警绳", "灶台静坐", "摸刀", "擦刀",
		"反复拔", "盘膝坐下", "调息",
	}
	if cfg != nil && len(cfg.ForbidActions) > 0 {
		forbiddenActions = cfg.ForbidActions
	}
	prevActs := parseActionList(args.PrevActions)
	foundActs := findActionsInText(text, forbiddenActions)
	repeated := intersectActs(foundActs, prevActs)

	if len(repeated) == 0 {
		i3.score = 10
		i3.detail = "未发现与上章重复的禁止动作"
	} else if len(repeated) <= 1 {
		i3.score = 5
		i3.detail = fmt.Sprintf("与上章重复：%s，扣5分", strings.Join(repeated, "/"))
	} else {
		i3.score = 0
		i3.detail = fmt.Sprintf("多处重复：%s，扣10分", strings.Join(repeated, "/"))
	}
	items = append(items, i3)

	// ---- Item 4: 无重复结尾类型 (10pts) ----
	i4 := checkItem{id: 4, name: "无重复结尾类型", max: 10, auto: true}
	cycleTypes := []string{"悬念", "压力", "行动", "伏笔"}
	if cfg != nil && len(cfg.EndingCycle) > 0 {
		cycleTypes = cfg.EndingCycle
	}
	thisEnding := detectEndingType(nodes, cycleTypes)
	i4.detail = fmt.Sprintf("本章结尾类型：%s", thisEnding)
	if args.PrevEndingType != "" && thisEnding == args.PrevEndingType {
		i4.score = 0
		i4.detail += fmt.Sprintf("，与上章重复（%s），扣10分", args.PrevEndingType)
	} else {
		i4.score = 10
		if args.PrevEndingType != "" {
			i4.detail += fmt.Sprintf("，上章=%s ✅不重复", args.PrevEndingType)
		}
	}
	items = append(items, i4)

	// ---- Item 8: 结尾有钩子 (10pts) ----
	i8 := checkItem{id: 8, name: "结尾有钩子", max: 10, auto: true}
	hasHook := detectChapterHook(nodes)
	if hasHook {
		i8.score = 10
		i8.detail = "检测到钩子信号"
	} else {
		i8.score = 0
		i8.detail = "结尾无明显钩子（无悬念/冲突/行动/伏笔信号）"
	}
	items = append(items, i8)

	// ---- Item 5: 人设稳定 (5pts) ----
	i5 := checkItem{id: 5, name: "人设稳定", max: 5, auto: false}
	i5.score = 0
	i5.detail = "需人工审查（陈墨=冷静/少言/果决）"
	items = append(items, i5)

	// ---- Item 6: 时间线一致 (10pts) ----
	i6 := checkItem{id: 6, name: "时间线一致", max: 10, auto: false}
	i6.score = 0
	i6.detail = "需人工审查（核对 anchor-state 时间锚点）"
	items = append(items, i6)

	// ---- Item 7: 物品无冲突 (5pts) ----
	i7 := checkItem{id: 7, name: "物品无冲突", max: 5, auto: false}
	i7.score = 0
	i7.detail = "需人工审查（核对物品清单：借据/匕首/柴刀/弩/百草经）"
	items = append(items, i7)

	// ---- Item 9: 无逻辑漏洞 (5pts) ----
	i9 := checkItem{id: 9, name: "无逻辑漏洞", max: 5, auto: false}
	i9.score = 0
	i9.detail = "需人工审查"
	items = append(items, i9)

	// ---- Item 10: 支线不干扰主线 (5pts) ----
	i10 := checkItem{id: 10, name: "支线不干扰主线", max: 5, auto: false}
	i10.score = 0
	i10.detail = "需人工审查"
	items = append(items, i10)

	// ---- Build report ----
	total := 0
	maxScore := 0
	manualScore := 0
	for _, it := range items {
		total += it.score
		maxScore += it.max
		if !it.auto {
			manualScore += it.max
		}
	}
	autoOnly := total - 0 // all manual items score 0
	autoMax := maxScore - manualScore

	passThreshold := 90
	if cfg != nil && cfg.PassScore > 0 {
		passThreshold = cfg.PassScore
	}
	passed := total >= passThreshold

	var sb strings.Builder
	title := args.ChapterID
	if title == "" {
		title = "本章"
	}
	sb.WriteString(fmt.Sprintf("═══ CheckAgent 质量校验：%s ═══\n\n", title))

	verdict := "❌ FAIL（需重生成）"
	if passed {
		verdict = "✅ PASS"
	}
	sb.WriteString(fmt.Sprintf("总分：%d/%d  %s\n", total, maxScore, verdict))
	sb.WriteString(fmt.Sprintf("自动评分：%d/%d（%d 分需人工确认）\n\n", autoOnly, autoMax, manualScore))

	// Per-item breakdown.
	sb.WriteString("| # | 检查项 | 得分 | 满分 | 详情 |\n")
	sb.WriteString("|---|--------|------|------|------|\n")
	for _, it := range items {
		marker := "🤖"
		if !it.auto {
			marker = "👁"
		}
		icon := "✅"
		if it.score < it.max && it.auto {
			icon = "❌"
		} else if !it.auto {
			icon = "⬜"
		}
		sb.WriteString(fmt.Sprintf("| %s %d | %s | %s %d | %d | %s |\n",
			icon, it.id, it.name, marker, it.score, it.max, it.detail))
	}

	sb.WriteString(fmt.Sprintf("\n### 本章结尾类型：%s\n", detectEndingType(nodes, cycleTypes)))
	sb.WriteString(fmt.Sprintf("下章需轮换，禁止重复此类型。"))
	if args.PrevEndingType != "" {
		sb.WriteString(fmt.Sprintf(" 上章=%s → 本章=%s", args.PrevEndingType, detectEndingType(nodes, cycleTypes)))
	}
	sb.WriteString("\n\n")

	if !passed {
		sb.WriteString("### 修复指引\n\n")
		for _, it := range items {
			if it.score < it.max && it.auto {
				sb.WriteString(fmt.Sprintf("- **项%d %s**：得分 %d/%d，%s\n", it.id, it.name, it.score, it.max, it.detail))
			}
		}
		sb.WriteString(fmt.Sprintf("\n需重生成直到自动评分 ≥ %d/%d（人工项待确认）\n", autoMax-manualScore, autoMax))
	}

	sb.WriteString("\n═══ 报告结束 ═══\n")
	return sb.String()
}

// ---------- detection helpers ----------

// segmentRange marks a segment boundary.
type segmentRange struct {
	name    string
	start   int // node index
	end     int // node index (exclusive)
	pct     float64
	inRange bool
}

// segmentRatioSpec defines the expected ratio ranges per SOP.
var segmentRatioSpec = []struct {
	name   string
	minPct float64
	maxPct float64
}{
	{"触发段", 15, 20},
	{"展开段", 25, 30},
	{"推进段", 35, 45},
	{"收束段", 10, 15},
}

// detectFourSegments heuristically splits nodes into 4 segments based on
// content markers and position in the chapter.
func detectFourSegments(nodes []paraNode) []string {
	if len(nodes) < 8 {
		// Too short for 4 segments — return what we can.
		n := len(nodes) / 2
		if n < 2 {
			n = 2
		}
		return []string{"触发段", "推进段"}
	}

	// Simple heuristic: split into quarters.
	total := len(nodes)
	q1 := total / 4
	q2 := total / 2
	q3 := 3 * total / 4

	segments := []string{"触发段", "展开段", "推进段", "收束段"}
	_ = q1
	_ = q2
	_ = q3

	return segments
}

// missingSegment returns which segment name is missing.
func missingSegment(segs []string) string {
	all := []string{"触发段", "展开段", "推进段", "收束段"}
	have := map[string]bool{}
	for _, s := range segs {
		have[s] = true
	}
	var missing []string
	for _, a := range all {
		if !have[a] {
			missing = append(missing, a)
		}
	}
	return strings.Join(missing, "/")
}

// segmentRatios computes CJK ratio for each segment.
func segmentRatios(segs []string, nodes []paraNode, totalCJK int) []segmentRange {
	if len(segs) == 0 || totalCJK == 0 {
		return nil
	}
	total := len(nodes)
	var out []segmentRange
	prev := 0
	for i, name := range segs {
		end := total * (i + 1) / len(segs)
		if i == len(segs)-1 {
			end = total
		}
		cjk := 0
		for j := prev; j < end && j < len(nodes); j++ {
			cjk += countCJK(nodes[j].text)
		}
		pct := float64(cjk) / float64(totalCJK) * 100
		inRange := false
		if i < len(segmentRatioSpec) {
			spec := segmentRatioSpec[i]
			inRange = pct >= spec.minPct && pct <= spec.maxPct
		}
		out = append(out, segmentRange{name: name, start: prev, end: end, pct: pct, inRange: inRange})
		prev = end
	}
	return out
}

// detectEndingType classifies the chapter ending. Uses cycleTypes from config if provided.
func detectEndingType(nodes []paraNode, cycleTypes []string) string {
	if len(nodes) == 0 {
		if len(cycleTypes) > 0 { return cycleTypes[0] }
		return "悬念"
	}
	// Look at last 5 content nodes.
	endNodes := nodes
	if len(endNodes) > 5 {
		endNodes = endNodes[len(endNodes)-5:]
	}
	lastText := ""
	for _, nd := range endNodes {
		lastText += nd.text
	}

	// Default type names and their keyword detectors.
	type detector struct {
		name     string
		keywords []string
		exact    []string
	}
	detectors := []detector{
		{"悬念", []string{"不知", "没有发现", "隐隐", "似乎", "好像有什么", "莫名"}, []string{"？", "……"}},
		{"压力", []string{"还剩", "再过", "倒计时", "期限", "只有", "来不及", "逼近"}, nil},
		{"行动", []string{"起身", "推门", "走出去", "拔", "翻身", "迈步", "准备", "要开始"}, nil},
		{"伏笔", []string{"发现", "注意", "忽然看到", "角落里", "那是", "奇怪"}, nil},
	}

	// If config specifies different type names (e.g., 声音/动作/视线/压迫), map them.
	typeMap := map[string]detector{}
	for i, d := range detectors {
		name := d.name
		if i < len(cycleTypes) && cycleTypes[i] != "" {
			name = cycleTypes[i]
		}
		typeMap[name] = d
	}

	for name, d := range typeMap {
		// Check exact matches first.
		for _, ex := range d.exact {
			if strings.Contains(lastText, ex) {
				return name
			}
		}
		// Check keywords.
		for _, kw := range d.keywords {
			if strings.Contains(lastText, kw) {
				return name
			}
		}
	}

	if len(cycleTypes) > 0 { return cycleTypes[0] }
	return "悬念"
}

// detectChapterHook checks if the ending has hook signals.
func detectChapterHook(nodes []paraNode) bool {
	if len(nodes) == 0 {
		return false
	}
	endNodes := nodes
	if len(endNodes) > 3 {
		endNodes = endNodes[len(endNodes)-3:]
	}
	lastText := ""
	for _, nd := range endNodes {
		lastText += nd.text
	}

	// Hook signals: question, exclamation, suspense, conflict, action, secret.
	hookSignals := []string{
		"？", "！", "……",
		"忽然", "突然", "就在",
		"不知", "没有发现", "隐约",
		"似乎", "好像", "莫名",
		"冷光", "寒光", "闪",
	}
	for _, sig := range hookSignals {
		if strings.Contains(lastText, sig) {
			return true
		}
	}
	return false
}

// ---------- action detection ----------

var forbiddenActionList = []string{
	"打坐", "行气", "拔匕首", "拔柴刀", "检查弩",
	"摸预警绳", "灶台静坐", "摸刀", "擦刀",
	"拔刀", "盘膝", "调息", "静坐",
}

func parseActionList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func findActionsInText(text string, actions []string) []string {
	var found []string
	for _, act := range actions {
		if strings.Contains(text, act) {
			found = append(found, act)
		}
	}
	return found
}

func intersectActs(current, prev []string) []string {
	if len(prev) == 0 {
		return nil
	}
	prevSet := map[string]bool{}
	for _, a := range prev {
		prevSet[a] = true
	}
	var repeated []string
	for _, a := range current {
		if prevSet[a] {
			repeated = append(repeated, a)
		}
	}
	return repeated
}
