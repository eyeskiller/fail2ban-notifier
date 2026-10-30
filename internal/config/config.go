package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Connector types
const (
	ConnectorTypeScript     = "script"
	ConnectorTypeExecutable = "executable"
	ConnectorTypeHTTP       = "http"
)

// GeoIP service types
const (
	GeoIPServiceIPWhois       = "ipwhois"
	GeoIPServiceGeoLite2      = "geolite2"
	GeoIPServiceIPAPI         = "ipapi"
	GeoIPServiceIPGeolocation = "ipgeolocation"
)

// GeoIP defaults
const (
	DefaultGeoIPCachePath    = "/var/cache/fail2ban-notifier/geoip.json"
	DefaultGeoLite2CityDB    = "/usr/share/GeoIP/GeoLite2-City.mmdb"
	DefaultGeoLite2ASNDB     = "/usr/share/GeoIP/GeoLite2-ASN.mmdb"
	DefaultGeoIPTTL          = 86400
	DefaultGeoIPService      = GeoIPServiceIPWhois
	DefaultGeoIPCacheMaxSize = 10000
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

	// Warnings contains non-fatal configuration problems discovered during
	// validation. It is never serialized.
	Warnings []string `json:"-"`
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
	Enabled           bool   `json:"enabled"`
	APIKey            string `json:"api_key,omitempty"`
	Service           string `json:"service"`             // ipwhois, geolite2, ipapi, or ipgeolocation
	Cache             bool   `json:"cache"`               // Persist geolocation results across processes
	TTL               int    `json:"ttl"`                 // Cache TTL in seconds
	CachePath         string `json:"cache_path"`          // Persistent cache file
	AllowInsecureHTTP bool   `json:"allow_insecure_http"` // Explicit opt-in for the HTTP-only ipapi provider
	CityDBPath        string `json:"city_db_path"`        // MaxMind GeoLite2 City database
	ASNDBPath         string `json:"asn_db_path"`         // MaxMind GeoLite2 ASN database
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Connectors:    []ConnectorConfig{},
		ConnectorPath: "/etc/fail2ban/connectors",
		GeoIP: GeoIPConfig{
			Enabled:    true,
			Service:    DefaultGeoIPService,
			Cache:      true,
			TTL:        DefaultGeoIPTTL,
			CachePath:  DefaultGeoIPCachePath,
			CityDBPath: DefaultGeoLite2CityDB,
			ASNDBPath:  DefaultGeoLite2ASNDB,
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

	data, err := os.ReadFile(configPath) // #nosec G304
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
func validateConnector(i int, connector *ConnectorConfig) error {
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

// validateGeoIPConfig validates the GeoIP configuration.
//
// Omitted values receive secure defaults. Unknown provider names are never
// silently replaced; instead a non-fatal warning is emitted and GeoIP
// enrichment is skipped at runtime. GeoIP problems must not prevent
// notifications from being delivered.
func validateGeoIPConfig(config *Config) {
	geo := &config.GeoIP
	config.Warnings = nil

	if geo.Service == "" {
		geo.Service = DefaultGeoIPService
	}

	validServices := []string{
		GeoIPServiceIPWhois,
		GeoIPServiceGeoLite2,
		GeoIPServiceIPAPI,
		GeoIPServiceIPGeolocation,
	}
	isValidService := false
	for _, service := range validServices {
		if geo.Service == service {
			isValidService = true
			break
		}
	}
	if !isValidService {
		config.Warnings = append(config.Warnings, fmt.Sprintf(
			"geoip.service: unsupported provider %q (supported: %s); GeoIP enrichment will be skipped",
			geo.Service, strings.Join(validServices, ", "),
		))
	}

	if geo.TTL <= 0 {
		geo.TTL = DefaultGeoIPTTL
	}

	if geo.CachePath == "" {
		geo.CachePath = DefaultGeoIPCachePath
	} else if !filepath.IsAbs(geo.CachePath) {
		config.Warnings = append(config.Warnings, fmt.Sprintf(
			"geoip.cache_path %q is not absolute; using %s", geo.CachePath, DefaultGeoIPCachePath,
		))
		geo.CachePath = DefaultGeoIPCachePath
	}

	if geo.CityDBPath == "" {
		geo.CityDBPath = DefaultGeoLite2CityDB
	} else if !filepath.IsAbs(geo.CityDBPath) {
		config.Warnings = append(config.Warnings, fmt.Sprintf(
			"geoip.city_db_path %q is not absolute; using %s", geo.CityDBPath, DefaultGeoLite2CityDB,
		))
		geo.CityDBPath = DefaultGeoLite2CityDB
	}

	if geo.ASNDBPath == "" {
		geo.ASNDBPath = DefaultGeoLite2ASNDB
	} else if !filepath.IsAbs(geo.ASNDBPath) {
		config.Warnings = append(config.Warnings, fmt.Sprintf(
			"geoip.asn_db_path %q is not absolute; using %s", geo.ASNDBPath, DefaultGeoLite2ASNDB,
		))
		geo.ASNDBPath = DefaultGeoLite2ASNDB
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
		if err := validateConnector(i, &connectorCopy); err != nil {
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
	for i := range c.Connectors {
		if c.Connectors[i].Name == name {
			return &c.Connectors[i], true
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
		Settings: map[string]string{ // #nosec G101
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

// createPagerDutyConnector creates a sample PagerDuty connector
func createPagerDutyConnector() ConnectorConfig {
	return ConnectorConfig{
		Name:    "pagerduty",
		Type:    ConnectorTypeScript,
		Enabled: false,
		Path:    "/etc/fail2ban/connectors/pagerduty.sh",
		Settings: map[string]string{
			"PAGERDUTY_ROUTING_KEY": "YOUR_PAGERDUTY_ROUTING_KEY",
		},
		Timeout:     30,
		RetryCount:  2,
		RetryDelay:  5,
		Description: "Send incidents to PagerDuty via Events API v2",
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
		createPagerDutyConnector(),
		createWebhookConnector(),
	}

	config.Connectors = sampleConnectors
	return config
}
