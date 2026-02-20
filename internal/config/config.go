package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	BotToken string `yaml:"bot_token,omitempty"`
	ChatID   string `yaml:"chat_id,omitempty"`
}

type WebhookConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	URL     string `yaml:"url,omitempty"`
}

type NotificationsConfig struct {
	Telegram TelegramConfig `yaml:"telegram,omitempty"`
	Webhook  WebhookConfig  `yaml:"webhook,omitempty"`
}

type Config struct {
	// Legacy field for backward compatibility
	WebhookURL string `yaml:"webhook_url,omitempty"`

	// New notifications structure
	Notifications NotificationsConfig `yaml:"notifications,omitempty"`

	NotifyOn   []string `yaml:"notify_on,omitempty"` // late, down, recovered
	ServerPort int      `yaml:"server_port,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		NotifyOn:   []string{"down", "recovered"},
		ServerPort: 8080,
	}
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cron-health"), nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func GetDBPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data.db"), nil
}

func Load() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing values
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 8080
	}
	if len(cfg.NotifyOn) == 0 {
		cfg.NotifyOn = []string{"down", "recovered"}
	}

	// Migrate legacy webhook_url to new structure
	if cfg.WebhookURL != "" && cfg.Notifications.Webhook.URL == "" {
		cfg.Notifications.Webhook.Enabled = true
		cfg.Notifications.Webhook.URL = cfg.WebhookURL
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func Exists() bool {
	path, err := GetConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// GetEffectiveWebhookURL returns the webhook URL from either the new or legacy config
func (c *Config) GetEffectiveWebhookURL() string {
	if c.Notifications.Webhook.Enabled && c.Notifications.Webhook.URL != "" {
		return c.Notifications.Webhook.URL
	}
	return c.WebhookURL
}

// IsTelegramEnabled returns whether Telegram notifications are enabled
func (c *Config) IsTelegramEnabled() bool {
	return c.Notifications.Telegram.Enabled &&
		c.Notifications.Telegram.BotToken != "" &&
		c.Notifications.Telegram.ChatID != ""
}
