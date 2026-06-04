// Package main is the entry point for the AI Novel Agent CLI.
//
// The binary provides a single-command interface to the Harness:
//
//	novel-agent init                     Initialize .novelAgent/ directory
//	novel-agent run --skill <name> ...   Execute a single stage
//	novel-agent pipeline --skill <name>  Run full creation pipeline
//	novel-agent skill list|install|...   Manage skills
//	novel-agent export --task-id <id>    Export generated content
//	novel-agent serve                    Start local HTTP API (for Flutter GUI)
//	novel-agent migrate --from v0.x      Migrate data from legacy system
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "novel-agent",
	Short: "AI Novel Agent — single-binary harness for AI-powered novel creation",
	Long: `A Go single-binary harness with pluggable YAML-defined skills.
All data is stored locally under .novelAgent/ — no external database required.`,
	Version: "1.0.0-alpha",
}

func main() {
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(pipelineCmd())
	rootCmd.AddCommand(skillCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(migrateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
