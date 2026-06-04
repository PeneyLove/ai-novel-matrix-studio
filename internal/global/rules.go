// Package global provides system-wide rules and network permission management.
//
// Global rules (language, formatting constraints) are injected into every
// pipeline prompt automatically by the orchestrator.
//
// Network permission: skills that require internet access (e.g. for hot meme
// lookup) must declare `requires_network: true` in their YAML. When triggered,
// the system requests user consent before enabling network access.
package global

import (
	"fmt"
	"strings"
)

// Rules holds the global system rules loaded from config.
type Rules struct {
	Language string   `yaml:"language" json:"language"` // primary output language, e.g. "zh-CN"
	Rules    []string `yaml:"rules"    json:"rules"`    // concrete rule statements
	Network  NetworkPolicy `yaml:"network" json:"network"`
}

// NetworkPolicy controls internet access for skills.
type NetworkPolicy struct {
	Enabled        bool     `yaml:"enabled"          json:"enabled"`
	AllowedDomains []string `yaml:"allowed_domains"  json:"allowed_domains"`
	AskPermission  bool     `yaml:"ask_permission"   json:"ask_permission"` // prompt user before enabling
}

// DefaultRules returns the built-in global rules (Simplified Chinese first).
func DefaultRules() Rules {
	return Rules{
		Language: "zh-CN",
		Rules: []string{
			"全程使用简体中文输出，包括所有说明、描述、对话、叙述",
			"专有名词、技术术语可保留原文（如 API、SDK、GDP、CEO 等）",
			"人名、地名、品牌名等专有名称可保留英文或拼音原文",
			"网络热梗、流行语、meme 可以使用，但涉及实时信息（如当前热搜、最新事件）时需申请联网权限",
			"代码块、命令行示例保持英文原样",
			"禁止输出繁体中文（繁體中文），除非用户明确要求",
			"数字使用阿拉伯数字（1, 2, 3），非特殊排版需要不使用中文数字",
			"标点符号使用全角中文标点（。，！？），英文内容内部使用半角标点",
		},
		Network: NetworkPolicy{
			Enabled:        false,
			AllowedDomains: []string{},
			AskPermission:  true,
		},
	}
}

// AsPromptPrefix renders the rules as a system prompt prefix.
func (r *Rules) AsPromptPrefix() string {
	var sb strings.Builder
	sb.WriteString("【全局规则 — 必须遵守】\n")
	for i, rule := range r.Rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rule))
	}
	return sb.String()
}

// NetworkPermissionRequest is returned when a skill requests network access
// and the user hasn't granted it yet.
type NetworkPermissionRequest struct {
	SkillName string `json:"skill_name"`
	Reason    string `json:"reason"` // why network is needed
}

func (npr *NetworkPermissionRequest) String() string {
	return fmt.Sprintf(
		"技能「%s」需要联网权限（%s）。\n是否允许？输入 y 同意 / n 拒绝：",
		npr.SkillName, npr.Reason,
	)
}

// CheckPermission determines whether a network-requiring skill can proceed.
// Returns nil if allowed, or a NetworkPermissionRequest if user consent is needed.
func CheckPermission(rules Rules, enabled bool, skillName, reason string) *NetworkPermissionRequest {
	if !rules.Network.Enabled && rules.Network.AskPermission {
		return &NetworkPermissionRequest{SkillName: skillName, Reason: reason}
	}
	return nil
}
