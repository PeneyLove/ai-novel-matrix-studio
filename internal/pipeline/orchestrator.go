// Package pipeline implements the creation pipeline orchestration.
//
// The pipeline runs a skill's stages in sequence:
//
//	topic_generation → outline_generation → content_generation → polish
//
// Each stage:
//   - Resolves the model client via Router
//   - Renders the skill's prompt template with input data
//   - Calls the model and stores the output
//   - Records a copyright trace (SHA256 hashes)
//
// Idempotency: if outputs/<taskID>/ already exists, the task is not re-executed.
package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/penney-101/ai-novel-agent/internal/model"
	"github.com/penney-101/ai-novel-agent/internal/skill"
	"github.com/penney-101/ai-novel-agent/internal/storage"
)

// Orchestrator runs pipeline tasks against a skill and model router.
type Orchestrator struct {
	root   string            // .novelAgent root
	router *model.Router
	mgr    *skill.Manager

	// HotKeywords and CorpusSamples are injected into prompt templates.
	// They are loaded from .novelAgent/corpus/ on init.
	HotKeywords   map[string][]string // skill name → keywords
	CorpusSamples map[string]string   // skill name → concatenated samples
}

// NewOrchestrator creates an Orchestrator for the given .novelAgent root.
func NewOrchestrator(root string, router *model.Router, mgr *skill.Manager) *Orchestrator {
	return &Orchestrator{
		root:          root,
		router:        router,
		mgr:           mgr,
		HotKeywords:   defaultKeywords(),
		CorpusSamples: loadCorpusSamples(root),
	}
}

// defaultKeywords returns the built-in hot keyword sets from v0.x.
func defaultKeywords() map[string][]string {
	return map[string][]string{
		"female_rebirth": {"重生", "穿越", "虐渣", "马甲", "打脸", "逆袭", "复仇"},
		"male_power":     {"都市", "异能", "系统", "签到", "无敌", "升级", "金手指"},
		"suspense":       {"悬疑", "推理", "侦探", "反转", "密室", "诡计", "真相"},
		"romance":        {"甜宠", "恋爱", "暖文", "校园", "青梅竹马", "总裁", "契约"},
	}
}

func loadCorpusSamples(root string) map[string]string {
	samples := make(map[string]string)
	for _, cat := range []string{"female_rebirth", "male_power", "suspense", "romance"} {
		lines, err := storage.ReadCorpus(root, cat)
		if err != nil || len(lines) == 0 {
			continue
		}
		// Take up to 5 samples
		if len(lines) > 5 {
			lines = lines[:5]
		}
		samples[cat] = strings.Join(lines, "\n---\n")
	}
	return samples
}

// --- Pipeline execution ---

// StageInput carries the input data for a single stage.
type StageInput struct {
	TrendData      string // topic_generation
	Topic          string // outline_generation
	ChapterOutline string // content_generation
	PrevContext    string // content_generation (previous chapter summary)
	Content        string // polish (raw content)
}

// StageOutput is the result of executing a single stage.
type StageOutput struct {
	Stage      string `json:"stage"`
	TaskID     string `json:"task_id"`
	Content    string `json:"content"`
	PromptHash string `json:"prompt_hash"`
	DraftHash  string `json:"draft_hash"`
}

// RunStage executes a single pipeline stage.
func (o *Orchestrator) RunStage(ctx context.Context, taskID, skillName, stage string, input StageInput) (*StageOutput, error) {
	sk := o.mgr.Get(skillName)
	if sk == nil {
		return nil, fmt.Errorf("pipeline: skill %q not found", skillName)
	}
	if !sk.SupportsStage(stage) {
		return nil, fmt.Errorf("pipeline: skill %q does not support stage %q", skillName, stage)
	}

	// Build prompt from template
	systemPrompt, err := o.renderPrompt(sk, stage, input)
	if err != nil {
		return nil, fmt.Errorf("pipeline: render prompt for %s/%s: %w", skillName, stage, err)
	}

	// Build user prompt (simple pass-through of the primary input field)
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

	return &StageOutput{
		Stage:      stage,
		TaskID:     taskID,
		Content:    content,
		PromptHash: promptHash,
		DraftHash:  draftHash,
	}, nil
}

// RunPipeline executes all stages of a skill in sequence.
// Idempotent: if the taskID already has outputs/, it returns the existing data.
func (o *Orchestrator) RunPipeline(ctx context.Context, taskID, skillName, trendData string) ([]StageOutput, error) {
	// Idempotency guard (property P4)
	if storage.TaskExists(o.root, taskID) {
		fmt.Fprintf(os.Stderr, "[pipeline] task %q already exists, skipping\n", taskID)
		// Re-read existing outputs
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

// --- Prompt rendering ---

// promptData is the set of values available to prompt templates.
type promptData struct {
	HotKeywords    string
	CorpusSamples  string
	TrendData      string
	Topic          string
	ChapterOutline string
	PrevContext    string
	Content        string
}

func (o *Orchestrator) renderPrompt(sk *skill.Skill, stage string, input StageInput) (string, error) {
	tmplStr := sk.PromptFor(stage)
	if tmplStr == "" {
		return "", fmt.Errorf("no prompt template for stage %q", stage)
	}

	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	data := promptData{
		HotKeywords:    strings.Join(o.HotKeywords[sk.Name], "、"),
		CorpusSamples:  o.CorpusSamples[sk.Name],
		TrendData:      input.TrendData,
		Topic:          input.Topic,
		ChapterOutline: input.ChapterOutline,
		PrevContext:    input.PrevContext,
		Content:        input.Content,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (o *Orchestrator) userPromptForStage(stage string, input StageInput) string {
	switch stage {
	case "topic_generation":
		return fmt.Sprintf("热榜数据：%s\n请生成3个差异化选题，每个含书名、核心人设、爽点方向。", input.TrendData)
	case "outline_generation":
		return fmt.Sprintf("选题：%s\n请生成分卷大纲（5卷），每卷含3个核心爽点和结尾钩子。", input.Topic)
	case "content_generation":
		prev := input.PrevContext
		if prev != "" {
			prev = "上文摘要：" + prev + "\n"
		}
		return fmt.Sprintf("%s本章大纲：%s\n请按大纲生成本章正文（1500-2000字），保持人设一致，结尾留钩子。", prev, input.ChapterOutline)
	case "polish":
		return fmt.Sprintf("原文：\n%s\n\n请润色：去除AI套话、优化节奏、强化情绪张力，保留原意。", input.Content)
	default:
		return input.TrendData + input.Topic + input.ChapterOutline + input.Content
	}
}

func (o *Orchestrator) readExistingOutputs(taskID string) ([]StageOutput, error) {
	// Re-read from storage
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
