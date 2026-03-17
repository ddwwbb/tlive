package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
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

// binaryCmd returns the tlive command string for use in generated files.
// Relies on PATH — tlive init ensures the install dir is in PATH.
func (g *ClaudeCodeGenerator) binaryCmd() string {
	return "tlive"
}

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
		filepath.Join(".claude", "rules", "termlive.md"),
		filepath.Join(".claude", "settings.local.json"),
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
description: Use when a task completes, needs user confirmation, encounters an error, or you want to report progress to the user
allowed-tools: Bash(tlive notify *)
---

# TermLive Notify

Send notifications to the user's TermLive dashboard. They'll get alerted on their phone or browser even when away from the terminal.

## Usage

` + "```" + `
tlive notify --type <type> --message "<message>" [--context "<details>"]
` + "```" + `

Types: done, confirm, error, progress

## Guidelines

Write natural, concise messages. The --type flag handles categorization, so don't prefix messages with "Completed:" or "Error:" etc.

- **done**: Summarize what was accomplished in plain language
- **error**: Describe what failed and why
- **confirm**: Explain what decision or input is needed
- **progress**: Brief status update on long-running work

## Examples

` + "```bash" + `
tlive notify --type done --message "Added JWT auth with refresh tokens" \
  --context "New files: auth.go, middleware.go. Tests passing."

tlive notify --type error --message "Build fails: missing crypto/ed25519 import" \
  --context "See internal/auth/keys.go:42"

tlive notify --type confirm --message "Database migration will drop the sessions table — proceed?" \
  --context "This affects 3 tables. Backup recommended."

tlive notify --type progress --message "Running test suite, 47/120 passed so far"
` + "```" + `

The command exits silently if the daemon is not running — it never blocks your workflow.
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
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "tlive notify --quiet --type confirm --message \"Claude Code needs your attention\" || true",
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
            "command": "tlive notify --quiet --type done --message \"Claude Code finished working\" || true",
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

func (g *ClaudeCodeGenerator) generateRules() error {
	dir := filepath.Join(g.projectDir, ".claude", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create rules dir: %w", err)
	}
	content := `# TermLive Notification Rules

- When you complete a significant task, invoke the termlive-notify skill
- When you need user confirmation, invoke the termlive-notify skill
- When you encounter a blocking error, invoke the termlive-notify skill
`
	return os.WriteFile(filepath.Join(dir, "termlive.md"), []byte(content), 0644)
}

func (g *ClaudeCodeGenerator) generateConfig() error {
	path := filepath.Join(g.projectDir, ".termlive.toml")
	content := fmt.Sprintf(`# TermLive configuration
# See: https://github.com/termlive/termlive

[daemon]
port = %d
token = "%s"
auto_start = false

[notify]
channels = ["web"]

# [notify.wechat]
# webhook_url = ""

# [notify.feishu]
# webhook_url = ""
# secret = ""

[notify.options]
include_context = true
history_limit = 100
`, g.cfg.DaemonPort, g.cfg.Token)
	return os.WriteFile(path, []byte(content), 0644)
}
