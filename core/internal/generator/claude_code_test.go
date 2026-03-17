package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCodeGenerator_Generate(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{
		DaemonPort: 9090,
	})

	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	// Check SKILL.md was created with bare tlive command
	skillPath := filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md not created: %v", err)
	}
	skill := string(data)
	if !strings.Contains(skill, "name: termlive-notify") {
		t.Fatal("SKILL.md missing frontmatter")
	}
	if !strings.Contains(skill, "allowed-tools: Bash(tlive notify *)") {
		t.Fatal("SKILL.md should use bare 'tlive' in allowed-tools")
	}
	if !strings.Contains(skill, "tlive notify --type done") {
		t.Fatal("SKILL.md missing notify examples")
	}

	// Check rules file
	data, err = os.ReadFile(filepath.Join(dir, ".claude", "rules", "termlive.md"))
	if err != nil {
		t.Fatalf("rules/termlive.md not created: %v", err)
	}
	if !strings.Contains(string(data), "TermLive Notification Rules") {
		t.Fatal("rules file missing TermLive rules")
	}

	// Check settings.local.json uses bare tlive with guards
	data, err = os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("settings.local.json not created: %v", err)
	}
	hooks := string(data)
	if !strings.Contains(hooks, "Notification") {
		t.Fatal("settings.local.json missing Notification hook")
	}
	if !strings.Contains(hooks, "Stop") {
		t.Fatal("settings.local.json missing Stop hook")
	}
	if !strings.Contains(hooks, "tlive notify") {
		t.Fatal("settings.local.json should use bare 'tlive' command")
	}

	// Check .termlive.toml
	data, err = os.ReadFile(filepath.Join(dir, ".termlive.toml"))
	if err != nil {
		t.Fatalf(".termlive.toml not created: %v", err)
	}
	config := string(data)
	if !strings.Contains(config, "port = 9090") {
		t.Fatal(".termlive.toml missing daemon port")
	}
	if !strings.Contains(config, "token = ") {
		t.Fatal(".termlive.toml missing token")
	}
}

func TestClaudeCodeGenerator_RulesInCorrectLocation(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	// Rules should be in .claude/rules/termlive.md
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "rules", "termlive.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "termlive-notify skill") {
		t.Fatal("rules file missing skill reference")
	}
}

func TestClaudeCodeGenerator_AlwaysUsesBareTlive(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{
		DaemonPort: 8080,
		BinaryPath: "/some/path/tlive",
	})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	// Should always use bare "tlive" regardless of BinaryPath
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/some/path/tlive") {
		t.Fatal("SKILL.md should not contain absolute path — should use bare 'tlive'")
	}
	if !strings.Contains(string(data), "tlive notify") {
		t.Fatal("SKILL.md should use bare 'tlive notify'")
	}
}

func TestClaudeCodeGenerator_EmptyBinaryPathFallback(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{
		DaemonPort: 8080,
		BinaryPath: "",
	})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "tlive notify") {
		t.Fatal("SKILL.md should use bare 'tlive' when BinaryPath is empty")
	}
}

func TestClaudeCodeGenerator_Idempotent(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})

	// Run twice
	gen.Generate()
	gen.Generate()

	// Rules file should exist and not have issues from double-write
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "rules", "termlive.md"))
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), "TermLive Notification Rules")
	if count != 1 {
		t.Fatalf("expected 1 occurrence of TermLive rules, got %d", count)
	}
}

func TestClaudeCodeGenerator_SkillHasGuidelines(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	skill := string(data)

	// Should have guidelines section with free-form advice
	if !strings.Contains(skill, "## Guidelines") {
		t.Fatal("SKILL.md missing Guidelines section")
	}
	if !strings.Contains(skill, "natural, concise") {
		t.Fatal("SKILL.md should encourage natural messages")
	}

	// Should NOT have rigid template prefixes
	if strings.Contains(skill, `"Completed: <summary>"`) {
		t.Fatal("SKILL.md should not have rigid 'Completed:' template")
	}
	if strings.Contains(skill, `"Need approval: <details>"`) {
		t.Fatal("SKILL.md should not have rigid 'Need approval:' template")
	}

	// Should have realistic examples
	if !strings.Contains(skill, "## Examples") {
		t.Fatal("SKILL.md missing Examples section")
	}
}

func TestClaudeCodeGenerator_HooksHaveGuards(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	hooks := string(data)

	// Hooks should use --quiet flag
	if !strings.Contains(hooks, "--quiet") {
		t.Fatal("hooks should use --quiet flag")
	}

	// Hooks should have || true guard
	if !strings.Contains(hooks, "|| true") {
		t.Fatal("hooks should have || true guard to prevent failures")
	}

	// Should use bare tlive, not absolute path
	if strings.Contains(hooks, "/") && strings.Contains(hooks, "bin/tlive") {
		t.Fatal("hooks should use bare 'tlive', not absolute path")
	}
}
