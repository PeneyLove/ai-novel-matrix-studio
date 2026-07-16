package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/novelwriter"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/recorderagent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/reviewagent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(recorderIntegrate{})
	tool.RegisterBuiltin(recorderConsult{})
	tool.RegisterBuiltin(novelwriterGenerate{})
	tool.RegisterBuiltin(novelwriterRewrite{})
	tool.RegisterBuiltin(reviewVerdict{})
}

// ─── recorder_integrate ─────────────────────────────────────────────────────

type recorderIntegrate struct{}

type recorderIntegrateArgs struct {
	ReactionsJSON   string `json:"reactions_json"`    // JSON array of characteragent.Reaction
	TriggerEventDesc string `json:"trigger_event_desc"` // trigger event description
	ChapterNum      int    `json:"chapter_num"`
	ProtagonistID   string `json:"protagonist_id"`
}

func (r recorderIntegrate) Name() string { return "recorder_integrate" }

func (r recorderIntegrate) Description() string {
	return "记录Agent整合模式：收集所有角色Agent的并行输出，排序事件、检测冲突、提取对话、生成结构化场景描述。输出结果供网文Agent成文使用。"
}

func (r recorderIntegrate) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"reactions_json": {"type": "string", "description": "所有角色反应的JSON数组"},
			"trigger_event_desc": {"type": "string", "description": "触发事件描述"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"protagonist_id": {"type": "string", "description": "主角节点ID"}
		},
		"required": ["reactions_json", "chapter_num"]
	}`)
}

func (r recorderIntegrate) ReadOnly() bool { return true }

func (r recorderIntegrate) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p recorderIntegrateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("recorder_integrate: %w", err)
	}

	var reactions []characteragent.Reaction
	if err := json.Unmarshal([]byte(p.ReactionsJSON), &reactions); err != nil {
		return "", fmt.Errorf("recorder_integrate: parse reactions: %w", err)
	}

	trigger := characteragent.TriggerEvent{
		Description: p.TriggerEventDesc,
		ChapterNum:  p.ChapterNum,
	}

	agent := recorderagent.New(nil, p.ProtagonistID)
	scene := agent.Integrate(recorderagent.IntegrateInput{
		ChapterNum:   p.ChapterNum,
		Reactions:    reactions,
		TriggerEvent: trigger,
	})

	data, _ := json.MarshalIndent(scene, "", "  ")
	return fmt.Sprintf("场景整合完成：%d个事件，%d个冲突，%d条对话。\n\n```json\n%s\n```", len(scene.EventSeq), len(scene.Conflicts), len(scene.Dialogues), string(data)), nil
}

// ─── recorder_consult ───────────────────────────────────────────────────────

type recorderConsult struct{}

type recorderConsultArgs struct {
	SceneJSON string `json:"scene_json"` // recorder_integrate output JSON
	QueryType string `json:"query_type"` // scene_recall | character_state | conflict_confirm | omission_check
	Question  string `json:"question"`   // natural language question
	Target    string `json:"target"`     // target character/event/scene
}

func (r recorderConsult) Name() string { return "recorder_consult" }

func (r recorderConsult) Description() string {
	return "记录Agent顾问模式：网文Agent在修改稿子时向记录Agent查询原始推演数据。支持4种查询：场景回溯、角色状态、冲突确认、遗漏检查。"
}

func (r recorderConsult) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"scene_json": {"type": "string", "description": "recorder_integrate 输出的场景JSON"},
			"query_type": {"type": "string", "enum": ["scene_recall", "character_state", "conflict_confirm", "omission_check"]},
			"question": {"type": "string", "description": "自然语言查询问题"},
			"target": {"type": "string", "description": "查询目标（角色名/事件描述/场景名）"}
		},
		"required": ["scene_json", "query_type", "target"]
	}`)
}

func (r recorderConsult) ReadOnly() bool { return true }

func (r recorderConsult) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p recorderConsultArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("recorder_consult: %w", err)
	}

	var scene recorderagent.Scene
	if err := json.Unmarshal([]byte(p.SceneJSON), &scene); err != nil {
		return "", fmt.Errorf("recorder_consult: parse scene: %w", err)
	}

	agent := recorderagent.New(nil, "")
	query := recorderagent.ConsultantQuery{
		Type:     p.QueryType,
		Question: p.Question,
		Target:   p.Target,
	}
	ans := agent.Consult(query, &scene)

	data, _ := json.MarshalIndent(ans, "", "  ")
	return fmt.Sprintf("顾问查询结果：\n```json\n%s\n```", string(data)), nil
}

// ─── novelwriter_generate ───────────────────────────────────────────────────

type novelwriterGenerate struct{}

type novelwriterGenerateArgs struct {
	SceneJSON  string `json:"scene_json"`  // recorder_integrate output
	ChapterNum int    `json:"chapter_num"`
	Genre      string `json:"genre"`       // 玄幻/都市/古言/科幻/悬疑/甜宠
	StyleGuide string `json:"style_guide"` // optional user style preferences
}

func (n novelwriterGenerate) Name() string { return "novelwriter_generate" }

func (n novelwriterGenerate) Description() string {
	return "网文Agent成文模式：将记录Agent的结构化场景描述转化为流畅网文章节。注入角色对话风格、四拍节奏、赛道特色，避免AI模板化表达。"
}

func (n novelwriterGenerate) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"scene_json": {"type": "string", "description": "recorder_integrate 输出的场景JSON"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道：玄幻/都市/古言/科幻/悬疑/甜宠"},
			"style_guide": {"type": "string", "description": "用户风格偏好（可选）"}
		},
		"required": ["scene_json", "chapter_num", "genre"]
	}`)
}

func (n novelwriterGenerate) ReadOnly() bool { return true }

func (n novelwriterGenerate) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p novelwriterGenerateArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("novelwriter_generate: %w", err)
	}

	var scene recorderagent.Scene
	if err := json.Unmarshal([]byte(p.SceneJSON), &scene); err != nil {
		return "", fmt.Errorf("novelwriter_generate: parse scene: %w", err)
	}

	agent := novelwriter.New(p.Genre)
	agent.SetStyleGuide(p.StyleGuide)
	minW, maxW := agent.TargetWordRange()

	prompt := agent.BuildGeneratePrompt(novelwriter.GenerateInput{
		ChapterNum: p.ChapterNum,
		Scene:      &scene,
		Round:      0,
	})

	return fmt.Sprintf("网文Agent成文提示词已生成。\n赛道: %s | 目标字数: %d-%d | 场景事件: %d个\n\n将以下提示词发送给LLM以生成正文：\n\n%s",
		p.Genre, minW, maxW, len(scene.EventSeq), prompt), nil
}

// ─── novelwriter_rewrite ────────────────────────────────────────────────────

type novelwriterRewrite struct{}

type novelwriterRewriteArgs struct {
	CurrentDraft string `json:"current_draft"` // chapter text to rewrite
	IssuesJSON   string `json:"issues_json"`   // JSON array of VerdictIssue from review_verdict
	SceneJSON    string `json:"scene_json"`    // original scene (for reference)
	ConsultCtx   string `json:"consult_ctx"`   // consultant context from recorder_consult (optional)
	ChapterNum   int    `json:"chapter_num"`
	Genre        string `json:"genre"`
	Round        int    `json:"round"`         // 1 or 2
}

func (n novelwriterRewrite) Name() string { return "novelwriter_rewrite" }

func (n novelwriterRewrite) Description() string {
	return "网文Agent修改模式：根据审核Agent的打回问题清单逐条修改章节。可接入记录Agent顾问数据辅助修改，确保不偏离原始推演。"
}

func (n novelwriterRewrite) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"current_draft": {"type": "string", "description": "当前被驳回的章节正文"},
			"issues_json": {"type": "string", "description": "review_verdict 输出的 issues JSON 数组"},
			"scene_json": {"type": "string", "description": "原始场景JSON（recorder_integrate输出，用于参考）"},
			"consult_ctx": {"type": "string", "description": "记录Agent顾问查询结果（可选，用于剧情遗漏/人设冲突问题的修改参考）"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签"},
			"round": {"type": "integer", "description": "修改轮次（1或2）"}
		},
		"required": ["current_draft", "issues_json", "chapter_num", "genre", "round"]
	}`)
}

func (n novelwriterRewrite) ReadOnly() bool { return true }

func (n novelwriterRewrite) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p novelwriterRewriteArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("novelwriter_rewrite: %w", err)
	}

	var issues []novelwriter.IssueItem
	if err := json.Unmarshal([]byte(p.IssuesJSON), &issues); err != nil {
		return "", fmt.Errorf("novelwriter_rewrite: parse issues: %w", err)
	}

	var scene *recorderagent.Scene
	if p.SceneJSON != "" {
		scene = &recorderagent.Scene{}
		if err := json.Unmarshal([]byte(p.SceneJSON), scene); err != nil {
			scene = nil
		}
	}

	agent := novelwriter.New(p.Genre)
	prompt := agent.BuildRewritePrompt(novelwriter.RewriteInput{
		ChapterNum:    p.ChapterNum,
		CurrentDraft:  p.CurrentDraft,
		Issues:        issues,
		Scene:         scene,
		Round:         p.Round,
		ConsultantCtx: p.ConsultCtx,
	})

	return fmt.Sprintf("网文Agent修改提示词已生成（第%d轮修改，%d个问题）。\n\n将以下提示词发送给LLM以生成修改稿：\n\n%s",
		p.Round, len(issues), prompt), nil
}

// ─── review_verdict ─────────────────────────────────────────────────────────

type reviewVerdict struct{}

type reviewVerdictArgs struct {
	Content    string `json:"content"`     // chapter draft
	ChapterNum int    `json:"chapter_num"`
	Genre      string `json:"genre"`
	Round      int    `json:"round"`       // 1-3
}

func (r reviewVerdict) Name() string { return "review_verdict" }

func (r reviewVerdict) Description() string {
	return "审核Agent判决模式：对网文章节进行四维打分（字数/流畅度/AI痕迹/一致性），给出PASS/REJECT判决。REJECT时附带结构化问题清单供网文Agent修改。"
}

func (r reviewVerdict) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "待审核的章节正文"},
			"chapter_num": {"type": "integer", "description": "章节号"},
			"genre": {"type": "string", "description": "赛道标签"},
			"round": {"type": "integer", "description": "审核轮次（1-3），第3轮强制PASS"}
		},
		"required": ["content", "genre"]
	}`)
}

func (r reviewVerdict) ReadOnly() bool { return true }

func (r reviewVerdict) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p reviewVerdictArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("review_verdict: %w", err)
	}
	if p.Round <= 0 {
		p.Round = 1
	}
	if p.Content == "" {
		return "", fmt.Errorf("review_verdict: content is empty")
	}

	// Run deterministic evaluation
	result := reviewagent.Evaluate(p.Content, p.Genre, p.Round)
	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	// Also include the LLM prompt for deeper analysis
	llmPrompt := reviewagent.BuildVerdictPrompt(p.Content, p.Genre, p.ChapterNum, p.Round)

	var out string
	out += fmt.Sprintf("## 审核判决: %s (评分: %d/100)\n\n", result.Verdict, result.Score)
	out += fmt.Sprintf("%s\n\n", result.Summary)
	out += fmt.Sprintf("```json\n%s\n```\n\n", string(resultJSON))

	if result.Verdict == reviewagent.VerdictReject {
		out += "## 下一步\n"
		out += "1. 使用 recorder_consult 查询原始场景数据\n"
		out += "2. 使用 novelwriter_rewrite 进行修改\n"
		out += fmt.Sprintf("3. 再次运行 review_verdict（round=%d）\n", p.Round+1)
	}

	out += "\n---\n## LLM深度审核提示词\n\n" + llmPrompt

	return out, nil
}
