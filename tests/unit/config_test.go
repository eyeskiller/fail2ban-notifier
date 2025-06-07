package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.ConnectorPath == "" {
		t.Error("Default config should have connector path")
	}

	if !cfg.GeoIP.Enabled {
		t.Error("GeoIP should be enabled by default")
	}

	if cfg.GeoIP.Service != "ipapi" {
		t.Error("Default GeoIP service should be ipapi")
	}

	if cfg.Timeout <= 0 {
		t.Error("Default timeout should be positive")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  config.DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty connector path",
			config: &config.Config{
				ConnectorPath: "",
			},
			wantErr: true,
		},
		{
			name: "invalid connector type",
			config: &config.Config{
				ConnectorPath: "/tmp",
				Connectors: []config.ConnectorConfig{
					{
						Name: "test",
						Type: "invalid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "http connector without url",
			config: &config.Config{
				ConnectorPath: "/tmp",
				Connectors: []config.ConnectorConfig{
					{
						Name:     "test",
						Type:     "http",
						Settings: map[string]string{},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	// Create test config
	originalConfig := config.DefaultConfig()
	originalConfig.Debug = true
	originalConfig.AddConnector(&config.ConnectorConfig{
		Name:    "test",
		Type:    "script",
		Enabled: true,
		Path:    "/tmp/test.sh",
		Settings: map[string]string{
			"TEST_VAR": "test_value",
		},
	})

	// Save config
	err := config.SaveConfig(configPath, originalConfig)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load config
	loadedConfig, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Compare configs
	if loadedConfig.Debug != originalConfig.Debug {
		t.Error("Debug setting not preserved")
	}

	if len(loadedConfig.Connectors) != len(originalConfig.Connectors) {
		t.Error("Connectors not preserved")
	}

	// Check specific connector
	connector, found := loadedConfig.GetConnectorByName("test")
	if !found {
		t.Error("Test connector not found after load")
	}

	if connector.Type != "script" {
		t.Error("Connector type not preserved")
	}

	if connector.Settings["TEST_VAR"] != "test_value" {
		t.Error("Connector settings not preserved")
	}
}

func TestConnectorManagement(t *testing.T) {
	cfg := config.DefaultConfig()

	// Test AddConnector
	testConnector := &config.ConnectorConfig{
		Name:    "test",
		Type:    "script",
		Enabled: true,
		Path:    "/tmp/test.sh",
	}

	cfg.AddConnector(testConnector)

	// Test GetConnectorByName
	connector, found := cfg.GetConnectorByName("test")
	if !found {
		t.Error("Connector not found after adding")
	}

	if connector.Name != "test" {
		t.Error("Wrong connector returned")
	}

	// Test GetEnabledConnectors
	enabled := cfg.GetEnabledConnectors()
	if len(enabled) != 1 {
		t.Error("Expected 1 enabled connector")
	}

	// Test UpdateConnector
	updatedConnector := *testConnector
	updatedConnector.Enabled = false

	updated := cfg.UpdateConnector("test", &updatedConnector)
	if !updated {
		t.Error("UpdateConnector should return true")
	}

	enabled = cfg.GetEnabledConnectors()
	if len(enabled) != 0 {
		t.Error("Expected 0 enabled connectors after update")
	}

	// Test RemoveConnector
	removed := cfg.RemoveConnector("test")
	if !removed {
		t.Error("RemoveConnector should return true")
	}

	_, found = cfg.GetConnectorByName("test")
	if found {
		t.Error("Connector should not be found after removal")
	}
}

func TestCreateSampleConfig(t *testing.T) {
	cfg := config.CreateSampleConfig()

	if len(cfg.Connectors) == 0 {
		t.Error("Sample config should have connectors")
	}

	// Check for expected sample connectors
	expectedConnectors := []string{"discord", "teams", "slack", "telegram", "email", "webhook"}

	for _, expected := range expectedConnectors {
		_, found := cfg.GetConnectorByName(expected)
		if !found {
			t.Errorf("Sample config missing %s connector", expected)
		}
	}

	// Verify sample connectors are disabled by default
	enabled := cfg.GetEnabledConnectors()
	if len(enabled) != 0 {
		t.Error("Sample connectors should be disabled by default")
	}

	// Verify connector settings
	discord, _ := cfg.GetConnectorByName("discord")
	if discord.Type != "script" {
		t.Error("Discord connector should be script type")
	}

	if _, ok := discord.Settings["DISCORD_WEBHOOK_URL"]; !ok {
		t.Error("Discord connector should have webhook URL setting")
	}
}
