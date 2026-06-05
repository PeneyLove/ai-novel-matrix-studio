// Package skill defines the pluggable Skill system aligned with the comprehensive
// web-novel writing agent prompt architecture (Prompt.md).
//
// Key concepts:
//   - Genre families: 6 genres (xuanhuan/dushi/guyan/xuanyi/kehuan/tianchong)
//   - Each genre contains 9 sub-skills across 4 phases:
//     Phase 1 — genre_init:      类型定型 & 初始化
//     Phase 2 — outline:          大纲迭代优化 + hooks: 伏笔/爽点/钩子预埋
//     Phase 3 — writing:          正文定向续写（大纲绑定版）
//     Phase 4 — optimize_*:       爽点强化 / 伏笔回收 / 节奏优化 / 人设优化 / 冲突升级
//   - Strict phase ordering enforced by the pipeline
//   - Iteration state tracked via NovelState (hook ledger + version history)
//   - Every output declares the invoked skill name
package skill

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// SkillType categorizes skills as core (phased creation) or optimization (on-demand tuning).
type SkillType string

const (
	SkillTypeCore         SkillType = "core"         // phased creation skill (init/outline/hooks/writing)
	SkillTypeOptimization SkillType = "optimization" // on-demand tuning (爽点/伏笔/节奏/人设/冲突)
)

// Phase represents the mandatory creation phase ordering.
type Phase int

const (
	PhaseInit    Phase = 1 // 类型定型 & 初始化
	PhaseOutline Phase = 2 // 大纲迭代 + 钩子预埋
	PhaseWriting Phase = 3 // 正文定向续写
	PhaseOptimize Phase = 4 // 专项优化（可随时触发）
)

func (p Phase) String() string {
	switch p {
	case PhaseInit:
		return "init"
	case PhaseOutline:
		return "outline"
	case PhaseWriting:
		return "writing"
	case PhaseOptimize:
		return "optimize"
	default:
		return "unknown"
	}
}

// Skill is the parsed representation of a skill.yaml file.
// Extended from v1.0 to support Prompt.md's comprehensive architecture.
type Skill struct {
	Name        string    `yaml:"name"        json:"name"`
	Version     string    `yaml:"version"     json:"version"`
	Description string    `yaml:"description" json:"description"`

	// Genre binding — which novel genre this skill belongs to.
	// One of: xuanhuan, dushi, guyan, xuanyi, kehuan, tianchong
	Genre string `yaml:"genre" json:"genre"`

	// SkillType: core (phased) or optimization (on-demand)
	Type SkillType `yaml:"type" json:"type"`

	// Phase within the creation flow (1-4)
	Phase Phase `yaml:"phase" json:"phase"`

	// SubSkill is the sub-skill identifier within a genre.
	// E.g. "genre_init", "outline", "hooks", "writing", "optimize_shuangdian"
	SubSkill string `yaml:"sub_skill" json:"sub_skill"`

	// Prerequisites lists stage names that must be completed before this skill runs.
	// E.g. writing requires outline → hooks.
	Prerequisites []string `yaml:"prerequisites" json:"prerequisites"`

	// Stages lists creation stages this skill supports (legacy compatibility).
	// For new genre skills, each sub-skill maps to exactly one stage.
	Stages []string `yaml:"stages" json:"stages"`

	// ModelBindings maps stage → model provider name.
	ModelBindings map[string]string `yaml:"model_bindings" json:"model_bindings"`

	// Prompts maps stage → system prompt template (Go text/template syntax).
	// Templates now support {{.NovelState}} for injecting current iteration state.
	Prompts map[string]string `yaml:"prompts" json:"prompts"`

	// IterationPrompt is appended to the system prompt on every iteration after
	// the first. It instructs the model to preserve existing state, update only
	// what the user asked, and maintain logical consistency with prior versions.
	IterationPrompt string `yaml:"iteration_prompt" json:"iteration_prompt"`

	// OutputHeader is the required header format for every response.
	// E.g. "当前调用Skill：玄幻修仙-大纲迭代优化Skill"
	OutputHeader string `yaml:"output_header" json:"output_header"`

	// RequiresNetwork indicates this skill needs internet access (e.g. hot meme lookup).
	// When true and network is not enabled, the pipeline will request permission.
	RequiresNetwork bool `yaml:"requires_network" json:"requires_network"`

	// InputSchema maps stage → JSON Schema for input validation.
	InputSchema map[string]json.RawMessage `yaml:"input_schema" json:"input_schema"`
}

// NeedsNetworkPermission returns true if this skill requires network access.
func (s *Skill) NeedsNetworkPermission() bool {
	return s.RequiresNetwork
}

// FullName returns the genre-qualified skill name: "xuanhuan/outline".
func (s *Skill) FullName() string {
	if s.Genre == "" {
		return s.Name
	}
	return s.Genre + "/" + s.SubSkill
}

// ValidStageNames is the set of recognized creation stages.
var ValidStageNames = map[string]bool{
	"genre_init":               true,
	"outline_generation":       true,
	"hooks_placement":          true,
	"content_generation":       true,
	"polish":                   true,
	"optimize_shuangdian":      true,
	"optimize_fubi":            true,
	"optimize_jiezou":          true,
	"optimize_renshe":          true,
	"optimize_chongtu":         true,
}

// ValidGenres is the set of supported novel genres from Prompt.md.
var ValidGenres = map[string]bool{
	"xuanhuan":  true, // 玄幻修仙
	"dushi":     true, // 都市网文
	"guyan":     true, // 古言权谋
	"xuanyi":    true, // 悬疑灵异
	"kehuan":    true, // 科幻无限
	"tianchong": true, // 现言甜宠
}

// GenreLabels maps genre codes to Chinese labels.
var GenreLabels = map[string]string{
	"xuanhuan":  "玄幻修仙",
	"dushi":     "都市网文",
	"guyan":     "古言权谋",
	"xuanyi":    "悬疑灵异",
	"kehuan":    "科幻无限",
	"tianchong": "现言甜宠",
}

// SubSkillLabels maps sub-skill codes to Chinese labels.
var SubSkillLabels = map[string]string{
	"genre_init":          "类型定型&初始化",
	"outline":             "大纲迭代优化",
	"hooks":               "伏笔/爽点/钩子全维度埋置",
	"writing":             "正文定向续写（大纲绑定版）",
	"optimize_shuangdian": "爽点强化",
	"optimize_fubi":       "伏笔回收",
	"optimize_jiezou":     "节奏优化",
	"optimize_renshe":     "人设优化",
	"optimize_chongtu":    "冲突升级",
}

// ValidModelProviders is the set of supported model providers.
var ValidModelProviders = map[string]bool{
	"deepseek": true,
	"minimax":  true,
	"mimo":     true,
	"doubao":   true,
	"qwen":     true,
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
	if s.Genre != "" && !ValidGenres[s.Genre] {
		errs = append(errs, fmt.Errorf("skill: invalid genre %q; valid: %v", s.Genre, validGenreNames()))
	}
	if s.Type != "" && s.Type != SkillTypeCore && s.Type != SkillTypeOptimization {
		errs = append(errs, fmt.Errorf("skill: invalid type %q; must be 'core' or 'optimization'", s.Type))
	}
	if s.OutputHeader == "" && s.SubSkill != "" {
		errs = append(errs, fmt.Errorf("skill: output_header is required for genre sub-skills"))
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

func validGenreNames() []string {
	names := make([]string, 0, len(ValidGenres))
	for g := range ValidGenres {
		names = append(names, g)
	}
	return names
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

// IsCore returns true if this is a core phased creation skill.
func (s *Skill) IsCore() bool { return s.Type == SkillTypeCore }

// IsOptimization returns true if this is an on-demand optimization skill.
func (s *Skill) IsOptimization() bool { return s.Type == SkillTypeOptimization }
