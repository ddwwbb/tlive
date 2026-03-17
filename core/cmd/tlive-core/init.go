package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/termlive/termlive/core/internal/generator"
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

// installDir returns the standard install directory for the current OS.
//   - Unix/macOS: ~/.local/bin
//   - Windows:    ~/.termlive/bin
func installDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, ".termlive", "bin"), nil
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// installBinary copies the current executable to the standard install directory.
// Returns the installed path. Skips copy if already installed at the target.
func installBinary() (string, error) {
	srcPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("detect executable: %w", err)
	}
	srcPath, err = filepath.EvalSymlinks(srcPath)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	dir, err := installDir()
	if err != nil {
		return "", err
	}

	binName := "tlive"
	if runtime.GOOS == "windows" {
		binName = "tlive.exe"
	}
	dstPath := filepath.Join(dir, binName)

	// Skip if already at target location
	if filepath.Clean(srcPath) == filepath.Clean(dstPath) {
		return dstPath, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create install dir: %w", err)
	}

	// Copy binary
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("open source binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("create target binary: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copy binary: %w", err)
	}

	return dstPath, nil
}

// ensurePATH checks if the install directory is in PATH and adds it if not.
// On Unix, appends an export line to the shell profile. On Windows, updates
// the user PATH via registry. Never returns an error — only warns on stderr.
func ensurePATH(dir string) {
	// Check if already in PATH
	pathEnv := os.Getenv("PATH")
	for _, p := range filepath.SplitList(pathEnv) {
		if filepath.Clean(p) == filepath.Clean(dir) {
			return
		}
	}

	if runtime.GOOS == "windows" {
		ensurePATHWindows(dir)
	} else {
		ensurePATHUnix(dir)
	}
}

func ensurePATHUnix(dir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not detect home directory: %v\n", err)
		return
	}

	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, dir)

	// Detect shell and pick profile file
	shell := filepath.Base(os.Getenv("SHELL"))
	var profilePath string
	switch shell {
	case "zsh":
		profilePath = filepath.Join(home, ".zshrc")
	case "fish":
		// fish uses a different syntax
		exportLine = fmt.Sprintf("fish_add_path %s", dir)
		profilePath = filepath.Join(home, ".config", "fish", "config.fish")
	default: // bash and others
		profilePath = filepath.Join(home, ".bashrc")
	}

	// Check if line already exists in profile
	data, err := os.ReadFile(profilePath)
	if err == nil && strings.Contains(string(data), dir) {
		return
	}

	// Append to profile
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not update %s: %v\n", profilePath, err)
		return
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# Added by TermLive\n%s\n", exportLine); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not write to %s: %v\n", profilePath, err)
		return
	}

	fmt.Fprintf(os.Stderr, "  Added %s to PATH in %s\n", dir, profilePath)
	fmt.Fprintf(os.Stderr, "  Restart your terminal or run: source %s\n", profilePath)
}

func ensurePATHWindows(dir string) {
	// Read current user PATH from registry
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable("Path", "User")`).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not read user PATH: %v\n", err)
		return
	}

	currentPath := strings.TrimSpace(string(out))
	// Check if already present
	for _, p := range strings.Split(currentPath, ";") {
		if filepath.Clean(strings.TrimSpace(p)) == filepath.Clean(dir) {
			return
		}
	}

	// Prepend to user PATH
	newPath := dir + ";" + currentPath
	err = exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`[Environment]::SetEnvironmentVariable("Path", "%s", "User")`, newPath)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not update user PATH: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "  Added %s to user PATH (new terminals will pick it up)\n", dir)
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Install binary to standard location
	installedPath, err := installBinary()
	if err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	// Ensure install directory is in PATH
	ensurePATH(filepath.Dir(installedPath))

	cfg := generator.GeneratorConfig{
		DaemonPort: port,
		BinaryPath: installedPath,
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

	fmt.Fprintf(os.Stderr, "\n  Installed: %s\n", installedPath)
	fmt.Fprintf(os.Stderr, "  Run 'tlive daemon start' to start the notification service.\n")

	return nil
}
