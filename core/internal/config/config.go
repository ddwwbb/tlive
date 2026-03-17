package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config is the top-level configuration for TermLive.
type Config struct {
	Daemon DaemonConfig `toml:"daemon"`
	Server ServerConfig `toml:"server"`
	Notify NotifyConfig `toml:"notify"`
}

// DaemonConfig holds settings for the background daemon process.
type DaemonConfig struct {
	Port      int    `toml:"port"`
	Token     string `toml:"token"`
	AutoStart bool   `toml:"auto_start"`
}

// ServerConfig holds settings for the HTTP/WebSocket server.
type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

// NotifyConfig holds settings for notification channels and idle detection.
type NotifyConfig struct {
	Channels     []string      `toml:"channels"`
	ShortTimeout int           `toml:"short_timeout"` // seconds, for awaiting-input (default 30)
	LongTimeout  int           `toml:"long_timeout"`  // seconds, for unknown idle (default 120)
	Options      NotifyOptions `toml:"options"`
	Patterns     PatternConfig `toml:"patterns"`
	WeChat       WeChatConfig  `toml:"wechat"`
	Feishu       FeishuConfig  `toml:"feishu"`
}

// NotifyOptions controls notification behavior.
type NotifyOptions struct {
	IncludeContext bool `toml:"include_context"`
	HistoryLimit   int  `toml:"history_limit"`
}

// PatternConfig holds custom patterns for output classification.
type PatternConfig struct {
	AwaitingInput []string `toml:"awaiting_input"`
	Processing    []string `toml:"processing"`
}

// WeChatConfig holds WeChat webhook settings.
type WeChatConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

// FeishuConfig holds Feishu webhook settings.
type FeishuConfig struct {
	WebhookURL string `toml:"webhook_url"`
	Secret     string `toml:"secret"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Daemon: DaemonConfig{Port: 8080},
		Server: ServerConfig{Port: 8080, Host: "0.0.0.0"},
		Notify: NotifyConfig{
			Channels:     []string{"web"},
			ShortTimeout: 30,
			LongTimeout:  120,
			Options: NotifyOptions{
				IncludeContext: true,
				HistoryLimit:   100,
			},
		},
	}
}

// LoadFromFile reads a TOML config from path, falling back to defaults.
func LoadFromFile(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
