package channels

import (
	"fmt"
	"os"
)

// Config holds the configuration for the Telegram channel
type Config struct {
	// BotToken is the Telegram bot token
	BotToken string `json:"bot_token"`

	// WebhookURL is optional, if set the bot will use webhook mode instead of long polling
	WebhookURL string `json:"webhook_url,omitempty"`

	// WebhookPort is the port for the webhook server (default: 8443)
	WebhookPort int `json:"webhook_port,omitempty"`

	// UseWebhook enables webhook mode (default: false, use long polling)
	UseWebhook bool `json:"use_webhook,omitempty"`

	// Debug enables debug logging
	Debug bool `json:"debug,omitempty"`

	// AllowedUsers is a list of allowed user IDs (empty means all users are allowed)
	AllowedUsers []int64 `json:"allowed_users,omitempty"`

	// AllowedGroups is a list of allowed group IDs (empty means all groups are allowed)
	AllowedGroups []int64 `json:"allowed_groups,omitempty"`
}

// LoadConfig loads configuration from environment variables or defaults
func LoadConfig() (*Config, error) {
	cfg := &Config{
		BotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		WebhookURL:    os.Getenv("TELEGRAM_WEBHOOK_URL"),
		WebhookPort:   8443,
		UseWebhook:    os.Getenv("TELEGRAM_USE_WEBHOOK") == "true",
		Debug:         os.Getenv("TELEGRAM_DEBUG") == "true",
		AllowedUsers:  make([]int64, 0),
		AllowedGroups: make([]int64, 0),
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	return cfg, nil
}

// LoadConfigFromMap loads configuration from a map
func LoadConfigFromMap(configMap map[string]interface{}) (*Config, error) {
	cfg := &Config{
		WebhookPort:   8443,
		AllowedUsers:  make([]int64, 0),
		AllowedGroups: make([]int64, 0),
	}

	if botToken, ok := configMap["bot_token"].(string); ok {
		cfg.BotToken = botToken
	} else {
		return nil, fmt.Errorf("bot_token is required in config")
	}

	if webhookURL, ok := configMap["webhook_url"].(string); ok {
		cfg.WebhookURL = webhookURL
	}

	if webhookPort, ok := configMap["webhook_port"].(float64); ok {
		cfg.WebhookPort = int(webhookPort)
	}

	if useWebhook, ok := configMap["use_webhook"].(bool); ok {
		cfg.UseWebhook = useWebhook
	}

	if debug, ok := configMap["debug"].(bool); ok {
		cfg.Debug = debug
	}

	if allowedUsers, ok := configMap["allowed_users"].([]interface{}); ok {
		for _, uid := range allowedUsers {
			if id, ok := uid.(float64); ok {
				cfg.AllowedUsers = append(cfg.AllowedUsers, int64(id))
			}
		}
	}

	if allowedGroups, ok := configMap["allowed_groups"].([]interface{}); ok {
		for _, gid := range allowedGroups {
			if id, ok := gid.(float64); ok {
				cfg.AllowedGroups = append(cfg.AllowedGroups, int64(id))
			}
		}
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BotToken == "" {
		return fmt.Errorf("bot_token is required")
	}

	if c.UseWebhook && c.WebhookURL == "" {
		return fmt.Errorf("webhook_url is required when use_webhook is true")
	}

	return nil
}
