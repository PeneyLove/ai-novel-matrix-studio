// Package audit provides a confirmation prompt for destructive operations.
//
// Rules are loaded from config.yaml:
//
//	audit:
//	  delete_requires_confirm: true      # file removal
//	  character_deactivate: "ask"       # ask | allow | deny
//	  output_overwrite: "ask"           # ask | allow | deny
//
// Mode semantics:
//   - "ask": print warning, wait for user 'y' / Enter (default=deny)
//   - "allow": proceed without prompt
//   - "deny": block immediately
package audit

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Policy controls destructive-operation gating.
type Policy struct {
	DeleteRequiresConfirm  bool   `yaml:"delete_requires_confirm"`
	CharacterDeactivate    string `yaml:"character_deactivate"`  // ask | allow | deny
	OutputOverwrite        string `yaml:"output_overwrite"`      // ask | allow | deny
	SkillInstall           string `yaml:"skill_install"`         // ask | allow | deny
}

// DefaultPolicy returns a safe-by-default policy.
func DefaultPolicy() Policy {
	return Policy{
		DeleteRequiresConfirm: true,
		CharacterDeactivate:   "ask",
		OutputOverwrite:       "ask",
		SkillInstall:          "allow",
	}
}

// Operation is the type of destructive action being requested.
type Operation string

const (
	OpDelete          Operation = "delete"
	OpCharDeactivate  Operation = "deactivate_character"
	OpOverwriteOutput Operation = "overwrite_output"
	OpRemoveSkill     Operation = "remove_skill"
)

// Check returns true if the operation is permitted, false if blocked.
// When mode is "ask", it prints a prompt and reads stdin (y = allow).
func (p Policy) Check(op Operation, detail string) (bool, error) {
	switch op {
	case OpDelete:
		if p.DeleteRequiresConfirm {
			return confirm(op, detail)
		}
		return true, nil
	case OpCharDeactivate:
		return p.resolveMode(p.CharacterDeactivate, op, detail)
	case OpOverwriteOutput:
		return p.resolveMode(p.OutputOverwrite, op, detail)
	case OpRemoveSkill:
		return p.resolveMode(p.SkillInstall, op, detail)
	default:
		return confirm(op, detail)
	}
}

func (p Policy) resolveMode(mode string, op Operation, detail string) (bool, error) {
	switch mode {
	case "allow":
		return true, nil
	case "deny":
		fmt.Fprintf(os.Stderr, "⚠ %s 已被策略阻止 (policy=deny)\n", op)
		return false, nil
	default: // "ask"
		return confirm(op, detail)
	}
}

// confirm prints a warning and reads a single 'y' from stdin.
func confirm(op Operation, detail string) (bool, error) {
	fmt.Println()
	fmt.Printf("⚠ 操作: %s\n", op)
	if detail != "" {
		fmt.Printf("   详情: %s\n", detail)
	}
	fmt.Print("   确认执行? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("audit: read input: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		return true, nil
	}
	fmt.Println("✗ 已取消")
	return false, nil
}

// Icon returns the appropriate visual prefix for the operation.
func (p Policy) Icon(op Operation) string {
	switch op {
	case OpDelete, OpCharDeactivate:
		return "⚠"
	case OpOverwriteOutput:
		return "📄"
	default:
		return "⚡"
	}
}
