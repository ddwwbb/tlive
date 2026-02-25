package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server ServerConfig `toml:"server"`
	Notify NotifyConfig `toml:"notify"`
}

type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type NotifyConfig struct {
	ShortTimeout int           `toml:"short_timeout"` // seconds, for awaiting-input (default 30)
	LongTimeout  int           `toml:"long_timeout"`  // seconds, for unknown idle (default 120)
	Patterns     PatternConfig `toml:"patterns"`
	WeChat       WeChatConfig  `toml:"wechat"`
	Feishu       FeishuConfig  `toml:"feishu"`
}

type PatternConfig struct {
	AwaitingInput []string `toml:"awaiting_input"`
	Processing    []string `toml:"processing"`
}

type WeChatConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

type FeishuConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{Port: 8080, Host: "0.0.0.0"},
		Notify: NotifyConfig{
			ShortTimeout: 30,
			LongTimeout:  120,
		},
	}
}

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
