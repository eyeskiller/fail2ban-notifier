package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/YOUR_USERNAME/fail2ban-notify-go/internal/config"
	"github.com/YOUR_USERNAME/fail2ban-notify-go/pkg/types"
)

// Manager manages and executes connectors
type Manager struct {
	config *config.Config
	logger *log.Logger
}

// NewManager creates a new connector manager
func NewManager(cfg *config.Config, logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.New(os.Stdout, "[connectors] ", log.LstdFlags)
	}

	return &Manager{
		config: cfg,
		logger: logger,
	}
}

// ExecuteAll executes all enabled connectors concurrently
func (m *Manager) ExecuteAll(data types.NotificationData) error {
	enabledConnectors := m.config.GetEnabledConnectors()

	if len(enabledConnectors) == 0 {
		return fmt.Errorf("no enabled connectors found")
	}

	if m.config.Debug {
		m.logger.Printf("Executing %d connectors for IP %s", len(enabledConnectors), data.IP)
	}

	// Execute connectors concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(enabledConnectors))

	for _, connector := range enabledConnectors {
		wg.Add(1)
		go func(conn config.ConnectorConfig) {
			defer wg.Done()

			if err := m.executeConnector(conn, data); err != nil {
				errChan <- fmt.Errorf("connector %s failed: %w", conn.Name, err)
			} else if m.config.Debug {
				m.logger.Printf("Connector %s executed successfully", conn.Name)
			}
		}(connector)
	}

	// Wait for all connectors to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []string
	for err := range errChan {
		errors = append(errors, err.Error())
		m.logger.Printf("Error: %v", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("connector failures: %s", strings.Join(errors, "; "))
	}

	return nil
}

// Execute executes a specific connector by name
func (m *Manager) Execute(connectorName string, data types.NotificationData) error {
	connector, found := m.config.GetConnectorByName(connectorName)
	if !found {
		return fmt.Errorf("connector %s not found", connectorName)
	}

	if !connector.Enabled {
		return fmt.Errorf("connector %s is disabled", connectorName)
	}

	return m.executeConnector(*connector, data)
}

// executeConnector executes a single connector with retry logic
func (m *Manager) executeConnector(connector config.ConnectorConfig, data types.NotificationData) error {
	var lastErr error

	for attempt := 0; attempt <= connector.RetryCount; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(time.Duration(connector.RetryDelay) * time.Second)
			if m.config.Debug {
				m.logger.Printf("Retrying connector %s (attempt %d/%d)", connector.Name, attempt+1, connector.RetryCount+1)
			}
		}

		var err error
		switch connector.Type {
		case "script", "executable":
			err = m.executeScript(connector, data)
		case "http":
			err = m.executeHTTP(connector, data)
		default:
			return fmt.Errorf("unknown connector type: %s", connector.Type)
		}

		if err == nil {
			return nil // Success
		}

		lastErr = err
		if m.config.Debug {
			m.logger.Printf("Connector %s attempt %d failed: %v", connector.Name, attempt+1, err)
		}
	}

	return fmt.Errorf("connector %s failed after %d attempts: %w", connector.Name, connector.RetryCount+1, lastErr)
}

// executeScript executes a script or executable connector
func (m *Manager) executeScript(connector config.ConnectorConfig, data types.NotificationData) error {
	// Check if file exists and is executable
	if _, err := os.Stat(connector.Path); os.IsNotExist(err) {
		return fmt.Errorf("connector script not found: %s", connector.Path)
	}

	// Prepare the command
	var cmd *exec.Cmd

	if connector.Type == "script" {
		// Determine interpreter based on file extension
		ext := filepath.Ext(connector.Path)
		switch ext {
		case ".sh", ".bash":
			cmd = exec.Command("bash", connector.Path)
		case ".py":
			cmd = exec.Command("python3", connector.Path)
		case ".js":
			cmd = exec.Command("node", connector.Path)
		case ".rb":
			cmd = exec.Command("ruby", connector.Path)
		case ".pl":
			cmd = exec.Command("perl", connector.Path)
		default:
			// Try to execute directly (assumes shebang)
			cmd = exec.Command(connector.Path)
		}
	} else {
		// Execute as binary
		cmd = exec.Command(connector.Path)
	}

	// Set up context with timeout
	timeout := time.Duration(connector.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)

	// Prepare environment variables
	env := os.Environ()

	// Add notification data as environment variables
	env = append(env, fmt.Sprintf("F2B_IP=%s", data.IP))
	env = append(env, fmt.Sprintf("F2B_JAIL=%s", data.Jail))
	env = append(env, fmt.Sprintf("F2B_ACTION=%s", data.Action))
	env = append(env, fmt.Sprintf("F2B_TIME=%s", data.Time.Format(time.RFC3339)))
	env = append(env, fmt.Sprintf("F2B_TIMESTAMP=%d", data.Time.Unix()))
	env = append(env, fmt.Sprintf("F2B_COUNTRY=%s", data.Country))
	env = append(env, fmt.Sprintf("F2B_REGION=%s", data.Region))
	env = append(env, fmt.Sprintf("F2B_CITY=%s", data.City))
	env = append(env, fmt.Sprintf("F2B_ISP=%s", data.ISP))
	env = append(env, fmt.Sprintf("F2B_HOSTNAME=%s", data.Hostname))
	env = append(env, fmt.Sprintf("F2B_FAILURES=%d", data.Failures))

	// Add custom settings as environment variables
	for key, value := range connector.Settings {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = env

	// Pass JSON data via stdin
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}
	cmd.Stdin = bytes.NewReader(jsonData)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err = cmd.Run()

	if m.config.Debug {
		if stdout.Len() > 0 {
			m.logger.Printf("Connector %s stdout: %s", connector.Name, stdout.String())
		}
		if stderr.Len() > 0 {
			m.logger.Printf("Connector %s stderr: %s", connector.Name, stderr.String())
		}
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("connector timed out after %v", timeout)
		}
		return fmt.Errorf("execution failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// executeHTTP executes an HTTP connector
func (m *Manager) executeHTTP(connector config.ConnectorConfig, data types.NotificationData) error {
	url, ok := connector.Settings["url"]
	if !ok {
		return fmt.Errorf("HTTP connector missing 'url' setting")
	}

	// Prepare JSON payload
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fail2ban-notify/2.0")

	// Set custom headers from settings
	for key, value := range connector.Settings {
		if strings.HasPrefix(key, "header_") {
			headerName := strings.TrimPrefix(key, "header_")
			req.Header.Set(headerName, value)
		}
	}

	// Set up HTTP client with timeout
	timeout := time.Duration(connector.Timeout) * time.Second
	client := &http.Client{Timeout: timeout}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	body, _ := io.ReadAll(resp.Body)

	if m.config.Debug {
		m.logger.Printf("HTTP connector %s response: %s %s", connector.Name, resp.Status, string(body))
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

// DiscoverConnectors scans the connector directory for available connectors
func (m *Manager) DiscoverConnectors() ([]config.ConnectorConfig, error) {
	var discovered []config.ConnectorConfig

	if _, err := os.Stat(m.config.ConnectorPath); os.IsNotExist(err) {
		return discovered, nil
	}

	entries, err := os.ReadDir(m.config.ConnectorPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read connector directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(m.config.ConnectorPath, name)

		// Check if file is executable
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Skip non-executable files
		if info.Mode()&0111 == 0 {
			continue
		}

		// Determine connector type
		connectorType := "executable"
		if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".bash") ||
		   strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".js") ||
		   strings.HasSuffix(name, ".rb") || strings.HasSuffix(name, ".pl") {
			connectorType = "script"
		}

		// Create connector config
		connector := config.ConnectorConfig{
			Name:        strings.TrimSuffix(name, filepath.Ext(name)),
			Type:        connectorType,
			Enabled:     false, // Discovered connectors are disabled by default
			Path:        path,
			Settings:    make(map[string]string),
			Timeout:     30,
			RetryCount:  2,
			RetryDelay:  5,
			Description: fmt.Sprintf("Auto-discovered %s connector", connectorType),
		}

		discovered = append(discovered, connector)
	}

	return discovered, nil
}

// TestConnector tests a specific connector with sample data
func (m *Manager) TestConnector(connectorName string, testData *types.NotificationData) error {
	connector, found := m.config.GetConnectorByName(connectorName)
	if !found {
		return fmt.Errorf("connector %s not found", connectorName)
	}

	// Use default test data if not provided
	if testData == nil {
		testData = &types.NotificationData{
			IP:       "192.168.1.100",
			Jail:     "test",
			Action:   "ban",
			Time:     time.Now(),
			Country:  "Test Country",
			Region:   "Test Region",
			City:     "Test City",
			ISP:      "Test ISP",
			Hostname: "test.example.com",
			Failures: 5,
		}
	}

	m.logger.Printf("Testing connector %s with test data", connectorName)

	// Temporarily enable the connector for testing
	originalEnabled := connector.Enabled
	connector.Enabled = true
	defer func() {
		connector.Enabled = originalEnabled
	}()

	return m.executeConnector(*connector, *testData)
}

// ValidateConnector validates a connector configuration
func (m *Manager) ValidateConnector(connector config.ConnectorConfig) error {
	switch connector.Type {
	case "script", "executable":
		// Check if file exists
		if _, err := os.Stat(connector.Path); os.IsNotExist(err) {
			return fmt.Errorf("connector script not found: %s", connector.Path)
		}

		// Check if file is executable
		info, err := os.Stat(connector.Path)
		if err != nil {
			return fmt.Errorf("failed to stat connector file: %w", err)
		}

		if info.Mode()&0111 == 0 {
			return fmt.Errorf("connector file is not executable: %s", connector.Path)
		}

	case "http":
		// Validate URL setting
		if _, ok := connector.Settings["url"]; !ok {
			return fmt.Errorf("HTTP connector must have 'url' setting")
		}

	default:
		return fmt.Errorf("unknown connector type: %s", connector.Type)
	}

	return nil
}

// GetConnectorStatus returns status information for all connectors
func (m *Manager) GetConnectorStatus() map[string]ConnectorStatus {
	status := make(map[string]ConnectorStatus)

	for _, connector := range m.config.Connectors {
		connStatus := ConnectorStatus{
			Name:        connector.Name,
			Type:        connector.Type,
			Enabled:     connector.Enabled,
			Path:        connector.Path,
			Description: connector.Description,
		}

		// Validate connector
		if err := m.ValidateConnector(connector); err != nil {
			connStatus.Status = "invalid"
			connStatus.Error = err.Error()
		} else if connector.Enabled {
			connStatus.Status = "ready"
		} else {
			connStatus.Status = "disabled"
		}

		status[connector.Name] = connStatus
	}

	return status
}

// ConnectorStatus represents the status of a connector
type ConnectorStatus struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Status      string `json:"status"` // "ready", "disabled", "invalid"
	Error       string `json:"error,omitempty"`
}
