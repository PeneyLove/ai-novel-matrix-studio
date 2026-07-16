// Package reviewagent provides the LLM-powered chapter review agent — the
// replacement for v2.0's Python-based pattern-matching review tools. Instead
// of regex/string-scan heuristics, the agent uses structured prompts to
// instruct the LLM itself to analyse, expand, rewrite, or de-AI-ify content.
//
// Three review modes:
//
//   Expand (扩写) — Enrich existing prose with more detail, description,
//   sensory input, internal monologue, or scene-setting. The agent identifies
//   "thin" passages and suggests concrete expansions.
//
//   Rewrite (改写) — Rewrite a passage according to user-specified
//   requirements (change POV, adjust tone, compress, restructure, change
//   style). This mode is interactive: the agent first analyses the text and
//   asks clarifying questions before rewriting.
//
//   DeAI (去AI化) — Detect and remove common AI-writing fingerprints:
//   formulaic transition phrases, overly balanced paragraph structures,
//   repetitive sentence openings, excessive hedging, generic emotional
//   beats, and "both sides" boilerplate. Produces a cleaned version with
//   an audit trail of what was changed and why.
//
// Design principle: the agent builds prompts; the LLM does the work.
// This package is call-site agnostic — the tool layer wires it to the
// actual model call.
package reviewagent

import (
	"fmt"
	"strings"
)

// ─── Review Mode ────────────────────────────────────────────────────────────

// Mode selects which review operation to perform.
type Mode string

const (
	ModeExpand Mode = "expand" // 扩写
	ModeRewrite Mode = "rewrite" // 改写
	ModeDeAI   Mode = "deai"    // 去AI化
)

// ─── AI Pattern Catalogue ───────────────────────────────────────────────────

// AIPattern describes a detectable AI-writing fingerprint.
type AIPattern struct {
	Name        string   // human-readable name (e.g. "万能过渡句")
	Description string   // what it looks like
	Examples    []string // concrete examples of the pattern
	Severity    string   // high / medium / low
}

// DefaultAIPatterns is the built-in catalogue of AI-writing fingerprints
// for Chinese web-novel text. Each pattern targets a specific class of
// detectable AI "slop".
func DefaultAIPatterns() []AIPattern {
	return []AIPattern{
		{
			Name:        "万能过渡句",
			Description: "模板化过渡句式，如'与此同时'、'值得一提的是'、'不可否认的是'、'总而言之'在非总结位置频繁出现",
			Examples:    []string{"与此同时，他的心中涌起了一股难以言喻的情绪。", "值得一提的是，这一切都在他的预料之中。"},
			Severity:    "high",
		},
		{
			Name:        "对称句式堆砌",
			Description: "连续使用'一方面…另一方面…'、'既…又…'、'不仅…而且…'等对偶结构，造成机械感",
			Examples:    []string{"他既感到愤怒，又感到无奈。一方面想要报复，另一方面又害怕后果。"},
			Severity:    "high",
		},
		{
			Name:        "空洞情绪描述",
			Description: "使用'心中涌起一股…'、'一种难以言喻的…'、'复杂的情感交织'等万能情绪模板代替具体描写",
			Examples:    []string{"他的心中涌起一股复杂的情绪。", "一种难以言喻的感觉席卷了她。"},
			Severity:    "high",
		},
		{
			Name:        "段落收束公式化",
			Description: "每段以总结性语句收尾，如'这一刻，他终于明白了…'、'从此，一切都变了…'",
			Examples:    []string{"这一刻，他终于明白了什么叫真正的力量。", "从这一天起，一切都变得不同了。"},
			Severity:    "medium",
		},
		{
			Name:        "动作链过于规整",
			Description: "连续三个以上'先…然后…接着…最后…'的结构化动作描述，像流程文档",
			Examples:    []string{"他先深吸一口气，然后缓缓抬起手，接着凝聚灵力，最后猛地向前一推。"},
			Severity:    "medium",
		},
		{
			Name:        "形容词膨胀",
			Description: "名词前堆砌两个以上修饰语，如'无比强大的恐怖的力量'、'深邃而神秘而又令人敬畏的目光'",
			Examples:    []string{"那是一股无比强大的、令人窒息的、仿佛来自九幽深处的恐怖气息。"},
			Severity:    "medium",
		},
		{
			Name:        "对话标签重复",
			Description: "每句对话后都跟'XX道'、'XX说'、'XX淡淡道'等标签，缺少动作替代",
			Examples:    []string{"\"我知道了。\"叶凡道。\"那就这样吧。\"林婉儿说。"},
			Severity:    "low",
		},
		{
			Name:        "解释性旁白",
			Description: "在角色动作后紧跟'因为…所以…'的解释性文字，代替让读者自行理解",
			Examples:    []string{"他握紧了拳头，因为这是他第一次感受到真正的威胁，所以他决定不再隐藏实力。"},
			Severity:    "low",
		},
	}
}

// ─── Review Input ───────────────────────────────────────────────────────────

// Input is the material the review agent works with.
type Input struct {
	Mode       Mode     // which operation
	Content    string   // full chapter text (or passage) to review
	ChapterNum int      // chapter number for context
	Genre      string   // 玄幻/都市/古言/悬疑/科幻/甜宠
	// For Rewrite mode, the user's specific requirements (optional initially)
	RewriteBrief string
	// For Expand mode, target word-count multiplier (e.g. 1.5 = 50% expansion)
	ExpandRatio float64
}

// ─── Review Output ──────────────────────────────────────────────────────────

// Output is the structured result of a review operation.
type Output struct {
	Mode        Mode     `json:"mode"`
	OriginalLen int      `json:"original_len"` // character count of input
	ResultLen   int      `json:"result_len"`   // character count of output

	// For Expand / DeAI modes: the processed text.
	ProcessedText string `json:"processed_text,omitempty"`

	// Audit trail: what was changed and why.
	Changes []Change `json:"changes,omitempty"`

	// For Rewrite mode: dialogue questions to ask the user.
	Questions []RewriteQuestion `json:"questions,omitempty"`

	// For Rewrite mode after user response: the rewritten text.
	RewrittenText string `json:"rewritten_text,omitempty"`

	// Summary stats for DeAI mode.
	DetectedPatterns []PatternHit `json:"detected_patterns,omitempty"`
}

// Change records one edit made during review.
type Change struct {
	Location string `json:"location"` // line or paragraph reference
	Before   string `json:"before"`   // original snippet (≤ 80 chars)
	After    string `json:"after"`    // replacement snippet (≤ 80 chars)
	Reason   string `json:"reason"`   // why this change was made
	Pattern  string `json:"pattern"`  // which AI pattern triggered it (DeAI only)
}

// PatternHit records a detected AI pattern with count and examples.
type PatternHit struct {
	Pattern string   `json:"pattern"`
	Count   int      `json:"count"`
	Examples []string `json:"examples"` // up to 3 snippets
	Severity string  `json:"severity"`
}

// RewriteQuestion is one clarifying question for the interactive rewrite flow.
type RewriteQuestion struct {
	ID      string   `json:"id"`
	Question string  `json:"question"`
	Options  []string `json:"options"` // 2-4 concrete options
	Default  string   `json:"default"`
}

// ─── Prompt Builders ────────────────────────────────────────────────────────

const reviewSystemPrompt = `【角色锁定】你是专业的网文内容审查Agent。你的任务是对给定的网文章节进行结构化审查和优化。
你必须严格遵循当前模式的指令，不跨模式操作，不擅自改变文本的剧情走向和核心信息。
所有输出使用简体中文。`

// BuildExpandPrompt builds the prompt for expand mode.
func BuildExpandPrompt(in Input) string {
	var b strings.Builder
	b.WriteString(reviewSystemPrompt)
	b.WriteString("\n\n## 任务：扩写\n\n")
	b.WriteString("对以下网文章节进行扩写。扩写原则：\n")
	b.WriteString("1. 保持原有剧情走向和对话内容不变\n")
	b.WriteString("2. 在动作场景中增加感官细节（视觉/听觉/触觉/嗅觉）\n")
	b.WriteString("3. 在对话间增加角色的微表情、肢体动作、内心活动\n")
	b.WriteString("4. 在场景转换处增加环境描写（1-3句即可，勿过度）\n")
	b.WriteString("5. 控制战斗场景节奏：关键回合详写，过渡回合略写\n")
	b.WriteString(fmt.Sprintf("6. 目标扩写比例：约 %.0f%%（原文 %d 字）\n", (in.ExpandRatio-1)*100, len([]rune(in.Content))))
	b.WriteString("\n## 输出格式\n")
	b.WriteString("先输出完整的扩写后正文，然后在末尾用 `---` 分隔，列出变更摘要：\n")
	b.WriteString("```\n- [位置] 原文片段 → 扩写内容 (扩写原因)\n```\n\n")
	b.WriteString("## 正文\n\n")
	b.WriteString(in.Content)
	return b.String()
}

// BuildRewriteAnalyzePrompt builds the first-round prompt for rewrite mode:
// analyses the text and asks clarifying questions.
func BuildRewriteAnalyzePrompt(in Input) string {
	var b strings.Builder
	b.WriteString(reviewSystemPrompt)
	b.WriteString("\n\n## 任务：改写 — 需求分析阶段\n\n")
	b.WriteString("你是改写Agent的第一阶段：分析原文并确定改写方向。\n\n")
	if in.RewriteBrief != "" {
		b.WriteString(fmt.Sprintf("## 用户初步需求\n%s\n\n", in.RewriteBrief))
	}
	b.WriteString("## 分析要求\n")
	b.WriteString("阅读以下正文，从以下维度分析当前文本特征，每项给出 1-2 句话的判断：\n")
	b.WriteString("1. POV视角（当前是第几人称？视角是否一致？）\n")
	b.WriteString("2. 叙事距离（远近/冷暖/客观主观）\n")
	b.WriteString("3. 对话密度与风格（对话占比、语气特征）\n")
	b.WriteString("4. 节奏（快/慢/跳跃/平铺）\n")
	b.WriteString("5. 描写密度（环境/动作/心理的比例）\n")
	b.WriteString("6. AI痕迹（是否检测到模板化句式）\n\n")
	b.WriteString("## 输出：改写建议问卷\n")
	b.WriteString("基于以上分析，生成 3-5 个改写方向问题，每个问题附带 2-4 个具体选项。格式如下：\n\n")
	b.WriteString("```json\n")
	b.WriteString(`{
  "analysis": "2-3句话的文本特征总结",
  "questions": [
    {
      "id": "q1",
      "question": "改写方向问题",
      "options": ["选项A描述", "选项B描述", "选项C描述"],
      "default": "选项A描述"
    }
  ]
}
`)
	b.WriteString("```\n\n")
	b.WriteString("## 正文\n\n")
	b.WriteString(in.Content)
	return b.String()
}

// BuildRewriteExecutePrompt builds the second-round prompt for rewrite mode:
// applies the user's chosen options to produce the rewritten text.
func BuildRewriteExecutePrompt(in Input, answers map[string]string) string {
	var b strings.Builder
	b.WriteString(reviewSystemPrompt)
	b.WriteString("\n\n## 任务：改写 — 执行阶段\n\n")
	b.WriteString("根据用户确认的改写方向，对正文进行改写。\n\n")
	b.WriteString("## 用户确认的改写方向\n")
	for qid, ans := range answers {
		b.WriteString(fmt.Sprintf("- %s: %s\n", qid, ans))
	}
	b.WriteString("\n## 改写原则\n")
	b.WriteString("1. 保持核心剧情和关键对话不变\n")
	b.WriteString("2. 按用户选择的风格方向调整叙事方式\n")
	b.WriteString("3. 删除或替换AI模板化表达\n")
	b.WriteString("4. 保持章节字数在合理范围内（±20%）\n\n")
	b.WriteString("## 输出格式\n")
	b.WriteString("先输出完整的改写后正文，末尾用 `---` 分隔，列出变更摘要。\n\n")
	b.WriteString("## 正文\n\n")
	b.WriteString(in.Content)
	return b.String()
}

// BuildDeAIPrompt builds the prompt for de-AI mode.
func BuildDeAIPrompt(in Input) string {
	var b strings.Builder
	b.WriteString(reviewSystemPrompt)
	b.WriteString("\n\n## 任务：去AI化\n\n")
	b.WriteString("你是专业的网文去AI化Agent。你的任务是检测并修复以下AI写作痕迹，\n")
	b.WriteString("输出自然的人类写作风格的文本。\n\n")

	b.WriteString("## AI痕迹检测清单\n\n")
	patterns := DefaultAIPatterns()
	for i, p := range patterns {
		b.WriteString(fmt.Sprintf("### %d. %s [%s]\n", i+1, p.Name, p.Severity))
		b.WriteString(fmt.Sprintf("%s\n", p.Description))
		b.WriteString("示例：\n")
		for _, ex := range p.Examples {
			b.WriteString(fmt.Sprintf("- %s\n", ex))
		}
		b.WriteString("\n")
	}

	b.WriteString("## 修复原则\n")
	b.WriteString("1. 万能过渡句 → 用具体的动作、环境、或对话承接场景转换\n")
	b.WriteString("2. 对称句式 → 打破对偶，改为长短句交替\n")
	b.WriteString("3. 空洞情绪 → 用具体的生理反应+行为替代抽象描述\n")
	b.WriteString("4. 公式化收束 → 用画面或动作结尾替代总结句\n")
	b.WriteString("5. 规整动作链 → 打断顺序，插入环境/心理/对话\n")
	b.WriteString("6. 形容词膨胀 → 保留最精准的1个修饰语，其余删除\n")
	b.WriteString("7. 对话标签重复 → 用动作描写替代50%以上的'XX道'\n")
	b.WriteString("8. 解释性旁白 → 删除因果连词，改为动作+留白\n\n")
	b.WriteString("## 核心原则：保留原意，只修表达。不改剧情，不改人物，不改对话内容。\n\n")

	b.WriteString("## 输出格式\n")
	b.WriteString("先输出完整的去AI化后正文。然后 `---` 分隔，输出检测报告：\n\n")
	b.WriteString("```json\n")
	b.WriteString(`{
  "detected_patterns": [
    {"pattern": "万能过渡句", "count": 5, "examples": ["例1", "例2"], "severity": "high"}
  ],
  "changes": [
    {"location": "第3段", "before": "原文片段", "after": "修改后片段", "reason": "替换万能过渡句", "pattern": "万能过渡句"}
  ]
}
`)
	b.WriteString("```\n\n")
	b.WriteString("## 正文\n\n")
	b.WriteString(in.Content)
	return b.String()
}

// ─── Rewrite Dialogue Helper ────────────────────────────────────────────────

// RewriteDialogue manages the interactive rewrite Q&A flow.
// It holds the original input and accumulated user answers.
type RewriteDialogue struct {
	Input      Input
	Stage      string            // "analyze" | "confirm" | "execute"
	Questions  []RewriteQuestion  // questions generated from analysis
	Answers    map[string]string  // user's answers
}

// NewRewriteDialogue starts a new rewrite dialogue.
func NewRewriteDialogue(in Input) *RewriteDialogue {
	return &RewriteDialogue{
		Input:   in,
		Stage:   "analyze",
		Answers: make(map[string]string),
	}
}

// SetQuestions records the questions generated by the LLM analysis phase.
func (d *RewriteDialogue) SetQuestions(qs []RewriteQuestion) {
	d.Questions = qs
	d.Stage = "confirm"
}

// RecordAnswer records one user answer and checks if all questions are answered.
func (d *RewriteDialogue) RecordAnswer(qid, answer string) bool {
	d.Answers[qid] = answer
	return len(d.Answers) >= len(d.Questions)
}

// IsComplete reports whether the dialogue has enough answers to proceed to execution.
func (d *RewriteDialogue) IsComplete() bool {
	return len(d.Answers) >= len(d.Questions) && len(d.Questions) > 0
}

// BuildConfirmPrompt builds a prompt that presents the questions to the user
// in a readable format (for display in TUI).
func (d *RewriteDialogue) BuildConfirmPrompt() string {
	var b strings.Builder
	b.WriteString("## 改写确认\n\n")
	b.WriteString("请回答以下问题以确定改写方向：\n\n")
	for _, q := range d.Questions {
		b.WriteString(fmt.Sprintf("### %s\n", q.Question))
		for i, opt := range q.Options {
			marker := "  "
			if opt == q.Default {
				marker = "→ "
			}
			b.WriteString(fmt.Sprintf("%s%d. %s\n", marker, i+1, opt))
		}
		b.WriteString("\n")
	}
	return b.String()
}
