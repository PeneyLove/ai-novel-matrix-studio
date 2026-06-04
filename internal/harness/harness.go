// Package harness is the top-level runtime that ties together
// skill management, model routing, pipeline orchestration, and storage.
//
// Usage:
//
//	h, err := harness.New(".novelAgent")
//	defer h.Close()
//	outputs, err := h.RunPipeline(ctx, "task-001", "female_rebirth", "重生虐渣文持续霸榜")
package harness

import (
	"context"
	"fmt"
	"os"

	"github.com/penney-101/ai-novel-agent/internal/global"
	"github.com/penney-101/ai-novel-agent/internal/model"
	"github.com/penney-101/ai-novel-agent/internal/pipeline"
	"github.com/penney-101/ai-novel-agent/internal/skill"
	"github.com/penney-101/ai-novel-agent/internal/storage"
)

// Harness is the top-level application runtime.
type Harness struct {
	Root        string
	Skills      *skill.Manager
	Router      *model.Router
	Pipe        *pipeline.Orchestrator
	GlobalRules global.Rules
}

// New creates a Harness from the given .novelAgent root directory.
func New(root string, modelConfigs map[string]model.Config, fallbackModel string) (*Harness, error) {
	// Validate root exists
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("harness: .novelAgent directory not found at %q — run 'novel-agent init' first", root)
	}

	// Load config for global rules
	rules := global.DefaultRules()
	if cfg, err := storage.ReadConfig(root); err == nil {
		if gr, ok := cfg["global_rules"]; ok {
			if grMap, ok := gr.(map[string]any); ok {
				if lang, ok := grMap["language"].(string); ok {
					rules.Language = lang
				}
				if ruleList, ok := grMap["rules"]; ok {
					if rl, ok := ruleList.([]any); ok {
						rules.Rules = make([]string, 0, len(rl))
						for _, r := range rl {
							if s, ok := r.(string); ok {
								rules.Rules = append(rules.Rules, s)
							}
						}
					}
				}
				if netCfg, ok := grMap["network"]; ok {
					if nm, ok := netCfg.(map[string]any); ok {
						if enabled, ok := nm["enabled"].(bool); ok {
							rules.Network.Enabled = enabled
						}
						if ask, ok := nm["ask_permission"].(bool); ok {
							rules.Network.AskPermission = ask
						}
					}
				}
			}
		}
	}

	// Load skills
	mgr, err := skill.NewManager(root)
	if err != nil {
		return nil, fmt.Errorf("harness: load skills: %w", err)
	}
	if len(mgr.List()) == 0 {
		fmt.Fprintln(os.Stderr, "[harness] warning: no skills loaded")
	}

	// Create model router
	router, err := model.NewRouter(modelConfigs, fallbackModel)
	if err != nil {
		return nil, fmt.Errorf("harness: create router: %w", err)
	}

	// Create pipeline orchestrator with global rules injected
	pipe := pipeline.NewOrchestrator(root, router, mgr)
	pipe.GlobalRules = rules

	return &Harness{
		Root:        root,
		Skills:      mgr,
		Router:      router,
		Pipe:        pipe,
		GlobalRules: rules,
	}, nil
}

// RunPipeline executes the full creation pipeline for a skill.
func (h *Harness) RunPipeline(ctx context.Context, taskID, skillName, trendData string) ([]pipeline.StageOutput, error) {
	return h.Pipe.RunPipeline(ctx, taskID, skillName, trendData)
}

// RunStage executes a single pipeline stage.
func (h *Harness) RunStage(ctx context.Context, taskID, skillName, stage string, input pipeline.StageInput) (*pipeline.StageOutput, error) {
	return h.Pipe.RunStage(ctx, taskID, skillName, stage, input)
}

// ListSkills returns the names of all installed skills.
func (h *Harness) ListSkills() []string {
	return h.Skills.List()
}

// GetSkill returns a skill by name or nil.
func (h *Harness) GetSkill(name string) *skill.Skill {
	return h.Skills.Get(name)
}

// ReloadSkills re-reads skill definitions from disk.
func (h *Harness) ReloadSkills() error {
	return h.Skills.Reload()
}

// Close performs cleanup (currently a no-op).
func (h *Harness) Close() error {
	return nil
}
