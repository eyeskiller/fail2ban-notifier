package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
	"github.com/eyeskiller/fail2ban-notifier/internal/connectors"
	"github.com/eyeskiller/fail2ban-notifier/internal/geoip"
	"github.com/eyeskiller/fail2ban-notifier/internal/version"
	"github.com/eyeskiller/fail2ban-notifier/pkg/types"
)

func main() {
	var (
		ip         = flag.String("ip", "", "IP address that was banned/unbanned")
		jail       = flag.String("jail", "", "Fail2ban jail name")
		action     = flag.String("action", "ban", "Action performed (ban/unban)")
		failures   = flag.Int("failures", 0, "Number of failures")
		configPath = flag.String("config", "/etc/fail2ban/fail2ban-notify.json", "Path to configuration file")
		initConfig = flag.Bool("init", false, "Initialize configuration file")
		discover   = flag.Bool("discover", false, "Discover available connectors")
		test       = flag.String("test", "", "Test specific connector")
		status     = flag.Bool("status", false, "Show connector status")
		debug      = flag.Bool("debug", false, "Enable debug logging")
		versionFlag = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	// Setup logging
	logger := log.New(os.Stderr, "[fail2ban-notify] ", log.LstdFlags)

	if *versionFlag {
		fmt.Println(version.GetBuildInfo())
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	if *debug {
		cfg.Debug = true
	}

	if cfg.Debug {
		logger.Printf("Loaded configuration from %s", *configPath)
	}

	// Initialize configuration
	if *initConfig {
		sampleConfig := config.CreateSampleConfig()

		// Try to discover existing connectors
		connectorManager := connectors.NewManager(cfg, logger)
		discovered, err := connectorManager.DiscoverConnectors()
		if err != nil {
			logger.Printf("Warning: Failed to discover connectors: %v", err)
		} else {
			// Merge discovered connectors with sample config
			for _, conn := range discovered {
				sampleConfig.AddConnector(conn)
			}
		}

		if err := config.SaveConfig(*configPath, sampleConfig); err != nil {
			logger.Fatalf("Failed to create config file: %v", err)
		}

		fmt.Printf("Configuration file created at: %s\n", *configPath)
		fmt.Printf("Connector directory: %s\n", sampleConfig.ConnectorPath)
		fmt.Printf("Found %d connectors\n", len(discovered))
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("1. Edit the configuration file to enable and configure your notification services")
		fmt.Println("2. Test connectors: sudo fail2ban-notify -test <connector-name>")
		fmt.Println("3. Add 'notify' action to your fail2ban jails")
		return
	}

	// Create connector manager
	connectorManager := connectors.NewManager(cfg, logger)

	// Discover connectors
	if *discover {
		discovered, err := connectorManager.DiscoverConnectors()
		if err != nil {
			logger.Fatalf("Failed to discover connectors: %v", err)
		}

		fmt.Printf("Connector directory: %s\n", cfg.ConnectorPath)
		fmt.Printf("Found %d connectors:\n", len(discovered))
		for _, conn := range discovered {
			fmt.Printf("  - %s (%s): %s\n", conn.Name, conn.Type, conn.Path)
		}

		if len(discovered) > 0 {
			fmt.Println("\nTo enable connectors:")
			fmt.Printf("1. Edit configuration: sudo nano %s\n", *configPath)
			fmt.Println("2. Set enabled: true for desired connectors")
			fmt.Println("3. Configure service-specific settings")
		}
		return
	}

	// Show connector status
	if *status {
		statuses := connectorManager.GetConnectorStatus()

		fmt.Printf("Connector Status (%d total):\n", len(statuses))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		for name, status := range statuses {
			statusIcon := "❌"
			if status.Status == "ready" {
				statusIcon = "✅"
			} else if status.Status == "disabled" {
				statusIcon = "⚪"
			}

			fmt.Printf("%s %s [%s] - %s\n", statusIcon, name, status.Status, status.Type)
			if status.Description != "" {
				fmt.Printf("   %s\n", status.Description)
			}
			if status.Error != "" {
				fmt.Printf("   Error: %s\n", status.Error)
			}
		}

		fmt.Println("")
		fmt.Println("Legend: ✅ Ready  ⚪ Disabled  ❌ Invalid")
		return
	}

	// Test specific connector
	if *test != "" {
		testData := &types.NotificationData{
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

		fmt.Printf("Testing connector: %s\n", *test)
		err := connectorManager.TestConnector(*test, testData)
		if err != nil {
			logger.Fatalf("Connector test failed: %v", err)
		}
		fmt.Println("✅ Connector test passed!")
		return
	}

	// Validate required parameters for notification
	if *ip == "" || *jail == "" {
		fmt.Fprintf(os.Stderr, "Error: ip and jail parameters are required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate action
	if *action != "ban" && *action != "unban" {
		logger.Fatalf("Invalid action: %s (must be 'ban' or 'unban')", *action)
	}

	if cfg.Debug {
		logger.Printf("Processing %s action for IP %s in jail %s", *action, *ip, *jail)
	}

	// Setup GeoIP manager
	geoManager := geoip.NewManager(cfg.GeoIP, logger)

	// Perform GeoIP lookup
	var geoInfo *geoip.GeoIPInfo
	if cfg.GeoIP.Enabled {
		geoInfo, err = geoManager.Lookup(*ip)
		if err != nil {
			if cfg.Debug {
				logger.Printf("GeoIP lookup failed: %v", err)
			}
			// Continue with empty geo info
			geoInfo = &geoip.GeoIPInfo{IP: *ip}
		} else if cfg.Debug {
			logger.Printf("GeoIP lookup successful: %s -> %s", *ip, geoInfo.Country)
		}
	} else {
		geoInfo = &geoip.GeoIPInfo{IP: *ip}
	}

	// Create notification data
	notificationData := types.NotificationData{
		IP:        *ip,
		Jail:      *jail,
		Action:    *action,
		Time:      time.Now(),
		Country:   geoInfo.Country,
		Region:    geoInfo.Region,
		City:      geoInfo.City,
		ISP:       geoInfo.ISP,
		Hostname:  "", // Could be populated from reverse DNS lookup if needed
		Failures:  *failures,
		Timezone:  geoInfo.Timezone,
		Latitude:  geoInfo.Lat,
		Longitude: geoInfo.Lon,
	}

	if cfg.Debug {
		logger.Printf("Notification data: %+v", notificationData)
	}

	// Get enabled connectors
	enabledConnectors := cfg.GetEnabledConnectors()
	if len(enabledConnectors) == 0 {
		logger.Printf("Warning: No connectors enabled. Edit %s to enable notification services.", *configPath)
		return
	}

	if cfg.Debug {
		logger.Printf("Found %d enabled connectors", len(enabledConnectors))
	}

	// Execute all enabled connectors
	err = connectorManager.ExecuteAll(notificationData)
	if err != nil {
		logger.Printf("Connector execution completed with errors: %v", err)
		// Don't exit with error code as some connectors may have succeeded
		// The connector manager logs individual failures
	} else if cfg.Debug {
		logger.Printf("All connectors executed successfully")
	}

	if cfg.Debug {
		logger.Printf("Notification processing completed for IP %s", *ip)
	}
}package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NotificationData contains information about the fail2ban event
type NotificationData struct {
	IP         string    `json:"ip"`
	Jail       string    `json:"jail"`
	Action     string    `json:"action"` // "ban" or "unban"
	Time       time.Time `json:"time"`
	Country    string    `json:"country,omitempty"`
	Region     string    `json:"region,omitempty"`
	City       string    `json:"city,omitempty"`
	ISP        string    `json:"isp,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	Failures   int       `json:"failures,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Connectors    []ConnectorConfig `json:"connectors"`
	ConnectorPath string            `json:"connector_path"`
	GeoIP         GeoIPConfig       `json:"geoip"`
	Debug         bool              `json:"debug"`
}

// ConnectorConfig defines a notification connector
type ConnectorConfig struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`     // "script", "executable", or "http"
	Enabled  bool              `json:"enabled"`
	Path     string            `json:"path"`     // Path to script/executable
	Settings map[string]string `json:"settings"` // Environment variables or config
	Timeout  int               `json:"timeout"`  // Timeout in seconds (default: 30)
}

// GeoIPConfig contains geolocation API settings
type GeoIPConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key,omitempty"`
	Service string `json:"service"` // "ipapi" or "ipgeolocation"
}

// ConnectorInterface defines how connectors communicate
type ConnectorInterface interface {
	Send(data NotificationData) error
	GetName() string
}

// ScriptConnector executes external scripts/executables
type ScriptConnector struct {
	Config ConnectorConfig
}

func (s *ScriptConnector) GetName() string {
	return s.Config.Name
}

func (s *ScriptConnector) Send(data NotificationData) error {
	// Prepare the command
	var cmd *exec.Cmd

	if s.Config.Type == "script" {
		// Execute script with interpreter
		ext := filepath.Ext(s.Config.Path)
		switch ext {
		case ".sh", ".bash":
			cmd = exec.Command("bash", s.Config.Path)
		case ".py":
			cmd = exec.Command("python3", s.Config.Path)
		case ".js":
			cmd = exec.Command("node", s.Config.Path)
		case ".rb":
			cmd = exec.Command("ruby", s.Config.Path)
		default:
			// Try to execute directly
			cmd = exec.Command(s.Config.Path)
		}
	} else {
		// Execute as binary
		cmd = exec.Command(s.Config.Path)
	}

	// Set timeout
	timeout := time.Duration(s.Config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

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
	for key, value := range s.Config.Settings {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = env

	// Also pass data as JSON via stdin
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %v", err)
	}

	cmd.Stdin = strings.NewReader(string(jsonData))

	// Set up timeout
	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			done <- fmt.Errorf("connector %s failed: %v - Output: %s", s.Config.Name, err, string(output))
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("connector %s timed out after %v", s.Config.Name, timeout)
	}
}

// HTTPConnector sends HTTP requests to webhooks
type HTTPConnector struct {
	Config ConnectorConfig
}

func (h *HTTPConnector) GetName() string {
	return h.Config.Name
}

func (h *HTTPConnector) Send(data NotificationData) error {
	url, ok := h.Config.Settings["url"]
	if !ok {
		return fmt.Errorf("HTTP connector %s missing 'url' setting", h.Config.Name)
	}

	// Prepare JSON payload
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fail2ban-notify/1.0")

	for key, value := range h.Config.Settings {
		if strings.HasPrefix(key, "header_") {
			headerName := strings.TrimPrefix(key, "header_")
			req.Header.Set(headerName, value)
		}
	}

	// Set timeout
	timeout := time.Duration(h.Config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP connector failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// GeoIP service to get location information
func getGeoIPInfo(ip string, config GeoIPConfig) (country, region, city, isp string) {
	if !config.Enabled {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var url string

	switch config.Service {
	case "ipgeolocation":
		url = fmt.Sprintf("https://api.ipgeolocation.io/ipgeo?apiKey=%s&ip=%s", config.APIKey, ip)
	default: // ipapi
		url = fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,regionName,city,isp", ip)
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to get geo info: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read geo response: %v", err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse geo response: %v", err)
		return
	}

	if config.Service == "ipgeolocation" {
		if v, ok := result["country_name"].(string); ok {
			country = v
		}
		if v, ok := result["state_prov"].(string); ok {
			region = v
		}
		if v, ok := result["city"].(string); ok {
			city = v
		}
		if v, ok := result["isp"].(string); ok {
			isp = v
		}
	} else {
		if status, ok := result["status"].(string); ok && status == "success" {
			if v, ok := result["country"].(string); ok {
				country = v
			}
			if v, ok := result["regionName"].(string); ok {
				region = v
			}
			if v, ok := result["city"].(string); ok {
				city = v
			}
			if v, ok := result["isp"].(string); ok {
				isp = v
			}
		}
	}

	return
}

// Load configuration from file
func loadConfig(configPath string) (*Config, error) {
	config := &Config{
		Connectors:    []ConnectorConfig{},
		ConnectorPath: "/etc/fail2ban/connectors",
		GeoIP: GeoIPConfig{
			Enabled: true,
			Service: "ipapi",
		},
		Debug: false,
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		return config, saveConfig(configPath, config)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, config)
	return config, err
}

// Save configuration to file
func saveConfig(configPath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Discover available connectors in the connector path
func discoverConnectors(connectorPath string) ([]ConnectorConfig, error) {
	var connectors []ConnectorConfig

	if _, err := os.Stat(connectorPath); os.IsNotExist(err) {
		return connectors, nil
	}

	entries, err := os.ReadDir(connectorPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(connectorPath, name)

		// Check if file is executable
		info, err := entry.Info()
		if err != nil {
			continue
		}

		connectorType := "executable"
		if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".bash") ||
		   strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".js") ||
		   strings.HasSuffix(name, ".rb") {
			connectorType = "script"
		}

		connector := ConnectorConfig{
			Name:     strings.TrimSuffix(name, filepath.Ext(name)),
			Type:     connectorType,
			Enabled:  false, // Discovered connectors are disabled by default
			Path:     path,
			Settings: make(map[string]string),
			Timeout:  30,
		}

		// Check if executable
		if info.Mode()&0111 != 0 {
			connectors = append(connectors, connector)
		}
	}

	return connectors, nil
}

// Create connectors from config
func createConnectors(config *Config) ([]ConnectorInterface, error) {
	var connectors []ConnectorInterface

	for _, conn := range config.Connectors {
		if !conn.Enabled {
			continue
		}

		switch conn.Type {
		case "script", "executable":
			// Check if connector file exists
			if _, err := os.Stat(conn.Path); os.IsNotExist(err) {
				log.Printf("Warning: Connector %s path does not exist: %s", conn.Name, conn.Path)
				continue
			}
			connectors = append(connectors, &ScriptConnector{Config: conn})

		case "http":
			connectors = append(connectors, &HTTPConnector{Config: conn})

		default:
			log.Printf("Warning: Unknown connector type: %s for connector %s", conn.Type, conn.Name)
		}
	}

	return connectors, nil
}

func main() {
	var (
		ip         = flag.String("ip", "", "IP address that was banned/unbanned")
		jail       = flag.String("jail", "", "Fail2ban jail name")
		action     = flag.String("action", "ban", "Action performed (ban/unban)")
		failures   = flag.Int("failures", 0, "Number of failures")
		configPath = flag.String("config", "/etc/fail2ban/fail2ban-notify.json", "Path to configuration file")
		initConfig = flag.Bool("init", false, "Initialize configuration file")
		discover   = flag.Bool("discover", false, "Discover available connectors")
		debug      = flag.Bool("debug", false, "Enable debug logging")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Println("fail2ban-notify v2.0.0 - Modular notification system")
		return
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *initConfig {
		// Discover available connectors
		discovered, err := discoverConnectors(config.ConnectorPath)
		if err != nil {
			log.Printf("Warning: Failed to discover connectors: %v", err)
		}

		// Add sample configurations
		sampleConnectors := []ConnectorConfig{
			{
				Name:    "discord",
				Type:    "script",
				Enabled: false,
				Path:    "/etc/fail2ban/connectors/discord.sh",
				Settings: map[string]string{
					"DISCORD_WEBHOOK_URL": "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN",
					"DISCORD_USERNAME":    "Fail2Ban",
				},
				Timeout: 30,
			},
			{
				Name:    "teams",
				Type:    "script",
				Enabled: false,
				Path:    "/etc/fail2ban/connectors/teams.sh",
				Settings: map[string]string{
					"TEAMS_WEBHOOK_URL": "https://your-tenant.webhook.office.com/webhookb2/YOUR_WEBHOOK_URL",
				},
				Timeout: 30,
			},
			{
				Name:    "slack",
				Type:    "script",
				Enabled: false,
				Path:    "/etc/fail2ban/connectors/slack.sh",
				Settings: map[string]string{
					"SLACK_WEBHOOK_URL": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
					"SLACK_CHANNEL":     "#security",
					"SLACK_USERNAME":    "fail2ban",
				},
				Timeout: 30,
			},
			{
				Name:    "telegram",
				Type:    "script",
				Enabled: false,
				Path:    "/etc/fail2ban/connectors/telegram.sh",
				Settings: map[string]string{
					"TELEGRAM_BOT_TOKEN": "YOUR_BOT_TOKEN",
					"TELEGRAM_CHAT_ID":   "YOUR_CHAT_ID",
				},
				Timeout: 30,
			},
			{
				Name:    "webhook",
				Type:    "http",
				Enabled: false,
				Path:    "",
				Settings: map[string]string{
					"url":                  "https://your-api.com/webhook",
					"header_Authorization": "Bearer YOUR_TOKEN",
				},
				Timeout: 30,
			},
		}

		// Merge discovered and sample connectors
		config.Connectors = append(discovered, sampleConnectors...)

		if err := saveConfig(*configPath, config); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}

		fmt.Printf("Configuration file created at: %s\n", *configPath)
		fmt.Printf("Connector directory: %s\n", config.ConnectorPath)
		fmt.Printf("Discovered %d connectors\n", len(discovered))
		fmt.Println("Please edit the configuration file to enable and configure your notification services.")
		return
	}

	if *discover {
		discovered, err := discoverConnectors(config.ConnectorPath)
		if err != nil {
			log.Fatalf("Failed to discover connectors: %v", err)
		}

		fmt.Printf("Found %d connectors in %s:\n", len(discovered), config.ConnectorPath)
		for _, conn := range discovered {
			fmt.Printf("  - %s (%s): %s\n", conn.Name, conn.Type, conn.Path)
		}
		return
	}

	if *ip == "" || *jail == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *debug {
		config.Debug = true
	}

	connectors, err := createConnectors(config)
	if err != nil {
		log.Fatalf("Failed to create connectors: %v", err)
	}

	if len(connectors) == 0 {
		log.Println("No connectors configured or enabled")
		return
	}

	// Get geolocation info
	country, region, city, isp := getGeoIPInfo(*ip, config.GeoIP)

	data := NotificationData{
		IP:       *ip,
		Jail:     *jail,
		Action:   *action,
		Time:     time.Now(),
		Country:  country,
		Region:   region,
		City:     city,
		ISP:      isp,
		Failures: *failures,
	}

	if config.Debug {
		log.Printf("Notification data: %+v", data)
		log.Printf("Found %d enabled connectors", len(connectors))
	}

	// Send notifications
	for _, connector := range connectors {
		if config.Debug {
			log.Printf("Sending notification via %s", connector.GetName())
		}

		if err := connector.Send(data); err != nil {
			log.Printf("Failed to send %s notification: %v", connector.GetName(), err)
		} else if config.Debug {
			log.Printf("Successfully sent %s notification", connector.GetName())
		}
	}
}
