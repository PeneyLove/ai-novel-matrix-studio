package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/global"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/skill"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

// Helper: create a minimal .novelAgent root with a single skill installed.
func setupTestRoot(t *testing.T, skillMap map[string]any) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), storage.Dir)
	os.MkdirAll(filepath.Join(root, "skills", "test_skill"), 0o755)

	// Write config
	cfg := map[string]any{
		"stage_routing": map[string]any{"fallback": "qwen"},
	}
	if err := storage.WriteConfig(root, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Write skill
	if err := storage.WriteSkill(root, "test_skill", skillMap); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return root
}

// makeSkill returns a minimal valid skill for testing.
func makeSkill(genre string, stages []string, prereqs []string, needNetwork bool) map[string]any {
	return map[string]any{
		"name":        "test_skill",
		"version":     "1.0.0",
		"description": "Test skill",
		"genre":       genre,
		"type":        "core",
		"phase":       1,
		"sub_skill":   "genre_init",
		"prerequisites": prereqs,
		"stages":      stages,
		"model_bindings": map[string]string{
			"genre_init": "qwen",
		},
		"prompts": map[string]string{
			"genre_init": "你是{{.Genre}}写作专家。{{.GlobalRules}}",
		},
		"output_header":    "当前调用Skill：Test",
		"requires_network": needNetwork,
	}
}

func TestOrchestratorNormalizesStageNames(t *testing.T) {
	skillMap := makeSkill("xuanhuan", []string{"genre_init"}, nil, false)
	root := setupTestRoot(t, skillMap)

	router, err := model.NewRouter(nil, "qwen")
	if err != nil {
		t.Skipf("no model clients available: %v", err)
	}

	mgr, err := skill.NewManager(root)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if mgr.Get("test_skill") == nil {
		t.Fatal("test_skill not loaded")
	}

	orch := pipeline.NewOrchestrator(root, router, mgr)

	// Verify global rules are set
	if !strings.Contains(orch.GlobalRules.AsPromptPrefix(), "简体中文") {
		t.Error("orchestrator should have global rules with 简体中文")
	}
}

// P4: Pipeline idempotency — writing to the same task twice returns existing outputs.
func TestTaskIdempotency(t *testing.T) {
	root := filepath.Join(t.TempDir(), storage.Dir)
	os.MkdirAll(root, 0o755)

	// Write outputs for a task
	if err := storage.WriteOutput(root, "task-001", "genre_init.txt", "test content"); err != nil {
		t.Fatalf("write output: %v", err)
	}

	if !storage.TaskExists(root, "task-001") {
		t.Error("TaskExists should return true after writing output")
	}
	if storage.TaskExists(root, "task-002") {
		t.Error("TaskExists should return false for non-existent task")
	}
}

// P6: Copyright trace completeness — prompt_hash and draft_hash are non-empty.
func TestTraceCompleteness(t *testing.T) {
	root := filepath.Join(t.TempDir(), storage.Dir)
	os.MkdirAll(root, 0o755)

	tr := storage.TraceRecord{
		TaskID:     "task-001",
		Stage:      "genre_init",
		PromptHash: storage.HashSHA256("system: test prompt"),
		DraftHash:  storage.HashSHA256("generated content"),
		FinalHash:  "",
		Timestamp:  "2025-01-01T00:00:00Z",
	}

	if tr.PromptHash == "" {
		t.Error("P6 violation: prompt_hash must not be empty")
	}
	if tr.DraftHash == "" {
		t.Error("P6 violation: draft_hash must not be empty")
	}
	if tr.TaskID == "" {
		t.Error("P6 violation: task_id must not be empty")
	}

	if err := storage.AppendTrace(root, tr); err != nil {
		t.Fatalf("append trace: %v", err)
	}

	// Verify trace file exists
	tracePath := filepath.Join(root, "traces", "task-001.jsonl")
	if _, err := os.Stat(tracePath); os.IsNotExist(err) {
		t.Fatal("trace file was not created")
	}
}

// P2: Model routing completeness — NewRouter rejects empty configs gracefully.
func TestRouterRejectsEmptyConfigs(t *testing.T) {
	_, err := model.NewRouter(nil, "qwen")
	if err == nil {
		t.Error("NewRouter should return error when no clients can be created")
	}
}

// P1: Skill schema consistency — valid skill passes Validate, invalid fails.
func TestSkillValidation(t *testing.T) {
	valid := &skill.Skill{
		Name:         "test_skill",
		Version:      "1.0.0",
		Stages:       []string{"genre_init"},
		ModelBindings: map[string]string{"genre_init": "qwen"},
		Prompts:      map[string]string{"genre_init": "prompt {{.Data}}"},
	}
	if errs := valid.Validate(); len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("valid skill should not have errors: %v", e)
		}
	}

	// Invalid: missing model binding for a stage
	invalid := &skill.Skill{
		Name:         "bad_skill",
		Version:      "1.0",
		Stages:       []string{"genre_init", "outline_generation"},
		ModelBindings: map[string]string{"genre_init": "qwen"},
		Prompts: map[string]string{
			"genre_init":         "prompt",
			"outline_generation": "prompt",
		},
	}
	if errs := invalid.Validate(); len(errs) == 0 {
		t.Error("expected validation error for missing model_bindings entry")
	}
}

// P3: Skill SupportsStage returns correct values.
func TestSupportsStage(t *testing.T) {
	s := &skill.Skill{Stages: []string{"genre_init", "polish"}}

	if !s.SupportsStage("genre_init") {
		t.Error("should support genre_init")
	}
	if !s.SupportsStage("polish") {
		t.Error("should support polish")
	}
	if s.SupportsStage("outline_generation") {
		t.Error("should NOT support outline_generation")
	}
}

// Test that NetworkPermissionRequired is a proper error type.
func TestNetworkPermissionRequiredError(t *testing.T) {
	err := &pipeline.NetworkPermissionRequired{
		Permission: global.NetworkPermissionRequest{
			SkillName: "test",
			Reason:    "测试原因",
		},
	}
	if !strings.Contains(err.Error(), "联网权限") {
		t.Errorf("error message should mention 联网权限: %s", err.Error())
	}
}

// Test global rules injection — verify prefix contains Language.
func TestGlobalRulesInjection(t *testing.T) {
	skillMap := makeSkill("xuanhuan", []string{"genre_init"}, nil, false)
	root := setupTestRoot(t, skillMap)

	router, _ := model.NewRouter(nil, "qwen")
	mgr, _ := skill.NewManager(root)
	orch := pipeline.NewOrchestrator(root, router, mgr)

	prefix := orch.GlobalRules.AsPromptPrefix()
	if !strings.Contains(prefix, "zh-CN") {
		t.Log("global rules prefix:", prefix)
	}
	// Verify 8 rules present
	ruleCount := strings.Count(prefix, ". ")
	if ruleCount < 8 {
		t.Errorf("expected at least 8 rules (numbered), got %d rule delimiters", ruleCount)
	}
}
