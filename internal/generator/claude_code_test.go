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

	// Check SKILL.md was created
	skillPath := filepath.Join(dir, ".claude", "skills", "termlive-notify", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md not created: %v", err)
	}
	if !strings.Contains(string(data), "name: termlive-notify") {
		t.Fatal("SKILL.md missing frontmatter")
	}
	if !strings.Contains(string(data), "tlive notify") {
		t.Fatal("SKILL.md missing tlive notify command")
	}

	// Check settings.local.json was created
	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.local.json not created: %v", err)
	}
	if !strings.Contains(string(data), "PostToolUse") {
		t.Fatal("settings.local.json missing hooks")
	}

	// Check .termlive.toml was created
	configPath := filepath.Join(dir, ".termlive.toml")
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf(".termlive.toml not created: %v", err)
	}
	if !strings.Contains(string(data), "port = 9090") {
		t.Fatal(".termlive.toml missing daemon port")
	}
}

func TestClaudeCodeGenerator_AppendsClaudeMD(t *testing.T) {
	dir := t.TempDir()

	// Pre-existing CLAUDE.md
	existing := "# My Project\n\nSome existing rules.\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})
	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "# My Project") {
		t.Fatal("existing CLAUDE.md content was overwritten")
	}
	if !strings.Contains(content, "TermLive Notification Rules") {
		t.Fatal("CLAUDE.md missing TermLive rules")
	}
}

func TestClaudeCodeGenerator_Idempotent(t *testing.T) {
	dir := t.TempDir()
	gen := NewClaudeCodeGenerator(dir, GeneratorConfig{DaemonPort: 8080})

	// Run twice
	gen.Generate()
	gen.Generate()

	// CLAUDE.md should not have duplicate rules
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	count := strings.Count(string(data), "TermLive Notification Rules")
	if count != 1 {
		t.Fatalf("expected 1 occurrence of TermLive rules, got %d", count)
	}
}
