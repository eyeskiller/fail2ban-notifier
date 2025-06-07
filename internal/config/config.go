package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Connector types
const (
	ConnectorTypeScript     = "script"
	ConnectorTypeExecutable = "executable"
	ConnectorTypeHTTP       = "http"
)

// GeoIP service types
const (
	GeoIPServiceIPAPI         = "ipapi"
	GeoIPServiceIPGeolocation = "ipgeolocation"
)

// File permissions
const (
	DirPermission  = 0750
	FilePermission = 0600
)

// Config represents the application configuration
type Config struct {
	Connectors    []ConnectorConfig `json:"connectors"`
	ConnectorPath string            `json:"connector_path"`
	GeoIP         GeoIPConfig       `json:"geoip"`
	Debug         bool              `json:"debug"`
	LogLevel      string            `json:"log_level"`
	Timeout       int               `json:"timeout"`
}

// ConnectorConfig defines a notification connector
type ConnectorConfig struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // "script", "executable", or "http"
	Enabled     bool              `json:"enabled"`
	Path        string            `json:"path"`        // Path to script/executable
	Settings    map[string]string `json:"settings"`    // Environment variables or config
	Timeout     int               `json:"timeout"`     // Timeout in seconds (default: 30)
	RetryCount  int               `json:"retry_count"` // Number of retries on failure
	RetryDelay  int               `json:"retry_delay"` // Delay between retries in seconds
	Description string            `json:"description"` // Human-readable description
}

// GeoIPConfig contains geolocation API settings
type GeoIPConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key,omitempty"`
	Service string `json:"service"` // "ipapi" or "ipgeolocation"
	Cache   bool   `json:"cache"`   // Cache geolocation results
	TTL     int    `json:"ttl"`     // Cache TTL in seconds
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Connectors:    []ConnectorConfig{},
		ConnectorPath: "/etc/fail2ban/connectors",
		GeoIP: GeoIPConfig{
			Enabled: true,
			Service: "ipapi",
			Cache:   true,
			TTL:     3600, // 1 hour
		},
		Debug:    false,
		LogLevel: "info",
		Timeout:  30,
	}
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config if it doesn't exist
		return config, SaveConfig(configPath, config)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(configPath string, config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, DirPermission); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, FilePermission); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConnector validates a single connector configuration
func validateConnector(_ *Config, i int, connector *ConnectorConfig) error {
	if connector.Name == "" {
		return fmt.Errorf("connector[%d]: name cannot be empty", i)
	}

	if connector.Type == "" {
		return fmt.Errorf("connector[%d] (%s): type cannot be empty", i, connector.Name)
	}

	validTypes := []string{ConnectorTypeScript, ConnectorTypeExecutable, ConnectorTypeHTTP}
	isValidType := false
	for _, t := range validTypes {
		if connector.Type == t {
			isValidType = true
			break
		}
	}

	if !isValidType {
		return fmt.Errorf("connector[%d] (%s): invalid type '%s', must be '%s', '%s', or '%s'",
			i, connector.Name, connector.Type, ConnectorTypeScript, ConnectorTypeExecutable, ConnectorTypeHTTP)
	}

	if connector.Type != ConnectorTypeHTTP && connector.Path == "" {
		return fmt.Errorf("connector[%d] (%s): path cannot be empty for type '%s'", i, connector.Name, connector.Type)
	}

	if connector.Type == ConnectorTypeHTTP {
		if _, ok := connector.Settings["url"]; !ok {
			return fmt.Errorf("connector[%d] (%s): HTTP connector must have 'url' setting", i, connector.Name)
		}
	}

	return nil
}

// validateGeoIPConfig validates the GeoIP configuration
func validateGeoIPConfig(config *Config) {
	// Validate GeoIP config
	if config.GeoIP.Service != GeoIPServiceIPAPI && config.GeoIP.Service != GeoIPServiceIPGeolocation {
		config.GeoIP.Service = GeoIPServiceIPAPI
	}

	if config.GeoIP.TTL <= 0 {
		config.GeoIP.TTL = 3600
	}
}

// ValidateConfig validates the configuration
func ValidateConfig(config *Config) error {
	if config.ConnectorPath == "" {
		return fmt.Errorf("connector_path cannot be empty")
	}

	if config.Timeout <= 0 {
		config.Timeout = 30
	}

	// Validate each connector
	for i, connector := range config.Connectors {
		connectorCopy := connector // Create a local copy to avoid memory aliasing
		if err := validateConnector(config, i, &connectorCopy); err != nil {
			return err
		}

		// Set default values
		if connector.Timeout <= 0 {
			config.Connectors[i].Timeout = config.Timeout
		}

		if connector.RetryCount < 0 {
			config.Connectors[i].RetryCount = 0
		}

		if connector.RetryDelay <= 0 {
			config.Connectors[i].RetryDelay = 5
		}
	}

	// Validate GeoIP configuration
	validateGeoIPConfig(config)

	return nil
}

// GetEnabledConnectors returns only enabled connectors
func (c *Config) GetEnabledConnectors() []ConnectorConfig {
	var enabled []ConnectorConfig
	for _, connector := range c.Connectors {
		if connector.Enabled {
			enabled = append(enabled, connector)
		}
	}
	return enabled
}

// GetConnectorByName returns a connector by name
func (c *Config) GetConnectorByName(name string) (*ConnectorConfig, bool) {
	for _, connector := range c.Connectors {
		if connector.Name == name {
			return &connector, true
		}
	}
	return nil, false
}

// AddConnector adds a new connector to the configuration
func (c *Config) AddConnector(connector *ConnectorConfig) {
	c.Connectors = append(c.Connectors, *connector)
}

// RemoveConnector removes a connector by name
func (c *Config) RemoveConnector(name string) bool {
	for i, connector := range c.Connectors {
		if connector.Name == name {
			c.Connectors = append(c.Connectors[:i], c.Connectors[i+1:]...)
			return true
		}
	}
	return false
}

// UpdateConnector updates an existing connector
func (c *Config) UpdateConnector(name string, updatedConnector *ConnectorConfig) bool {
	for i, connector := range c.Connectors {
		if connector.Name == name {
			c.Connectors[i] = *updatedConnector
			return true
		}
	}
	return false
}

// createDiscordConnector creates a sample Discord connector
func createDiscordConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "discord",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/discord.sh",
		Settings: map[string]string{
			"DISCORD_WEBHOOK_URL": "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN",
			"DISCORD_USERNAME":    "Fail2Ban",
			"DISCORD_AVATAR_URL":  "",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send notifications to Discord via webhook",
	}
}

// createTeamsConnector creates a sample Microsoft Teams connector
func createTeamsConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "teams",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/teams.sh",
		Settings: map[string]string{
			"TEAMS_WEBHOOK_URL": "https://your-tenant.webhook.office.com/webhookb2/YOUR_WEBHOOK_URL",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send notifications to Microsoft Teams via webhook",
	}
}

// createSlackConnector creates a sample Slack connector
func createSlackConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "slack",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/slack.sh",
		Settings: map[string]string{
			"SLACK_WEBHOOK_URL": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
			"SLACK_CHANNEL":     "#security",
			"SLACK_USERNAME":    "fail2ban",
			"SLACK_ICON_EMOJI":  ":cop:",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send notifications to Slack via webhook",
	}
}

// createTelegramConnector creates a sample Telegram connector
func createTelegramConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "telegram",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/telegram.sh",
		Settings: map[string]string{
			"TELEGRAM_BOT_TOKEN": "YOUR_BOT_TOKEN",
			"TELEGRAM_CHAT_ID":   "YOUR_CHAT_ID",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send notifications to Telegram via bot API",
	}
}

// createEmailConnector creates a sample Email connector
func createEmailConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "email",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/email.py",
		Settings: map[string]string{
			"EMAIL_SMTP_SERVER":    "localhost",
			"EMAIL_SMTP_PORT":      "587",
			"EMAIL_SMTP_USER":      "",
			"EMAIL_SMTP_PASSWORD":  "",
			"EMAIL_SMTP_TLS":       "true",
			"EMAIL_FROM":           "fail2ban@localhost",
			"EMAIL_TO":             "admin@localhost",
			"EMAIL_SUBJECT_PREFIX": "[Fail2Ban]",
		},
		Timeout:     60,
		RetryCount:  3,
		RetryDelay:  10,
		Description: "Send notifications via email using SMTP",
	}
}

// createWebhookConnector creates a sample webhook connector
func createWebhookConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "webhook",
		Type:    ConnectorTypeHTTP,
		Enabled: false,
		Path:    "",
		Settings: map[string]string{
			"url":                  "https://your-api.com/webhook",
			"header_Content-Type":  "application/json",
			"header_Authorization": "Bearer YOUR_TOKEN",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send notifications to a custom HTTP endpoint",
	}
}

// CreateSampleConfig creates a configuration with sample connectors
func CreateSampleConfig() *Config {
	config := DefaultConfig()

	// Add sample connectors
	sampleConnectors := []ConnectorConfig{
		createDiscordConnector(),
		createTeamsConnector(),
		createSlackConnector(),
		createTelegramConnector(),
		createEmailConnector(),
		createWebhookConnector(),
	}

	config.Connectors = sampleConnectors
	return config
}
