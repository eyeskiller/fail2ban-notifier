package connectors

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
	"github.com/eyeskiller/fail2ban-notifier/pkg/types"
)

func TestGetInterpreter(t *testing.T) {
	tests := []struct {
		path        string
		interpreter string
		argsCount   int
	}{
		{"/path/to/script.sh", "bash", 1},
		{"/path/to/script.bash", "bash", 1},
		{"/path/to/script.py", "python3", 1},
		{"/path/to/script.js", "node", 1},
		{"/path/to/script.rb", "ruby", 1},
		{"/path/to/script.pl", "perl", 1},
		{"/path/to/binary", "/path/to/binary", 0},
	}

	for _, test := range tests {
		interpreter, args := getInterpreter(test.path)
		if interpreter != test.interpreter {
			t.Errorf("expected interpreter %s for path %s, got %s", test.interpreter, test.path, interpreter)
		}
		if len(args) != test.argsCount {
			t.Errorf("expected %d args, got %d", test.argsCount, len(args))
		}
	}
}

func TestExecuteHTTPConnector(t *testing.T) {
	var receivedBody string
	var receivedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes := make([]byte, 1024)
		n, _ := r.Body.Read(bodyBytes)
		receivedBody = string(bodyBytes[:n])
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Connectors = []config.ConnectorConfig{
		{
			Name:    "webhook",
			Type:    config.ConnectorTypeHTTP,
			Enabled: true,
			Settings: map[string]string{
				"url":                  server.URL,
				"header_Authorization": "Bearer test-token",
			},
			Timeout: 5,
		},
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)

	data := &types.NotificationData{
		IP:       "1.2.3.4",
		Jail:     "ssh",
		Action:   "ban",
		Time:     time.Now(),
		Country:  "Slovakia",
		Region:   "Bratislava",
		City:     "Bratislava",
		ISP:      "Test ISP",
		Hostname: "test-host",
		Failures: 3,
	}

	err := m.Execute("webhook", data)
	if err != nil {
		t.Fatalf("expected execution to succeed, got: %v", err)
	}

	if receivedAuthHeader != "Bearer test-token" {
		t.Errorf("expected Authorization header to be 'Bearer test-token', got '%s'", receivedAuthHeader)
	}

	if receivedBody == "" {
		t.Error("expected server to receive a payload body, but got empty string")
	}
}

func TestExecuteScriptConnector(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping bash script test on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "f2b-connectors-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "test.sh")
	// Write a simple bash script that checks for custom env vars and stdin
	scriptContent := `#!/bin/bash
if [ "$F2B_IP" != "5.6.7.8" ]; then
    echo "F2B_IP got: $F2B_IP" >&2
    exit 1
fi
if [ "$F2B_JAIL" != "nginx" ]; then
    echo "F2B_JAIL got: $F2B_JAIL" >&2
    exit 1
fi
if [ "$CUSTOM_SETTING" != "custom-val" ]; then
    echo "CUSTOM_SETTING got: $CUSTOM_SETTING" >&2
    exit 1
fi
exit 0
`
	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("failed to write script file: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Connectors = []config.ConnectorConfig{
		{
			Name:    "test-script",
			Type:    config.ConnectorTypeScript,
			Enabled: true,
			Path:    scriptPath,
			Settings: map[string]string{
				"CUSTOM_SETTING": "custom-val",
			},
			Timeout: 5,
		},
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)

	data := &types.NotificationData{
		IP:     "5.6.7.8",
		Jail:   "nginx",
		Action: "ban",
		Time:   time.Now(),
	}

	err = m.Execute("test-script", data)
	if err != nil {
		t.Errorf("expected script execution to succeed, got: %v", err)
	}
}

func TestDiscoverConnectors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping discovery test on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "f2b-discover-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create executable scripts
	for _, name := range []string{"discord.sh", "slack.py", "teams.js"} {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/bash\nexit 0"), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	// Create a non-executable file (should be skipped)
	nonExecPath := filepath.Join(tmpDir, "readme.txt")
	if err := os.WriteFile(nonExecPath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create readme.txt: %v", err)
	}

	// Create a subdirectory (should be skipped)
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.ConnectorPath = tmpDir
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)

	discovered, err := m.DiscoverConnectors()
	if err != nil {
		t.Fatalf("DiscoverConnectors failed: %v", err)
	}

	if len(discovered) != 3 {
		t.Fatalf("expected 3 discovered connectors, got %d", len(discovered))
	}

	names := make(map[string]bool)
	for _, c := range discovered {
		names[c.Name] = true
		if !filepath.IsAbs(c.Path) {
			t.Errorf("connector %q path is not absolute: %s", c.Name, c.Path)
		}
	}

	for _, expected := range []string{"discord", "slack", "teams"} {
		if !names[expected] {
			t.Errorf("expected connector %q not found among %v", expected, names)
		}
	}
}

func TestDiscoverConnectorsEmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "f2b-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.DefaultConfig()
	cfg.ConnectorPath = tmpDir
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)

	discovered, err := m.DiscoverConnectors()
	if err != nil {
		t.Fatalf("DiscoverConnectors failed: %v", err)
	}
	if len(discovered) != 0 {
		t.Errorf("expected 0 connectors, got %d", len(discovered))
	}
}

func TestDiscoverConnectorsNonexistentDir(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ConnectorPath = "/nonexistent/path"
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)

	discovered, err := m.DiscoverConnectors()
	if err != nil {
		t.Fatalf("DiscoverConnectors failed: %v", err)
	}
	if len(discovered) != 0 {
		t.Errorf("expected 0 connectors for nonexistent dir, got %d", len(discovered))
	}
}

func TestValidateConnectorScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping script validation test on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "f2b-validate-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "valid.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 0"), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.DefaultConfig(), logger)

	t.Run("valid script", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "test",
			Type: config.ConnectorTypeScript,
			Path: scriptPath,
		})
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("non-absolute path", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "relative",
			Type: config.ConnectorTypeScript,
			Path: "relative/path.sh",
		})
		if err == nil {
			t.Error("expected error for relative path")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "missing",
			Type: config.ConnectorTypeScript,
			Path: "/nonexistent/script.sh",
		})
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("not executable", func(t *testing.T) {
		nonExecPath := filepath.Join(tmpDir, "not-exec.sh")
		if err := os.WriteFile(nonExecPath, []byte("exit 0"), 0644); err != nil {
			t.Fatalf("failed to create non-exec file: %v", err)
		}
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "not-exec",
			Type: config.ConnectorTypeScript,
			Path: nonExecPath,
		})
		if err == nil {
			t.Error("expected error for non-executable file")
		}
	})
}

func TestValidateConnectorHTTP(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.DefaultConfig(), logger)

	t.Run("valid HTTP connector", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "webhook",
			Type: config.ConnectorTypeHTTP,
			Settings: map[string]string{
				"url": "https://example.com/webhook",
			},
		})
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("missing URL", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "webhook",
			Type: config.ConnectorTypeHTTP,
		})
		if err == nil {
			t.Error("expected error for missing URL")
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		err := m.ValidateConnector(&config.ConnectorConfig{
			Name: "bad",
			Type: "invalid-type",
		})
		if err == nil {
			t.Error("expected error for unknown type")
		}
	})
}

func TestGetConnectorStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping status test on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "f2b-status-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "active.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexit 0"), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Connectors = []config.ConnectorConfig{
		{
			Name:    "active",
			Type:    config.ConnectorTypeScript,
			Enabled: true,
			Path:    scriptPath,
		},
		{
			Name:    "disabled",
			Type:    config.ConnectorTypeScript,
			Enabled: false,
			Path:    scriptPath,
		},
		{
			Name:    "invalid",
			Type:    config.ConnectorTypeScript,
			Enabled: true,
			Path:    "/nonexistent/script.sh",
		},
	}

	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(cfg, logger)
	status := m.GetConnectorStatus()

	if len(status) != 3 {
		t.Fatalf("expected 3 status entries, got %d", len(status))
	}

	tests := []struct {
		name           string
		expectedStatus string
	}{
		{"active", "ready"},
		{"disabled", "disabled"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		s, ok := status[tt.name]
		if !ok {
			t.Errorf("missing status for connector %q", tt.name)
			continue
		}
		if s.Status != tt.expectedStatus {
			t.Errorf("connector %q status = %q, want %q", tt.name, s.Status, tt.expectedStatus)
		}
		if tt.name == "invalid" && s.Error == "" {
			t.Errorf("expected error message for invalid connector")
		}
	}
}
