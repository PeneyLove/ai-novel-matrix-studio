package prompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

// Optimizer uses the fallback model to refine a skill's prompt template based on
// user feedback from a previous generation round.
//
// This implements the "对话迭代模式" — after each creation stage, if the user
// provides feedback ("写得太水了, 爽点不够"), the optimizer:
//   1. Snapshots the current prompt to history
//   2. Calls the fallback model with a meta-prompt asking it to improve the template
//   3. Writes the improved prompt back to the skill YAML
type Optimizer struct {
	router *model.Router
	root   string
}

// NewOptimizer creates a prompt auto-optimizer.
func NewOptimizer(router *model.Router, root string) *Optimizer {
	return &Optimizer{router: router, root: root}
}

// OptimizePrompt improves a prompt template based on user feedback and returns
// the new template. The caller is responsible for writing it back to the skill YAML.
//
// Parameters:
//   - feedback: user criticism (e.g. "节奏太慢, 开头缺钩子")
//   - currentPrompt: the prompt template that produced the unsatisfactory result
//   - stageDescription: what this prompt is supposed to do (for context)
func (o *Optimizer) OptimizePrompt(
	ctx context.Context,
	feedback string,
	currentPrompt string,
	stageDescription string,
) (string, error) {
	// Snapshot before mutation
	if _, err := Snapshot(o.root, "auto-optimize", stageDescription, currentPrompt); err != nil {
		fmt.Fprintf(os.Stderr, "[prompt] snapshot warning: %v\n", err)
	}

	// Get the fallback client (qwen)
	client, err := o.router.GetClient("qwen")
	if err != nil {
		// Try deepseek
		client, err = o.router.GetClient("deepseek")
		if err != nil {
			return "", fmt.Errorf("prompt: no available model for optimization: %w", err)
		}
	}

	metaPrompt := `你是提示词优化专家。你的任务是根据用户对生成内容的反馈，优化 system prompt 模板。

【重要规则】
1. 保留原 prompt 中的核心定位、写作规则、输出格式声明 — 这些是骨骼，不能删
2. 只优化用户反馈指出的具体问题（如节奏、爽点密度、伏笔埋设）
3. 在 prompt 中增加更具体的写作指令来解决反馈中提到的问题
4. 保持原有的 {{.Variable}} 模板变量不变
5. 不要在优化后的 prompt 中添加任何解释性开头或结尾，直接输出优化后的完整 prompt

用户反馈：{{FEEDBACK}}

当前阶段说明：{{STAGE_DESC}}

原始 Prompt 模板：
---ORIGINAL---
{{ORIGINAL}}
---END---

请输出优化后的完整 Prompt 模板（纯模板文本，无 markdown 代码块包裹）：`

	metaPrompt = strings.Replace(metaPrompt, "{{FEEDBACK}}", feedback, 1)
	metaPrompt = strings.Replace(metaPrompt, "{{STAGE_DESC}}", stageDescription, 1)
	metaPrompt = strings.Replace(metaPrompt, "{{ORIGINAL}}", currentPrompt, 1)

	optimized, err := client.Generate(ctx, "你是专业的AI提示词优化专家，擅长根据写作反馈精调创作提示词。", metaPrompt)
	if err != nil {
		return "", fmt.Errorf("prompt: optimization call failed: %w", err)
	}

	// Clean up common wrapping artifacts
	optimized = strings.TrimSpace(optimized)
	optimized = strings.TrimPrefix(optimized, "```yaml")
	optimized = strings.TrimPrefix(optimized, "```")
	optimized = strings.TrimSuffix(optimized, "```")
	optimized = strings.TrimSpace(optimized)

	if optimized == "" || optimized == currentPrompt {
		return currentPrompt, nil
	}

	return optimized, nil
}

// EditSkillPrompt opens the skill YAML file for manual editing.
// It snapshots the current state first, then returns the path for the user
// to open in their preferred editor.
func EditSkillPrompt(root, skillName, stage string) (string, string, error) {
	// Read current skill
	raw, err := storage.ReadSkill(root, skillName)
	if err != nil {
		return "", "", fmt.Errorf("prompt: read skill %q: %w", skillName, err)
	}

	// Get the current prompt for the given stage
	var currentPrompt string
	if prompts, ok := raw["prompts"]; ok {
		if pmap, ok := prompts.(map[string]any); ok {
			if p, ok := pmap[stage]; ok {
				currentPrompt = fmt.Sprint(p)
			}
		}
	}

	// Snapshot
	if currentPrompt != "" {
		Snapshot(root, skillName, stage, currentPrompt)
	}

	// Return the YAML path for editing
	skillPath := filepath.Join(root, "skills", skillName, "skill.yaml")
	return skillPath, currentPrompt, nil
}

// ApplyEditedPrompt reads back the skill YAML after manual editing and validates
// the prompt for the given stage is still present.
func ApplyEditedPrompt(root, skillName, stage string) (string, error) {
	raw, err := storage.ReadSkill(root, skillName)
	if err != nil {
		return "", fmt.Errorf("prompt: re-read skill: %w", err)
	}
	if prompts, ok := raw["prompts"]; ok {
		if pmap, ok := prompts.(map[string]any); ok {
			if p, ok := pmap[stage]; ok {
				newPrompt := fmt.Sprint(p)
				if newPrompt == "" {
					return "", fmt.Errorf("prompt: stage %q prompt is empty after editing — restore from history", stage)
				}
				return newPrompt, nil
			}
		}
	}
	return "", fmt.Errorf("prompt: stage %q not found after editing", stage)
}

// --- Helpers for saving back to skill YAML ---

// WritePromptToSkill reads the skill YAML, updates the prompt for the given stage,
// snapshots the old version, and writes back.
func WritePromptToSkill(root, skillName, stage, newPrompt string) error {
	raw, err := storage.ReadSkill(root, skillName)
	if err != nil {
		return fmt.Errorf("prompt: read skill: %w", err)
	}

	// Get old prompt for snapshot
	var oldPrompt string
	if prompts, ok := raw["prompts"]; ok {
		if pmap, ok := prompts.(map[string]any); ok {
			if p, ok := pmap[stage]; ok {
				oldPrompt = fmt.Sprint(p)
			}
		}
	}

	// Snapshot old version
	if oldPrompt != "" && oldPrompt != newPrompt {
		Snapshot(root, skillName, stage, oldPrompt)
	}

	// Read the actual file for raw YAML manipulation
	skillPath := filepath.Join(root, "skills", skillName, "skill.yaml")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("prompt: read skill file: %w", err)
	}

	// Parse as generic YAML
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("prompt: parse skill YAML: %w", err)
	}

	// Update the prompt
	if prompts, ok := doc["prompts"]; ok {
		if pmap, ok := prompts.(map[string]any); ok {
			pmap[stage] = newPrompt
		}
	}

	// Write back
	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("prompt: marshal updated skill: %w", err)
	}

	// Ensure the file header comment is preserved by pre-pending it
	header := []byte(fmt.Sprintf("# Auto-optimized at %s\n", time.Now().UTC().Format(time.RFC3339)))
	out = append(header, out...)

	if err := os.WriteFile(skillPath, out, 0o644); err != nil {
		return fmt.Errorf("prompt: write updated skill: %w", err)
	}

	return nil
}


