// Package main is the entry point for the AI Novel Agent CLI.
//
// Two modes:
//
//	novel-agent             → Bubble Tea TUI (no arguments)
//	novel-agent init|...    → subcommand (cobra)
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "novel-agent",
	Short: "AI Novel Agent — interactive CLI for Chinese web novel creation",
	Long: `A Go single-binary harness with pluggable YAML-defined skills.
Run without arguments to enter the interactive writing terminal.
All data is stored locally under .novelAgent/ — no external database required.`,
	Version: "2.0.0-alpha",
	RunE:    runTUI,
}

func main() {
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(pipelineCmd())
	rootCmd.AddCommand(skillCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(promptCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	h, err := newHarness()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 无法启动: %v\n", err)
		fmt.Fprintln(os.Stderr, "  请先运行 novel-agent init 初始化 .novelAgent/ 目录")
		os.Exit(1)
	}
	defer h.Close()

	root := agentRoot()
	m := tui.New(h, root)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI 错误: %v\n", err)
		return err
	}
	return nil
}
