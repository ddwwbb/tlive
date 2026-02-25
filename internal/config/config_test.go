package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Notify.ShortTimeout != 30 {
		t.Errorf("expected default short timeout 30, got %d", cfg.Notify.ShortTimeout)
	}
	if cfg.Notify.LongTimeout != 120 {
		t.Errorf("expected default long timeout 120, got %d", cfg.Notify.LongTimeout)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := []byte(`
[server]
port = 3000
host = "127.0.0.1"

[notify]
short_timeout = 15
long_timeout = 300

[notify.wechat]
webhook_url = "https://example.com/wechat"

[notify.feishu]
webhook_url = "https://example.com/feishu"
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Notify.ShortTimeout != 15 {
		t.Errorf("expected short timeout 15, got %d", cfg.Notify.ShortTimeout)
	}
	if cfg.Notify.LongTimeout != 300 {
		t.Errorf("expected long timeout 300, got %d", cfg.Notify.LongTimeout)
	}
	if cfg.Notify.WeChat.WebhookURL != "https://example.com/wechat" {
		t.Errorf("unexpected wechat webhook url: %s", cfg.Notify.WeChat.WebhookURL)
	}
	if cfg.Notify.Feishu.WebhookURL != "https://example.com/feishu" {
		t.Errorf("unexpected feishu webhook url: %s", cfg.Notify.Feishu.WebhookURL)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/config.toml")
	if err != nil {
		t.Fatal("missing file should return defaults, not error")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadPatternsFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := []byte(`
[notify]
short_timeout = 10
long_timeout = 60

[notify.patterns]
awaiting_input = ["\\$\\s*$", ">>>\\s*$", "mysql>\\s*$"]
processing = ["^\\s*\\d+%", "ETA\\s+\\d"]
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notify.ShortTimeout != 10 {
		t.Errorf("expected short timeout 10, got %d", cfg.Notify.ShortTimeout)
	}
	if cfg.Notify.LongTimeout != 60 {
		t.Errorf("expected long timeout 60, got %d", cfg.Notify.LongTimeout)
	}
	if len(cfg.Notify.Patterns.AwaitingInput) != 3 {
		t.Fatalf("expected 3 awaiting_input patterns, got %d", len(cfg.Notify.Patterns.AwaitingInput))
	}
	if cfg.Notify.Patterns.AwaitingInput[0] != "\\$\\s*$" {
		t.Errorf("unexpected awaiting_input[0]: %s", cfg.Notify.Patterns.AwaitingInput[0])
	}
	if len(cfg.Notify.Patterns.Processing) != 2 {
		t.Fatalf("expected 2 processing patterns, got %d", len(cfg.Notify.Patterns.Processing))
	}
	if cfg.Notify.Patterns.Processing[1] != "ETA\\s+\\d" {
		t.Errorf("unexpected processing[1]: %s", cfg.Notify.Patterns.Processing[1])
	}
}
