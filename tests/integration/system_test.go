//go:build integration
// +build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
	"github.com/eyeskiller/fail2ban-notifier/internal/connectors"
	"github.com/eyeskiller/fail2ban-notifier/internal/geoip"
	"github.com/eyeskiller/fail2ban-notifier/pkg/types"
)

func TestFullSystemIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	connectorDir := filepath.Join(tmpDir, "connectors")

	// Create connector directory
	err := os.MkdirAll(connectorDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create connector directory: %v", err)
	}

	// Create test connector script
	testScript := filepath.Join(connectorDir, "test.sh")
	scriptContent := `#!/bin/bash
echo "Test connector executed with IP: $F2B_IP, Action: $F2B_ACTION, Jail: $F2B_JAIL"
exit 0
`
	err = os.WriteFile(testScript, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Create configuration
	cfg := config.DefaultConfig()
	cfg.ConnectorPath = connectorDir
	cfg.Debug = true
	cfg.AddConnector(config.ConnectorConfig{
		Name:    "test",
		Type:    "script",
		Enabled: true,
		Path:    testScript,
		Settings: map[string]string{
			"TEST_VAR": "integration_test",
		},
		Timeout: 30,
	})

	// Save configuration
	err = config.SaveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test configuration loading
	loadedConfig, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test connector discovery
	manager := connectors.NewManager(loadedConfig, nil)
	discovered, err := manager.DiscoverConnectors()
	if err != nil {
		t.Fatalf("Failed to discover connectors: %v", err)
	}

	if len(discovered) == 0 {
		t.Error("Should discover at least one connector")
	}

	// Test GeoIP
	geoManager := geoip.NewManager(loadedConfig.GeoIP, nil)
	geoInfo, err := geoManager.Lookup("8.8.8.8") // Google DNS
	if err != nil {
		t.Errorf("GeoIP lookup failed: %v", err)
	} else {
		if geoInfo.Country == "" {
			t.Error("GeoIP should return country information")
		}
	}

	// Test notification execution
	notificationData := types.NotificationData{
		IP:       "192.168.1.100",
		Jail:     "sshd",
		Action:   "ban",
		Time:     time.Now(),
		Country:  geoInfo.Country,
		Region:   geoInfo.Region,
		City:     geoInfo.City,
		ISP:      geoInfo.ISP,
		Failures: 3,
	}

	err = manager.ExecuteAll(notificationData)
	if err != nil {
		t.Errorf("Failed to execute connectors: %v", err)
	}

	// Test connector status
	statuses := manager.GetConnectorStatus()
	if len(statuses) == 0 {
		t.Error("Should return connector statuses")
	}

	testStatus, found := statuses["test"]
	if !found {
		t.Error("Test connector status not found")
	}

	if testStatus.Status != "ready" {
		t.Errorf("Test connector status should be ready, got: %s", testStatus.Status)
	}
}

func TestBinaryExecution(t *testing.T) {
	// Skip if binary not available
	binaryPath := os.Getenv("BINARY_PATH")
	if binaryPath == "" {
		t.Skip("BINARY_PATH not set, skipping binary execution test")
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found at %s", binaryPath)
	}

	// Test version command
	cmd := exec.Command(binaryPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to execute binary: %v", err)
	}

	if len(output) == 0 {
		t.Error("Version output should not be empty")
	}

	// Test help command
	cmd = exec.Command(binaryPath, "-h")
	output, err = cmd.Output()
	// Help command may exit with code 2, so we don't check err

	if len(output) == 0 {
		t.Error("Help output should not be empty")
	}

	// Test init command (in temp directory)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	cmd = exec.Command(binaryPath, "-init", "-config", configPath)
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should be created by init command")
	}

	// Test discover command
	cmd = exec.Command(binaryPath, "-discover", "-config", configPath)
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to discover connectors: %v", err)
	}
}

func TestConnectorExecution(t *testing.T) {
	// Create temporary test setup
	tmpDir := t.TempDir()
	connectorDir := filepath.Join(tmpDir, "connectors")
	configPath := filepath.Join(tmpDir, "config.json")

	// Create connector directory
	err := os.MkdirAll(connectorDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create connector directory: %v", err)
	}

	// Test different connector types
	testCases := []struct {
		name     string
		filename string
		content  string
		Type     string
	}{
		{
			name:     "bash_script",
			filename: "test-bash.sh",
			content: `#!/bin/bash
echo "Bash connector: $F2B_IP $F2B_ACTION"
test -n "$F2B_IP" || exit 1
test -n "$F2B_ACTION" || exit 1
exit 0
`,
			Type: "script",
		},
		{
			name:     "python_script",
			filename: "test-python.py",
			content: `#!/usr/bin/env python3
import os
import sys
import json

# Check environment variables
if not os.getenv('F2B_IP'):
    sys.exit(1)
if not os.getenv('F2B_ACTION'):
    sys.exit(1)

# Try to read JSON from stdin
try:
    data = json.loads(sys.stdin.read())
    print(f"Python connector: {data['ip']} {data['action']}")
except:
    print(f"Python connector: {os.getenv('F2B_IP')} {os.getenv('F2B_ACTION')}")

sys.exit(0)
`,
			Type: "script",
		},
	}

	cfg := config.DefaultConfig()
	cfg.ConnectorPath = connectorDir
	cfg.Debug = true

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create script file
			scriptPath := filepath.Join(connectorDir, tc.filename)
			err := os.WriteFile(scriptPath, []byte(tc.content), 0755)
			if err != nil {
				t.Fatalf("Failed to create script: %v", err)
			}

			// Add connector to config
			cfg.AddConnector(config.ConnectorConfig{
				Name:    tc.name,
				Type:    tc.Type,
				Enabled: true,
				Path:    scriptPath,
				Timeout: 30,
			})
		})
	}

	// Save configuration
	err = config.SaveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test execution
	manager := connectors.NewManager(cfg, nil)
	notificationData := types.NotificationData{
		IP:       "10.0.0.1",
		Jail:     "integration-test",
		Action:   "ban",
		Time:     time.Now(),
		Failures: 1,
	}

	err = manager.ExecuteAll(notificationData)
	if err != nil {
		t.Errorf("Connector execution failed: %v", err)
	}

	// Test individual connector execution
	for _, tc := range testCases {
		t.Run(tc.name+"_individual", func(t *testing.T) {
			err := manager.Execute(tc.name, notificationData)
			if err != nil {
				t.Errorf("Individual connector execution failed: %v", err)
			}
		})
	}
}

func TestGeoIPServices(t *testing.T) {
	cfg := config.GeoIPConfig{
		Enabled: true,
		Service: "ipapi",
		Cache:   true,
		TTL:     3600,
	}

	manager := geoip.NewManager(cfg, nil)

	// Test public IP lookup
	testIPs := []string{
		"8.8.8.8", // Google DNS
		"1.1.1.1", // Cloudflare DNS
	}

	for _, ip := range testIPs {
		t.Run("lookup_"+ip, func(t *testing.T) {
			info, err := manager.Lookup(ip)
			if err != nil {
				t.Errorf("GeoIP lookup failed for %s: %v", ip, err)
				return
			}

			if info.IP != ip {
				t.Errorf("Returned IP doesn't match: expected %s, got %s", ip, info.IP)
			}

			if info.Country == "" {
				t.Error("Country should not be empty for public IP")
			}
		})
	}

	// Test private IP handling
	privateIPs := []string{
		"192.168.1.1",
		"10.0.0.1",
		"172.16.0.1",
		"127.0.0.1",
	}

	for _, ip := range privateIPs {
		t.Run("private_"+ip, func(t *testing.T) {
			info, err := manager.Lookup(ip)
			if err != nil {
				t.Errorf("Private IP lookup should not fail: %v", err)
				return
			}

			if info.Country != "Private Network" {
				t.Errorf("Private IP should return 'Private Network', got: %s", info.Country)
			}
		})
	}

	// Test cache functionality
	t.Run("cache_functionality", func(t *testing.T) {
		testIP := "8.8.8.8"

		// First lookup
		start := time.Now()
		_, err := manager.Lookup(testIP)
		if err != nil {
			t.Fatalf("First lookup failed: %v", err)
		}
		firstDuration := time.Since(start)

		// Second lookup (should be cached)
		start = time.Now()
		_, err = manager.Lookup(testIP)
		if err != nil {
			t.Fatalf("Cached lookup failed: %v", err)
		}
		secondDuration := time.Since(start)

		// Cached lookup should be significantly faster
		if secondDuration >= firstDuration {
			t.Logf("Warning: Cached lookup not faster (first: %v, second: %v)", firstDuration, secondDuration)
		}

		// Test cache stats
		stats := manager.GetCacheStats()
		if stats["enabled"] != true {
			t.Error("Cache should be enabled")
		}

		if stats["entries"].(int) == 0 {
			t.Error("Cache should have entries")
		}
	})

	// Test batch lookup
	t.Run("batch_lookup", func(t *testing.T) {
		ips := []string{"8.8.8.8", "1.1.1.1", "192.168.1.1"}
		results := manager.BatchLookup(ips)

		if len(results) != len(ips) {
			t.Errorf("Expected %d results, got %d", len(ips), len(results))
		}

		for _, ip := range ips {
			if _, found := results[ip]; !found {
				t.Errorf("Missing result for IP: %s", ip)
			}
		}
	})
}
