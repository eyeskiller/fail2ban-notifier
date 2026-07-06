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
