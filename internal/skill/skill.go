// Package skill defines the pluggable Skill system.
//
// Each skill is a YAML file under .novelAgent/skills/<name>/skill.yaml
// that declares its supported stages, model bindings, prompt templates,
// and input/output JSON schemas.
package skill

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Skill is the parsed representation of a skill.yaml file.
type Skill struct {
	Name        string `yaml:"name"        json:"name"`
	Version     string `yaml:"version"     json:"version"`
	Description string `yaml:"description" json:"description"`

	// Stages lists which creation stages this skill supports
	Stages []string `yaml:"stages" json:"stages"`

	// ModelBindings maps stage → model provider name
	ModelBindings map[string]string `yaml:"model_bindings" json:"model_bindings"`

	// Prompts maps stage → system prompt template (Go text/template syntax)
	Prompts map[string]string `yaml:"prompts" json:"prompts"`

	// InputSchema maps stage → JSON Schema for input validation
	InputSchema map[string]json.RawMessage `yaml:"input_schema" json:"input_schema"`
}

// ValidStageNames is the set of recognized creation stages.
var ValidStageNames = map[string]bool{
	"topic_generation":   true,
	"outline_generation": true,
	"content_generation": true,
	"polish":             true,
}

// ValidModelProviders is the set of supported model providers.
var ValidModelProviders = map[string]bool{
	"minimax":  true,
	"doubao":   true,
	"qwen":     true,
	"deepseek": true,
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// Validate checks the skill for correctness and returns all errors found.
func (s *Skill) Validate() []error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, fmt.Errorf("skill: name is required"))
	} else if !namePattern.MatchString(s.Name) {
		errs = append(errs, fmt.Errorf("skill: name %q must match %s", s.Name, namePattern))
	}
	if s.Version == "" {
		errs = append(errs, fmt.Errorf("skill: version is required"))
	}
	if len(s.Stages) == 0 {
		errs = append(errs, fmt.Errorf("skill: stages list is empty"))
	}
	for i, st := range s.Stages {
		if !ValidStageNames[st] {
			errs = append(errs, fmt.Errorf("skill: invalid stage %q at stages[%d]", st, i))
		}
	}
	for stage, model := range s.ModelBindings {
		if !ValidStageNames[stage] {
			errs = append(errs, fmt.Errorf("skill: model_bindings references unknown stage %q", stage))
		}
		if !ValidModelProviders[model] {
			errs = append(errs, fmt.Errorf("skill: model_bindings[%s] has invalid provider %q", stage, model))
		}
	}
	// Every stage must have a model binding
	for _, st := range s.Stages {
		if _, ok := s.ModelBindings[st]; !ok {
			errs = append(errs, fmt.Errorf("skill: stage %q has no model_bindings entry", st))
		}
	}
	// Every stage must have a prompt template
	for _, st := range s.Stages {
		if p := s.Prompts[st]; p == "" {
			errs = append(errs, fmt.Errorf("skill: stage %q has empty prompt template", st))
		}
	}

	return errs
}

// SupportsStage returns true if this skill can handle the given stage.
func (s *Skill) SupportsStage(stage string) bool {
	for _, st := range s.Stages {
		if st == stage {
			return true
		}
	}
	return false
}

// ModelFor returns the model provider bound to the given stage, or empty string.
func (s *Skill) ModelFor(stage string) string {
	return s.ModelBindings[stage]
}

// PromptFor returns the system prompt template for the given stage.
func (s *Skill) PromptFor(stage string) string {
	return s.Prompts[stage]
}
