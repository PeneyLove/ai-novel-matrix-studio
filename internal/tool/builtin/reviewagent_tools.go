package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/reviewagent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(reviewExpand{})
	tool.RegisterBuiltin(reviewRewrite{})
	tool.RegisterBuiltin(reviewDeAI{})
	tool.RegisterBuiltin(reviewRepair{})
}

// ─── review_expand (扩写) ──────────────────────────────────────────────────

type reviewExpand struct{}

type reviewExpandArgs struct {
	Content     string  `json:"content"`      // 待扩写章节全文
	ChapterNum  int     `json:"chapter_num"`  // 章节号
	Genre       string  `json:"genre"`        // 赛道：玄幻/都市/古言/科幻/悬疑/甜宠
	ExpandRatio float64 `json:"expand_ratio"` // 扩写比例，默认1.5（+50%）
}

func (r reviewExpand) Name() string { return "review_expand" }

func (r reviewExpand) Description() string {
	return "扩写章节内容：对原文进行感官细节补充、场景环境描写、对话微表情增强。保持原有剧情走向和对话不变，目标为增加描写密度和阅读沉浸感。"
}

func (r reviewExpand) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "待扩写的章节全文（先用 read_file 读取）"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签：玄幻/都市/古言/科幻/悬疑/甜宠"},
			"expand_ratio": {"type": "number", "description": "扩写比例，1.3=30%扩展，1.5=50%扩展，默认1.5"}
		},
		"required": ["content", "chapter_num", "genre"]
	}`)
}

func (r reviewExpand) ReadOnly() bool { return true }

func (r reviewExpand) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewExpandArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_expand: invalid args: %w", err)
	}
	if p.Content == "" {
		return "", fmt.Errorf("review_expand: content is empty — read the file first")
	}
	if p.ExpandRatio <= 0 {
		p.ExpandRatio = 1.5
	}

	in := reviewagent.Input{
		Mode:        reviewagent.ModeExpand,
		Content:     p.Content,
		ChapterNum:  p.ChapterNum,
		Genre:       p.Genre,
		ExpandRatio: p.ExpandRatio,
	}

	return reviewagent.BuildExpandPrompt(in), nil
}

// ─── review_rewrite (改写) ──────────────────────────────────────────────────

type reviewRewrite struct{}

type reviewRewriteArgs struct {
	Content    string            `json:"content"`     // 待改写章节全文
	ChapterNum int               `json:"chapter_num"` // 章节号
	Genre      string            `json:"genre"`       // 赛道标签
	Brief      string            `json:"brief"`       // 用户初步改写需求（可选）
	Stage      string            `json:"stage"`       // analyze | execute
	Answers    map[string]string `json:"answers"`     // 用户对改写问题的回答（execute阶段）
}

func (r reviewRewrite) Name() string { return "review_rewrite" }

func (r reviewRewrite) Description() string {
	return "改写章节内容：两阶段交互式改写。第一阶段（stage=analyze）：分析原文特征并生成改写方向问卷；第二阶段（stage=execute）：根据用户回答执行改写。改写需要和用户对话确认需求，不是自动覆盖。"
}

func (r reviewRewrite) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "待改写的章节全文"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签"},
			"brief": {"type": "string", "description": "用户的初步改写需求，如'改成第一人称'、'删掉感情线'"},
			"stage": {"type": "string", "enum": ["analyze", "execute"], "description": "analyze=分析原文并生成改写问卷；execute=根据用户回答执行改写"},
			"answers": {"type": "object", "description": "用户在改写问卷中的回答，key为问题ID，value为选择的选项（execute阶段必填）"}
		},
		"required": ["content", "chapter_num", "genre"]
	}`)
}

func (r reviewRewrite) ReadOnly() bool { return true }

func (r reviewRewrite) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewRewriteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_rewrite: invalid args: %w", err)
	}
	if p.Content == "" {
		return "", fmt.Errorf("review_rewrite: content is empty — read the file first")
	}

	in := reviewagent.Input{
		Mode:         reviewagent.ModeRewrite,
		Content:      p.Content,
		ChapterNum:   p.ChapterNum,
		Genre:        p.Genre,
		RewriteBrief: p.Brief,
	}

	switch p.Stage {
	case "execute":
		if len(p.Answers) == 0 {
			return "", fmt.Errorf("review_rewrite: stage=execute requires answers from the analyze phase")
		}
		return reviewagent.BuildRewriteExecutePrompt(in, p.Answers), nil
	default:
		// stage=analyze or empty
		d := reviewagent.NewRewriteDialogue(in)
		prompt := reviewagent.BuildRewriteAnalyzePrompt(in)
		// Append a note about how to use the output
		prompt += "\n\n---\n"
		prompt += "使用说明：请将以上prompt发送给LLM，获取JSON格式的问卷。\n"
		prompt += "然后使用 request_user_input 向用户展示问卷。\n"
		prompt += fmt.Sprintf("收集用户回答后，再次调用 review_rewrite，设置 stage=execute 并传入 answers。\n")
		_ = d
		return prompt, nil
	}
}

// ─── review_deai (去AI化) ──────────────────────────────────────────────────

type reviewDeAI struct{}

type reviewDeAIArgs struct {
	Content    string `json:"content"`     // 待去AI化章节全文
	ChapterNum int    `json:"chapter_num"` // 章节号
	Genre      string `json:"genre"`       // 赛道标签
}

func (r reviewDeAI) Name() string { return "review_deai" }

func (r reviewDeAI) Description() string {
	return "去AI化处理：检测并修复8类AI写作痕迹（万能过渡句、对称句式、空洞情绪、公式收束、规整动作链、形容词膨胀、对话标签重复、解释性旁白），输出自然人类写作风格的文本+检测报告。"
}

func (r reviewDeAI) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "待去AI化的章节全文（先用 read_file 读取）"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签"}
		},
		"required": ["content"]
	}`)
}

func (r reviewDeAI) ReadOnly() bool { return true }

func (r reviewDeAI) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewDeAIArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_deai: invalid args: %w", err)
	}
	if p.Content == "" {
		return "", fmt.Errorf("review_deai: content is empty — read the file first")
	}

	in := reviewagent.Input{
		Mode:       reviewagent.ModeDeAI,
		Content:    p.Content,
		ChapterNum: p.ChapterNum,
		Genre:      p.Genre,
	}

	return reviewagent.BuildDeAIPrompt(in), nil
}

// ─── review_repair (改文修复) ───────────────────────────────────────────────

type reviewRepair struct{}

type reviewRepairArgs struct {
	Content     string   `json:"content"`      // 待改文章节全文
	ChapterNum  int      `json:"chapter_num"`  // 章节号
	Genre       string   `json:"genre"`        // 赛道标签
	Stage       string   `json:"stage"`        // diagnose | execute
	RepairFixes []string `json:"repair_fixes"` // 用户确认的修复项列表（execute阶段）
}

func (r reviewRepair) Name() string { return "review_repair" }

func (r reviewRepair) Description() string {
	return "改文修复：对数据差的小说章节进行8维度量化诊断（开头钩子/节奏/章末钩子/AI痕迹/对话/描写/信息密度/分段），输出评分+TOP问题列表，然后根据用户确认执行针对性改写。两阶段：stage=diagnose 先诊断，stage=execute 再执行改写。"
}

func (r reviewRepair) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "待诊断/改写的章节全文（先用 read_file 读取）"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签：玄幻/都市/古言/科幻/悬疑/甜宠"},
			"stage": {"type": "string", "enum": ["diagnose", "execute"], "description": "diagnose=8维度诊断+评分+定位TOP问题；execute=根据用户确认的修复项执行改写"},
			"repair_fixes": {"type": "array", "items": {"type": "string"}, "description": "用户在诊断阶段确认要修复的问题列表（execute阶段必填）"}
		},
		"required": ["content", "stage"]
	}`)
}

func (r reviewRepair) ReadOnly() bool { return true }

func (r reviewRepair) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewRepairArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_repair: invalid args: %w", err)
	}
	if p.Content == "" {
		return "", fmt.Errorf("review_repair: content is empty — read the file first")
	}

	in := reviewagent.Input{
		Mode:        reviewagent.ModeRepair,
		Content:     p.Content,
		ChapterNum:  p.ChapterNum,
		Genre:       p.Genre,
		RepairFixes: p.RepairFixes,
	}

	switch p.Stage {
	case "execute":
		if len(p.RepairFixes) == 0 {
			return "", fmt.Errorf("review_repair: stage=execute requires repair_fixes from the diagnose phase")
		}
		return reviewagent.BuildRepairExecutePrompt(in), nil
	default:
		return reviewagent.BuildRepairDiagnosePrompt(in), nil
	}
}
