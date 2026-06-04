package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/penney-101/ai-novel-agent/internal/prompt"
	"github.com/penney-101/ai-novel-agent/internal/storage"
)

func promptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage skill prompt templates (edit, history, diff, optimize, rollback)",
		Long: `Manage and iterate on skill prompt templates.

Sub-commands:
  prompt edit    — Open a skill prompt in your editor for manual tweaking
  prompt history — List version snapshots for a skill prompt
  prompt diff    — Show changes between current and last snapshot
  prompt optimize — Auto-optimize a prompt based on user feedback
  prompt rollback — Restore a prompt to a previous version`,
	}

	cmd.AddCommand(promptEditCmd())
	cmd.AddCommand(promptHistoryCmd())
	cmd.AddCommand(promptDiffCmd())
	cmd.AddCommand(promptOptimizeCmd())
	cmd.AddCommand(promptRollbackCmd())

	return cmd
}

// --- prompt edit ---

func promptEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit --skill <name> --stage <stage>",
		Short: "Open a skill prompt in your editor",
		Long: `Snapshots the current prompt, then opens it in $EDITOR (or notepad on Windows).
After editing, the prompt is validated and the skill is hot-reloaded.`,
		RunE: runPromptEdit,
	}
}

func runPromptEdit(cmd *cobra.Command, args []string) error {
	skillName, _ := cmd.Flags().GetString("skill")
	stage, _ := cmd.Flags().GetString("stage")
	if skillName == "" || stage == "" {
		return fmt.Errorf("--skill and --stage are required")
	}

	root := agentRoot()
	skillPath, oldPrompt, err := prompt.EditSkillPrompt(root, skillName, stage)
	if err != nil {
		return err
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		if _, err := exec.LookPath("notepad"); err == nil {
			editor = "notepad"
		} else if _, err := exec.LookPath("vim"); err == nil {
			editor = "vim"
		} else if _, err := exec.LookPath("nano"); err == nil {
			editor = "nano"
		} else {
			editor = "notepad" // Windows fallback
		}
	}

	cmd.Printf("Opening %s in %s...\n", skillPath, editor)
	cmd.Printf("Old prompt length: %d chars\n", len(oldPrompt))
	cmd.Println("Save the file and close the editor when done.")

	editCmd := exec.Command(editor, skillPath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Validate after editing
	newPrompt, err := prompt.ApplyEditedPrompt(root, skillName, stage)
	if err != nil {
		return fmt.Errorf("after editing: %w", err)
	}
	cmd.Printf("✓ Prompt updated (%d chars). Use 'prompt diff --skill %s --stage %s' to see changes.\n",
		len(newPrompt), skillName, stage)

	return nil
}

// --- prompt history ---

func promptHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history --skill <name> --stage <stage>",
		Short: "List version snapshots for a prompt",
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName, _ := cmd.Flags().GetString("skill")
			stage, _ := cmd.Flags().GetString("stage")
			if skillName == "" || stage == "" {
				return fmt.Errorf("--skill and --stage are required")
			}
			root := agentRoot()
			files, err := prompt.ListHistory(root, skillName, stage)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				cmd.Println("No history found.")
				return nil
			}
			cmd.Printf("%d snapshot(s) for %s/%s:\n", len(files), skillName, stage)
			for _, f := range files {
				info, _ := os.Stat(f)
				cmd.Printf("  %s  (%d bytes)\n", filepath.Base(f), info.Size())
			}
			return nil
		},
	}
}

// --- prompt diff ---

func promptDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff --skill <name> --stage <stage>",
		Short: "Show prompt changes vs last snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName, _ := cmd.Flags().GetString("skill")
			stage, _ := cmd.Flags().GetString("stage")
			if skillName == "" || stage == "" {
				return fmt.Errorf("--skill and --stage are required")
			}
			root := agentRoot()
			// Read current from skill YAML
			raw, err := storage.ReadSkill(root, skillName)
			if err != nil {
				return err
			}
			var current string
			if prompts, ok := raw["prompts"]; ok {
				if pmap, ok := prompts.(map[string]any); ok {
					if p, ok := pmap[stage]; ok {
						current = fmt.Sprint(p)
					}
				}
			}
			diff := prompt.Diff(root, skillName, stage, current)
			cmd.Println(diff)
			return nil
		},
	}
}

// --- prompt optimize ---

func promptOptimizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimize --skill <name> --stage <stage> --feedback <text>",
		Short: "Auto-optimize a prompt based on feedback",
		Long: `Uses the fallback model to refine a skill's prompt template based on your feedback.
Example:
  novel-agent prompt optimize --skill xuanhuan --stage content_generation \
    --feedback "节奏太慢，需要每章前300字就出现一个小爽点"`,
		RunE: runPromptOptimize,
	}
	cmd.Flags().String("feedback", "", "Your feedback on what to improve (required)")
	return cmd
}

func runPromptOptimize(cmd *cobra.Command, args []string) error {
	skillName, _ := cmd.Flags().GetString("skill")
	stage, _ := cmd.Flags().GetString("stage")
	feedback, _ := cmd.Flags().GetString("feedback")

	if skillName == "" || stage == "" || feedback == "" {
		return fmt.Errorf("--skill, --stage, and --feedback are required")
	}

	h, err := newHarness()
	if err != nil {
		return err
	}
	defer h.Close()

	// Get current prompt
	sk := h.GetSkill(skillName)
	if sk == nil {
		return fmt.Errorf("skill %q not found", skillName)
	}
	currentPrompt := sk.PromptFor(stage)
	if currentPrompt == "" {
		return fmt.Errorf("stage %q has no prompt template in skill %q", stage, skillName)
	}

	optimizer := prompt.NewOptimizer(h.Router, h.Root)

	cmd.Println("Optimizing prompt... (this calls the AI model)")
	stageDesc := fmt.Sprintf("%s/%s — %s", skillName, stage, strings.SplitN(sk.Description, "\n", 2)[0])
	newPrompt, err := optimizer.OptimizePrompt(cmd.Context(), feedback, currentPrompt, stageDesc)
	if err != nil {
		return fmt.Errorf("optimize: %w", err)
	}

	if newPrompt == currentPrompt {
		cmd.Println("No changes needed — prompt already optimal.")
		return nil
	}

	// Write back
	if err := prompt.WritePromptToSkill(h.Root, skillName, stage, newPrompt); err != nil {
		return fmt.Errorf("write optimized prompt: %w", err)
	}

	// Hot-reload
	if err := h.ReloadSkills(); err != nil {
		cmd.Printf("Warning: hot-reload failed: %v\n", err)
	}

	cmd.Printf("✓ Prompt optimized (%d → %d chars)\n", len(currentPrompt), len(newPrompt))
	cmd.Println("  Use 'prompt diff --skill", skillName, "--stage", stage, "' to compare")
	return nil
}

// --- prompt rollback ---

func promptRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback --skill <name> --stage <stage> --to <snapshot-file>",
		Short: "Restore a prompt to a previous version",
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName, _ := cmd.Flags().GetString("skill")
			stage, _ := cmd.Flags().GetString("stage")
			to, _ := cmd.Flags().GetString("to")
			if skillName == "" || stage == "" || to == "" {
				return fmt.Errorf("--skill, --stage, and --to are required")
			}
			root := agentRoot()
			restoredContent, err := prompt.Rollback(root, skillName, stage, to)
			if err != nil {
				return err
			}
			if err := prompt.WritePromptToSkill(root, skillName, stage, restoredContent); err != nil {
				return fmt.Errorf("write rollback: %w", err)
			}
			cmd.Printf("✓ Rolled back %s/%s to %s\n", skillName, stage, to)
			return nil
		},
	}
}
