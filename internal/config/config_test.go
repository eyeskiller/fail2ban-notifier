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

func TestGetEnabledConnectors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "a", Enabled: false},
		{Name: "b", Enabled: true},
		{Name: "c", Enabled: true},
	}

	enabled := cfg.GetEnabledConnectors()
	if len(enabled) != 2 {
		t.Fatalf("expected 2 enabled connectors, got %d", len(enabled))
	}
	if enabled[0].Name != "b" || enabled[1].Name != "c" {
		t.Errorf("got %+v, want [b c]", enabled)
	}
}

func TestGetEnabledConnectorsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	enabled := cfg.GetEnabledConnectors()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled connectors, got %d", len(enabled))
	}
}

func TestGetConnectorByName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "discord"},
		{Name: "slack"},
	}

	conn, found := cfg.GetConnectorByName("discord")
	if !found {
		t.Fatal("expected to find 'discord'")
	}
	if conn.Name != "discord" {
		t.Errorf("got name %q, want %q", conn.Name, "discord")
	}

	_, found = cfg.GetConnectorByName("nonexistent")
	if found {
		t.Error("expected 'nonexistent' to not be found")
	}
}

func TestGetConnectorByNameReturnsPointer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "example", Enabled: false},
	}

	conn, _ := cfg.GetConnectorByName("example")
	conn.Enabled = true

	if !cfg.Connectors[0].Enabled {
		t.Error("modifying returned pointer should modify original")
	}
}

func TestAddConnector(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddConnector(&ConnectorConfig{Name: "new-connector"})

	if len(cfg.Connectors) != 1 {
		t.Fatalf("expected 1 connector, got %d", len(cfg.Connectors))
	}
	if cfg.Connectors[0].Name != "new-connector" {
		t.Errorf("got name %q, want %q", cfg.Connectors[0].Name, "new-connector")
	}
}

func TestAddConnectorAppendsMultiple(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddConnector(&ConnectorConfig{Name: "a"})
	cfg.AddConnector(&ConnectorConfig{Name: "b"})
	cfg.AddConnector(&ConnectorConfig{Name: "c"})

	if len(cfg.Connectors) != 3 {
		t.Fatalf("expected 3 connectors, got %d", len(cfg.Connectors))
	}
	if cfg.Connectors[2].Name != "c" {
		t.Errorf("last connector name = %q, want %q", cfg.Connectors[2].Name, "c")
	}
}

func TestRemoveConnector(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}

	if !cfg.RemoveConnector("b") {
		t.Fatal("RemoveConnector returned false for existing connector")
	}
	if len(cfg.Connectors) != 2 {
		t.Fatalf("expected 2 connectors after removal, got %d", len(cfg.Connectors))
	}
	if cfg.Connectors[0].Name != "a" || cfg.Connectors[1].Name != "c" {
		t.Errorf("got %+v, want [a c]", cfg.Connectors)
	}
}

func TestRemoveConnectorFirstAndLast(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "first"},
		{Name: "middle"},
		{Name: "last"},
	}

	// Remove first
	cfg.RemoveConnector("first")
	if len(cfg.Connectors) != 2 || cfg.Connectors[0].Name != "middle" {
		t.Errorf("after removing first, got %+v", cfg.Connectors)
	}

	// Remove last
	cfg.RemoveConnector("last")
	if len(cfg.Connectors) != 1 || cfg.Connectors[0].Name != "middle" {
		t.Errorf("after removing last, got %+v", cfg.Connectors)
	}
}

func TestRemoveConnectorNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{{Name: "a"}}

	if cfg.RemoveConnector("nonexistent") {
		t.Error("expected false for nonexistent connector")
	}
	if len(cfg.Connectors) != 1 {
		t.Errorf("expected 1 connector unchanged, got %d", len(cfg.Connectors))
	}
}

func TestUpdateConnector(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connectors = []ConnectorConfig{
		{Name: "old-name", Type: ConnectorTypeHTTP},
	}

	updated := cfg.UpdateConnector("old-name", &ConnectorConfig{
		Name: "old-name",
		Type: ConnectorTypeScript,
		Path: "/new/path.sh",
	})
	if !updated {
		t.Fatal("UpdateConnector returned false")
	}
	if cfg.Connectors[0].Type != ConnectorTypeScript {
		t.Errorf("type = %q, want %q", cfg.Connectors[0].Type, ConnectorTypeScript)
	}
	if cfg.Connectors[0].Path != "/new/path.sh" {
		t.Errorf("path = %q, want %q", cfg.Connectors[0].Path, "/new/path.sh")
	}
}

func TestUpdateConnectorNotFound(t *testing.T) {
	cfg := DefaultConfig()
	updated := cfg.UpdateConnector("nonexistent", &ConnectorConfig{Name: "x"})
	if updated {
		t.Error("expected false for nonexistent connector")
	}
}

func TestCreateSampleConfig(t *testing.T) {
	cfg := CreateSampleConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Connectors) != 7 {
		t.Fatalf("expected 7 sample connectors, got %d", len(cfg.Connectors))
	}

	expectedNames := []string{"discord", "teams", "slack", "telegram", "email", "pagerduty", "webhook"}
	for i, name := range expectedNames {
		if cfg.Connectors[i].Name != name {
			t.Errorf("connector[%d].Name = %q, want %q", i, cfg.Connectors[i].Name, name)
		}
	}
}

func TestCreateSampleConfigDefaults(t *testing.T) {
	cfg := CreateSampleConfig()
	for _, c := range cfg.Connectors {
		if c.Enabled {
			t.Errorf("connector %q should be disabled by default", c.Name)
		}
		if c.Timeout == 0 {
			t.Errorf("connector %q has zero timeout", c.Name)
		}
	}
}
