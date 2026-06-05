package project

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// EvolutionStep records one change in a character's state over the course of a story.
type EvolutionStep struct {
	Chapter     int    `yaml:"chapter"`      // chapter number where this change occurs
	Type        string `yaml:"type"`         // 升级 / 转折 / 下线 / 获得能力 / 关系变化
	Description string `yaml:"description"`  // human-readable description
	Before      string `yaml:"before"`       // state before this step
	After       string `yaml:"after"`        // state after this step
	RecordedAt  string `yaml:"recorded_at"`  // ISO timestamp
	ConfirmedBy string `yaml:"confirmed_by"` // "auto" | "user:input"
}

// AppendEvolution adds a new step to the character's growth log.
func (ch *CharacterProfile) AppendEvolution(step EvolutionStep) []EvolutionStep {
	if step.RecordedAt == "" {
		step.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if step.ConfirmedBy == "" {
		step.ConfirmedBy = "auto"
	}
	ch.Evolution = append(ch.Evolution, step)
	return ch.Evolution
}

// LatestEvolution returns the most recent evolution step (for prompt injection).
func (ch *CharacterProfile) LatestEvolution() *EvolutionStep {
	if len(ch.Evolution) == 0 {
		return nil
	}
	return &ch.Evolution[len(ch.Evolution)-1]
}

// CurrentState summarizes the character's latest status for skill prompt injection.
// It combines the base personality with the most recent evolution step.
func (ch *CharacterProfile) CurrentState() string {
	s := fmt.Sprintf(
		"角色：%s（%s）\n性格：%s\n背景：%s\n动机：%s",
		ch.Name, ch.Role, ch.Personality, ch.Background, ch.Motivation,
	)
	if ch.Abilities != "" {
		s += "\n能力：" + ch.Abilities
	}
	if ch.Arc != "" {
		s += "\n人物弧光：" + ch.Arc
	}
	if len(ch.Evolution) > 0 {
		s += fmt.Sprintf("\n\n成长轨迹（%d 步）：", len(ch.Evolution))
		for _, e := range ch.Evolution {
			s += fmt.Sprintf(
				"\n  第%d章 [%s] %s → %s",
				e.Chapter, e.Type, e.Before, e.After,
			)
		}
	}
	return s
}

// LatestChapter returns the highest chapter number the character appears in.
func (ch *CharacterProfile) LatestChapter() int {
	max := 0
	for _, e := range ch.Evolution {
		if e.Chapter > max {
			max = e.Chapter
		}
	}
	return max
}

// EvolvePromptFromString stores a refined character skill prompt.
func (ch *CharacterProfile) EvolvePromptFromString(p string) {
	ch.EvolvePrompt = p
}

// Serialize returns a compact YAML representation of the character with
// evolution history suitable for reading by a skill system prompt.
func (ch *CharacterProfile) Serialize() string {
	data, _ := yaml.Marshal(ch)
	return string(data)
}
