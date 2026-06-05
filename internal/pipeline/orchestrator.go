// Package pipeline implements the creation pipeline orchestration.
//
// v2.0 — updated for Prompt.md comprehensive architecture:
//   - 4-phase enforcement (init → outline+hooks → writing → optimize)
//   - NovelState injection into prompt templates via {{.NovelState}}
//   - Hook ledger summarization via {{.HookSummary}}
//   - Output header validation (every response must declare skill)
//   - Prerequisites checking before writing phase
package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/global"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/skill"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/state"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

// Orchestrator runs pipeline tasks against a skill and model router.
type Orchestrator struct {
	root   string
	router *model.Router
	mgr    *skill.Manager

	// Global rules injected into every system prompt
	GlobalRules global.Rules

	// Cached state per novel
	states map[string]*state.NovelState

	HotKeywords   map[string][]string
	CorpusSamples map[string]string
}

// NewOrchestrator creates an Orchestrator for the given .novelAgent root.
func NewOrchestrator(root string, router *model.Router, mgr *skill.Manager) *Orchestrator {
	return &Orchestrator{
		root:          root,
		router:        router,
		mgr:           mgr,
		GlobalRules:   global.DefaultRules(),
		states:        make(map[string]*state.NovelState),
		HotKeywords:   defaultKeywords(),
		CorpusSamples: loadCorpusSamples(root),
	}
}

func defaultKeywords() map[string][]string {
	return map[string][]string{
		"xuanhuan":  {"修仙", "渡劫", "逆袭", "宗门", "天道", "法宝", "境界", "飞升"},
		"dushi":     {"打脸", "逆袭", "神医", "战神", "豪门", "赘婿", "系统", "签到"},
		"guyan":     {"宫斗", "权谋", "重生", "穿越", "嫡女", "王爷", "朝堂", "翻盘"},
		"xuanyi":    {"悬疑", "反转", "诡计", "密室", "怪谈", "探案", "真相", "恐怖"},
		"kehuan":    {"末世", "副本", "无限", "进化", "星际", "系统", "生存", "赛博"},
		"tianchong": {"甜宠", "霸总", "暗恋", "破镜重圆", "青梅竹马", "追妻", "契约", "暖婚"},
	}
}

func loadCorpusSamples(root string) map[string]string {
	samples := make(map[string]string)
	for _, cat := range []string{"xuanhuan", "dushi", "guyan", "xuanyi", "kehuan", "tianchong"} {
		lines, err := storage.ReadCorpus(root, cat)
		if err != nil || len(lines) == 0 {
			continue
		}
		if len(lines) > 5 {
			lines = lines[:5]
		}
		samples[cat] = strings.Join(lines, "\n---\n")
	}
	return samples
}

// --- State management ---

// GetOrCreateState returns the novel state, creating it if necessary.
func (o *Orchestrator) GetOrCreateState(novelID, genre string) *state.NovelState {
	if ns, ok := o.states[novelID]; ok {
		return ns
	}
	// Try loading from disk
	ns, err := state.LoadNovelState(o.root, novelID)
	if err == nil {
		o.states[novelID] = ns
		return ns
	}
	ns = state.NewNovelState(novelID, genre)
	o.states[novelID] = ns
	return ns
}

// SaveState persists the novel state to disk.
func (o *Orchestrator) SaveState(ns *state.NovelState) error {
	return ns.Save(o.root)
}

// --- Stage execution ---

// StageInput carries the input data for a single stage (v2.x extended).
type StageInput struct {
	TrendData      string
	Topic          string
	ChapterOutline string
	PrevContext    string
	Content        string
	ChapterNo      int
	TotalChapters  int
	NovelID        string // for state lookup
}

// StageOutput is the result of executing a single stage.
type StageOutput struct {
	Stage      string `json:"stage"`
	TaskID     string `json:"task_id"`
	Content    string `json:"content"`
	PromptHash string `json:"prompt_hash"`
	DraftHash  string `json:"draft_hash"`
	SkillName  string `json:"skill_name"` // the invoked skill name (for output header)
}

// RunStage executes a single pipeline stage with NovelState injection and global rules.
func (o *Orchestrator) RunStage(ctx context.Context, taskID, skillName, stage string, input StageInput) (*StageOutput, error) {
	sk := o.mgr.Get(skillName)
	if sk == nil {
		return nil, fmt.Errorf("pipeline: skill %q not found", skillName)
	}
	if !sk.SupportsStage(stage) {
		return nil, fmt.Errorf("pipeline: skill %q does not support stage %q", skillName, stage)
	}

	// Network permission check
	if sk.NeedsNetworkPermission() && !o.GlobalRules.Network.Enabled {
		permReq := global.CheckPermission(o.GlobalRules, false, sk.FullName(),
			"此Skill需要联网获取实时信息（如网络热梗、热搜数据）")
		if permReq != nil {
			return nil, &NetworkPermissionRequired{Permission: *permReq}
		}
	}

	// Phase enforcement
	if err := o.checkPrerequisites(sk, input.NovelID); err != nil {
		return nil, err
	}

	// Get or create novel state
	var ns *state.NovelState
	if input.NovelID != "" {
		ns = o.GetOrCreateState(input.NovelID, sk.Genre)
	}

	// Render system prompt with state injected
	systemPrompt, err := o.renderPromptV2(sk, stage, input, ns)
	if err != nil {
		return nil, fmt.Errorf("pipeline: render prompt for %s/%s: %w", skillName, stage, err)
	}

	userPrompt := o.userPromptForStage(stage, input)

	// Call model
	content, err := o.router.Generate(ctx, sk, stage, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("pipeline: generate %s/%s: %w", skillName, stage, err)
	}

	// Store output
	filename := stage + ".txt"
	if err := storage.WriteOutput(o.root, taskID, filename, content); err != nil {
		return nil, fmt.Errorf("pipeline: write output: %w", err)
	}

	// Record copyright trace
	promptHash := storage.HashSHA256(systemPrompt + userPrompt)
	draftHash := storage.HashSHA256(content)
	tr := storage.TraceRecord{
		TaskID:     taskID,
		Stage:      stage,
		PromptHash: promptHash,
		DraftHash:  draftHash,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := storage.AppendTrace(o.root, tr); err != nil {
		fmt.Fprintf(os.Stderr, "[pipeline] WARNING: failed to record copyright trace for %s/%s: %v\n", taskID, stage, err)
	}

	// Update novel state after successful stage
	if ns != nil {
		switch stage {
		case "genre_init":
			ns.InitCompleted = true
			ns.InitSummary = last500(content)
		case "outline_generation":
			ns.AddOutlineVersion("AI generated", content)
		case "hooks_placement":
			// Hooks are parsed from content — simplified: store as hook entries
			ns.OutlineFinalized = true
		case "content_generation":
			ns.WritingStarted = true
			ns.ChaptersWritten++
			ns.LastChapterSummary = last300(content)
		}
		o.SaveState(ns)
	}

	return &StageOutput{
		Stage:      stage,
		TaskID:     taskID,
		Content:    content,
		PromptHash: promptHash,
		DraftHash:  draftHash,
		SkillName:  sk.FullName(),
	}, nil
}

// checkPrerequisites validates that required preceding stages are done.
func (o *Orchestrator) checkPrerequisites(sk *skill.Skill, novelID string) error {
	if len(sk.Prerequisites) == 0 || novelID == "" {
		return nil
	}
	ns, err := state.LoadNovelState(o.root, novelID)
	if err != nil {
		return fmt.Errorf("pipeline: cannot check prerequisites for novel %q: state not found", novelID)
	}
	for _, prereq := range sk.Prerequisites {
		switch prereq {
		case "outline_generation":
			if !ns.OutlineFinalized {
				return fmt.Errorf("pipeline: outline must be finalized before writing (use 'novel-agent run --skill %s_outline' first)", sk.Genre)
			}
		case "hooks_placement":
			if !ns.OutlineFinalized {
				return fmt.Errorf("pipeline: hooks must be placed before writing (use 'novel-agent run --skill %s_hooks' first)", sk.Genre)
			}
		}
	}
	return nil
}

// RunPipeline executes all stages of a skill in sequence.
func (o *Orchestrator) RunPipeline(ctx context.Context, taskID, skillName, trendData string) ([]StageOutput, error) {
	if storage.TaskExists(o.root, taskID) {
		fmt.Fprintf(os.Stderr, "[pipeline] task %q already exists, skipping\n", taskID)
		return o.readExistingOutputs(taskID)
	}

	sk := o.mgr.Get(skillName)
	if sk == nil {
		return nil, fmt.Errorf("pipeline: skill %q not found", skillName)
	}

	stages := sk.Stages
	var outputs []StageOutput
	var lastContent string

	for _, stage := range stages {
		input := StageInput{
			TrendData:  trendData,
			Topic:      lastContent,
			PrevContext: last300(lastContent),
			Content:    lastContent,
			NovelID:    taskID,
		}
		out, err := o.RunStage(ctx, taskID, skillName, stage, input)
		if err != nil {
			return outputs, fmt.Errorf("pipeline: stage %q failed: %w", stage, err)
		}
		outputs = append(outputs, *out)
		lastContent = out.Content
	}
	return outputs, nil
}

// --- Prompt rendering (v2.x with state injection) ---

type promptDataV2 struct {
	GlobalRules    string // formatted global rules (language, formatting)
	HotKeywords    string
	CorpusSamples  string
	TrendData      string
	Topic          string
	ChapterOutline string
	PrevContext    string
	Content        string
	ChapterNo      int
	TotalChapters  int
	NovelState     string // serialized novel state summary
	HookSummary    string // formatted hook ledger
}

func (o *Orchestrator) renderPromptV2(sk *skill.Skill, stage string, input StageInput, ns *state.NovelState) (string, error) {
	tmplStr := sk.PromptFor(stage)
	if tmplStr == "" {
		return "", fmt.Errorf("no prompt template for stage %q", stage)
	}

	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	data := promptDataV2{
		GlobalRules:   o.GlobalRules.AsPromptPrefix(),
		HotKeywords:    strings.Join(o.HotKeywords[sk.Genre], "、"),
		CorpusSamples:  o.CorpusSamples[sk.Genre],
		TrendData:      input.TrendData,
		Topic:          input.Topic,
		ChapterOutline: input.ChapterOutline,
		PrevContext:    input.PrevContext,
		Content:        input.Content,
		ChapterNo:      input.ChapterNo,
		TotalChapters:  input.TotalChapters,
	}

	if ns != nil {
		// Serialize the state into a compact prompt-ready string
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("【创作状态】类型:%s 赛道:%s\n", sk.Genre, ns.SubTrack))
		if ns.InitCompleted {
			sb.WriteString("✓ 初始化已完成\n")
		}
		if ns.OutlineFinalized {
			sb.WriteString(fmt.Sprintf("✓ 大纲已定稿(V%d)\n", ns.OutlineVersion))
		} else if ns.OutlineVersion > 0 {
			sb.WriteString(fmt.Sprintf("大纲迭代中(V%d)\n", ns.OutlineVersion))
		}
		if ns.WritingStarted {
			sb.WriteString(fmt.Sprintf("已写%d章\n", ns.ChaptersWritten))
		}
		// Hook summary
		hs := ns.HookSummary()
		if strings.Contains(hs, "共 0 条") {
			hs = "（暂无伏笔）"
		}
		data.HookSummary = hs
		data.NovelState = sb.String()
	} else {
		data.NovelState = "（新任务，无历史状态）"
		data.HookSummary = "（暂无伏笔）"
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	// Prepend global rules to every system prompt
	rendered := buf.String()
	prefix := o.GlobalRules.AsPromptPrefix()
	if !strings.Contains(rendered, prefix) {
		rendered = prefix + "\n" + rendered
	}
	return rendered, nil
}

func (o *Orchestrator) userPromptForStage(stage string, input StageInput) string {
	switch stage {
	case "genre_init":
		return fmt.Sprintf("用户创作意向：%s\n请确认赛道锁定、初始化创作档案，然后询问用户是否进入大纲阶段。", input.TrendData)
	case "outline_generation":
		return fmt.Sprintf("选题：%s\n请生成完整大纲（核心设定+人物谱系+主线剧情+爽点节点），分卷呈现。", input.Topic)
	case "hooks_placement":
		return "请基于已定稿大纲完成全维度伏笔/爽点/钩子埋置，每个伏笔标注ID和回收时机。"
	case "content_generation":
		prev := ""
		if input.PrevContext != "" {
			prev = "上文摘要：" + input.PrevContext + "\n"
		}
		return fmt.Sprintf("%s本章大纲：%s\n请生成第%d章正文（1500-2500字），植入微爽点+钩子+伏笔铺垫。", prev, input.ChapterOutline, input.ChapterNo)
	case "polish":
		return fmt.Sprintf("原文：\n%s\n\n请润色：去除AI套话、优化节奏、强化情绪张力，保留原意。", input.Content)
	case "optimize_shuangdian", "optimize_fubi", "optimize_jiezou", "optimize_renshe", "optimize_chongtu":
		return fmt.Sprintf("原文：\n%s\n\n请按该优化Skill的规则执行优化。", input.Content)
	default:
		return input.TrendData + input.Topic + input.ChapterOutline + input.Content
	}
}

func (o *Orchestrator) readExistingOutputs(taskID string) ([]StageOutput, error) {
	entries, err := os.ReadDir(o.root + "/outputs/" + taskID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: read existing outputs: %w", err)
	}
	var outputs []StageOutput
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		content, err := storage.ReadOutput(o.root, taskID, e.Name())
		if err != nil {
			continue
		}
		stage := strings.TrimSuffix(e.Name(), ".txt")
		outputs = append(outputs, StageOutput{
			Stage:   stage,
			TaskID:  taskID,
			Content: content,
		})
	}
	return outputs, nil
}

func last300(s string) string {
	runes := []rune(s)
	if len(runes) <= 300 {
		return s
	}
	return string(runes[len(runes)-300:])
}

func last500(s string) string {
	runes := []rune(s)
	if len(runes) <= 500 {
		return s
	}
	return string(runes[len(runes)-500:])
}

// NetworkPermissionRequired is returned when a skill needs network access
// and the user hasn't granted it yet. Callers should check for this type
// and prompt the user.
type NetworkPermissionRequired struct {
	Permission global.NetworkPermissionRequest
}

func (e *NetworkPermissionRequired) Error() string {
	return e.Permission.String()
}
