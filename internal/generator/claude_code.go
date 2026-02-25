package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeCodeGenerator generates skills, rules, hooks, and config
// files for Claude Code integration.
type ClaudeCodeGenerator struct {
	projectDir string
	cfg        GeneratorConfig
}

// NewClaudeCodeGenerator creates a generator targeting the given project directory.
func NewClaudeCodeGenerator(projectDir string, cfg GeneratorConfig) *ClaudeCodeGenerator {
	if cfg.Token == "" {
		b := make([]byte, 16)
		rand.Read(b)
		cfg.Token = hex.EncodeToString(b)
	}
	return &ClaudeCodeGenerator{projectDir: projectDir, cfg: cfg}
}

func (g *ClaudeCodeGenerator) Name() string { return "Claude Code" }

// Generate creates all Claude Code integration files.
func (g *ClaudeCodeGenerator) Generate() error {
	steps := []func() error{
		g.generateSkill,
		g.generateHooks,
		g.generateRules,
		g.generateConfig,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// GeneratedFiles returns the list of relative paths that were (or will be) created.
func (g *ClaudeCodeGenerator) GeneratedFiles() []string {
	return []string{
		filepath.Join(".claude", "skills", "termlive-notify", "SKILL.md"),
		filepath.Join(".claude", "settings.local.json"),
		"CLAUDE.md",
		".termlive.toml",
	}
}

func (g *ClaudeCodeGenerator) generateSkill() error {
	dir := filepath.Join(g.projectDir, ".claude", "skills", "termlive-notify")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	content := `---
name: termlive-notify
description: Use when a task completes, needs user confirmation, or you want to report progress to the user
---

# TermLive Notify

## When to Use
- Task completed or milestone reached
- Need user confirmation or decision
- Encountered an error that requires user attention
- Long-running task progress update

## How to Notify
Run via Bash tool:

` + "```bash" + `
tlive notify --type done --message "Completed: <summary>"
tlive notify --type confirm --message "Need approval: <details>"
tlive notify --type error --message "Error: <details>"
tlive notify --type progress --message "Progress: <details>"
` + "```" + `
`
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}

func (g *ClaudeCodeGenerator) generateHooks() error {
	dir := filepath.Join(g.projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	content := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [
          {
            "type": "command",
            "command": "tlive notify --type confirm --message \"AI is waiting for your input\"",
            "timeout": 5000
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "tlive notify --type done --message \"Session ended\"",
            "timeout": 5000
          }
        ]
      }
    ]
  }
}
`
	return os.WriteFile(filepath.Join(dir, "settings.local.json"), []byte(content), 0644)
}

const termliveMark = "## TermLive Notification Rules"

func (g *ClaudeCodeGenerator) generateRules() error {
	path := filepath.Join(g.projectDir, "CLAUDE.md")
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	// Idempotent: skip if already present
	if strings.Contains(existing, termliveMark) {
		return nil
	}

	rules := "\n" + termliveMark + `
- When you complete a significant task, invoke the termlive-notify skill
- When you need user confirmation, invoke the termlive-notify skill
- When you encounter a blocking error, invoke the termlive-notify skill
`
	content := existing + rules
	return os.WriteFile(path, []byte(content), 0644)
}

func (g *ClaudeCodeGenerator) generateConfig() error {
	path := filepath.Join(g.projectDir, ".termlive.toml")
	content := fmt.Sprintf(`# TermLive configuration
# See: https://github.com/termlive/termlive

[daemon]
port = %d
auto_start = false

[notify]
channels = ["web"]
# wechat_webhook = ""
# feishu_webhook = ""

[notify.options]
include_context = true
history_limit = 100
`, g.cfg.DaemonPort)
	return os.WriteFile(path, []byte(content), 0644)
}
