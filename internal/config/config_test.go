package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected default config to not be nil")
	}
	if cfg.ConnectorPath != "/etc/fail2ban/connectors" {
		t.Errorf("expected default connector path to be '/etc/fail2ban/connectors', got '%s'", cfg.ConnectorPath)
	}
	if !cfg.GeoIP.Enabled {
		t.Error("expected GeoIP to be enabled by default")
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Valid config
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected default config to be valid, got error: %v", err)
	}

	// Invalid connector path
	cfg.ConnectorPath = ""
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for empty connector_path, got nil")
	}
	cfg.ConnectorPath = "/etc/fail2ban/connectors"

	// Invalid connector type
	cfg.Connectors = []ConnectorConfig{
		{
			Name: "test",
			Type: "invalid-type",
		},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid connector type, got nil")
	}

	// Valid script connector
	cfg.Connectors = []ConnectorConfig{
		{
			Name: "test-script",
			Type: ConnectorTypeScript,
			Path: "/etc/fail2ban/connectors/test.sh",
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected script connector to be valid, got: %v", err)
	}

	// Missing URL for HTTP connector
	cfg.Connectors = []ConnectorConfig{
		{
			Name: "test-http",
			Type: ConnectorTypeHTTP,
		},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for HTTP connector missing URL, got nil")
	}

	// Valid HTTP connector
	cfg.Connectors = []ConnectorConfig{
		{
			Name: "test-http",
			Type: ConnectorTypeHTTP,
			Settings: map[string]string{
				"url": "http://localhost:8080",
			},
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected HTTP connector to be valid, got: %v", err)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "f2b-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cfg := DefaultConfig()
	cfg.Timeout = 45
	cfg.Connectors = []ConnectorConfig{
		{
			Name: "discord",
			Type: ConnectorTypeScript,
			Path: "/etc/fail2ban/connectors/discord.sh",
		},
	}

	err = SaveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Timeout != 45 {
		t.Errorf("expected loaded timeout to be 45, got %d", loaded.Timeout)
	}

	if len(loaded.Connectors) != 1 || loaded.Connectors[0].Name != "discord" {
		t.Errorf("expected 1 connector named 'discord', got %+v", loaded.Connectors)
	}
}
