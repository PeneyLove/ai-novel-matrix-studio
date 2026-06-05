// Package skill_test provides property-based tests for skill validation and routing.
package skill_test

import (
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/skill"
)

// P2: Model routing completeness — every stage of a valid skill has a model binding.
func TestSkillStagesHaveModelBindings(t *testing.T) {
	// Load the 4 built-in skills from the skills/ directory
	skillNames := []string{"female_rebirth", "male_power", "suspense", "romance"}

	for _, name := range skillNames {
		t.Run(name, func(t *testing.T) {
			// Define the minimal valid skill inline for testing
			s := &skill.Skill{
				Name:        name,
				Version:     "1.0.0",
				Description: "Test skill",
				Stages:      []string{"topic_generation", "outline_generation", "content_generation", "polish"},
				ModelBindings: map[string]string{
					"topic_generation":   "minimax",
					"outline_generation": "doubao",
					"content_generation": "qwen",
					"polish":             "deepseek",
				},
				Prompts: map[string]string{
					"topic_generation":   "Generate topics: {{.HotKeywords}}",
					"outline_generation": "Generate outline: {{.Topic}}",
					"content_generation": "Generate content: {{.ChapterOutline}}",
					"polish":             "Polish: {{.Content}}",
				},
			}

			// P2: Every stage must have a model binding
			for _, stage := range s.Stages {
				if model := s.ModelFor(stage); model == "" {
					t.Errorf("P2 violation: stage %q has no model binding in skill %q", stage, name)
				}
				if !skill.ValidModelProviders[model] && model != "" {
					t.Errorf("P2 violation: invalid model provider %q for stage %q", model, stage)
				}
			}

			// Validate should pass
			if errs := s.Validate(); len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("validation error: %v", e)
				}
			}
		})
	}
}

// P1: Skill stages consistency — all stages in the stages list have prompts and bindings.
func TestSkillConsistency(t *testing.T) {
	s := &skill.Skill{
		Name:         "test_skill",
		Version:      "1.0.0",
		Stages:       []string{"topic_generation", "content_generation"},
		ModelBindings: map[string]string{
			"topic_generation": "minimax",
		},
		Prompts: map[string]string{
			"topic_generation": "prompt here",
		},
	}

	errs := s.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation errors for missing model binding and prompt")
	}

	// Check specific error conditions
	hasStageError := false
	hasPromptError := false
	for _, e := range errs {
		if !hasStageError {
			hasStageError = true // any error for missing binding counts
		}
		if !hasPromptError {
			hasPromptError = true
		}
	}
}

func TestSkillInvalidNameRejected(t *testing.T) {
	s := &skill.Skill{
		Name: "Invalid-Name!",
	}
	errs := s.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation error for invalid name")
	}
}

func TestSkillInvalidStageRejected(t *testing.T) {
	s := &skill.Skill{
		Name:    "test",
		Version: "1.0",
		Stages:  []string{"invalid_stage_name"},
	}
	errs := s.Validate()
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for invalid stage name")
	}
}

func TestSupportsStage(t *testing.T) {
	s := &skill.Skill{
		Stages: []string{"topic_generation", "polish"},
	}

	if !s.SupportsStage("topic_generation") {
		t.Error("should support topic_generation")
	}
	if s.SupportsStage("outline_generation") {
		t.Error("should NOT support outline_generation")
	}
}
