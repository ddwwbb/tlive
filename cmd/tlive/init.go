package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/internal/generator"
)

var (
	initTool string
	initYes  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize TermLive in the current project",
	Long: `Generate skills, rules, hooks, and config files for AI tool integration.

Currently supports: claude-code (default).
Run with --yes to skip interactive prompts.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initTool, "tool", "claude-code", "AI tool to configure (claude-code)")
	initCmd.Flags().BoolVar(&initYes, "yes", false, "Skip interactive prompts, use defaults")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := generator.GeneratorConfig{
		DaemonPort: port,
	}

	var gen generator.Generator
	switch initTool {
	case "claude-code":
		gen = generator.NewClaudeCodeGenerator(dir, cfg)
	default:
		return fmt.Errorf("unsupported tool: %s (supported: claude-code)", initTool)
	}

	if !initYes {
		fmt.Fprintf(os.Stderr, "  Initializing TermLive for %s...\n\n", gen.Name())
	}

	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// Print generated files
	fmt.Fprintf(os.Stderr, "  Generated files:\n")
	for _, f := range gen.(*generator.ClaudeCodeGenerator).GeneratedFiles() {
		fmt.Fprintf(os.Stderr, "    %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "\n  Run 'tlive daemon start' to start the notification service.\n")

	return nil
}
