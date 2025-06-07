package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"     //nolint:depguard
	"github.com/eyeskiller/fail2ban-notifier/internal/connectors" //nolint:depguard
	"github.com/eyeskiller/fail2ban-notifier/internal/geoip"      //nolint:depguard
	"github.com/eyeskiller/fail2ban-notifier/internal/version"    //nolint:depguard
	"github.com/eyeskiller/fail2ban-notifier/pkg/types"           //nolint:depguard
)

// Action types
const (
	ActionBan   = "ban"
	ActionUnban = "unban"
)

func handleInitConfig(configPath string, cfg *config.Config, logger *log.Logger) {
	sampleConfig := config.CreateSampleConfig()

	// Try to discover existing connectors
	connectorManager := connectors.NewManager(cfg, logger)
	discovered, discoverErr := connectorManager.DiscoverConnectors()
	if discoverErr != nil {
		logger.Printf("Warning: Failed to discover connectors: %v", discoverErr)
	} else {
		// Merge discovered connectors with sample config
		for _, conn := range discovered {
			connCopy := conn // Create a local copy to avoid memory aliasing
			sampleConfig.AddConnector(&connCopy)
		}
	}

	if err := config.SaveConfig(configPath, sampleConfig); err != nil {
		logger.Fatalf("Failed to create config file: %v", err)
	}

	fmt.Printf("Configuration file created at: %s\n", configPath)
	fmt.Printf("Connector directory: %s\n", sampleConfig.ConnectorPath)
	fmt.Printf("Found %d connectors\n", len(discovered))
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("1. Edit the configuration file to enable and configure your notification services")
	fmt.Println("2. Test connectors: sudo fail2ban-notify -test <connector-name>")
	fmt.Println("3. Add 'notify' action to your fail2ban jails")
}

// handleDiscoverConnectors discovers available connectors
func handleDiscoverConnectors(configPath string, cfg *config.Config, logger *log.Logger) {
	connectorManager := connectors.NewManager(cfg, logger)
	discovered, discoverErr := connectorManager.DiscoverConnectors()
	if discoverErr != nil {
		logger.Fatalf("Failed to discover connectors: %v", discoverErr)
	}

	fmt.Printf("Connector directory: %s\n", cfg.ConnectorPath)
	fmt.Printf("Found %d connectors:\n", len(discovered))
	for _, conn := range discovered {
		fmt.Printf("  - %s (%s): %s\n", conn.Name, conn.Type, conn.Path)
	}

	if len(discovered) > 0 {
		fmt.Println("\nTo enable connectors:")
		fmt.Printf("1. Edit configuration: sudo nano %s\n", configPath)
		fmt.Println("2. Set enabled: true for desired connectors")
		fmt.Println("3. Configure service-specific settings")
	}
}

// handleConnectorStatus shows the status of all connectors
func handleConnectorStatus(cfg *config.Config, logger *log.Logger) {
	connectorManager := connectors.NewManager(cfg, logger)
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
}

// handleTestConnector tests a specific connector
func handleTestConnector(testConnector string, cfg *config.Config, logger *log.Logger) {
	testData := &types.NotificationData{
		IP:       "192.168.1.100",
		Jail:     "test",
		Action:   ActionBan,
		Time:     time.Now(),
		Country:  "Test Country",
		Region:   "Test Region",
		City:     "Test City",
		ISP:      "Test ISP",
		Hostname: "test.example.com",
		Failures: 5,
	}

	fmt.Printf("Testing connector: %s\n", testConnector)
	connectorManager := connectors.NewManager(cfg, logger)
	testErr := connectorManager.TestConnector(testConnector, testData)
	if testErr != nil {
		logger.Fatalf("Connector test failed: %v", testErr)
	}
	fmt.Println("✅ Connector test passed!")
}

// handleNotification processes a notification
//
//nolint:funlen
func handleNotification(ip, jail, action string, failures int, cfg *config.Config, logger *log.Logger) {
	// Validate required parameters
	if ip == "" || jail == "" {
		_, err := fmt.Fprintf(os.Stderr, "Error: ip and jail parameters are required\n\n")
		if err != nil {
			return
		}
		flag.Usage()
		os.Exit(1)
	}

	// Validate action
	if action != ActionBan && action != ActionUnban {
		logger.Fatalf("Invalid action: %s (must be '%s' or '%s')", action, ActionBan, ActionUnban)
	}

	if cfg.Debug {
		logger.Printf("Processing %s action for IP %s in jail %s", action, ip, jail)
	}

	// Setup GeoIP manager
	geoManager := geoip.NewManager(cfg.GeoIP, logger)

	// Perform GeoIP lookup
	var geoInfo *geoip.Info
	if cfg.GeoIP.Enabled {
		geoInfo, lookupErr := geoManager.Lookup(ip)
		if lookupErr != nil {
			if cfg.Debug {
				logger.Printf("GeoIP lookup failed: %v", lookupErr)
			}
			// Continue with empty geo info
			geoInfo = &geoip.Info{IP: ip}
		} else if cfg.Debug {
			logger.Printf("GeoIP lookup successful: %s -> %s", ip, geoInfo.Country)
		}
	} else {
		geoInfo = &geoip.Info{IP: ip}
	}

	// Create notification data
	notificationData := types.NotificationData{
		IP:     ip,
		Jail:   jail,
		Action: action,
		Time:   time.Now(),
		Country: func() string {
			if geoInfo != nil {
				return geoInfo.Country
			}
			return ""
		}(),
		Region: func() string {
			if geoInfo != nil {
				return geoInfo.Region
			}
			return ""
		}(),
		City: func() string {
			if geoInfo != nil {
				return geoInfo.City
			}
			return ""
		}(),
		ISP: func() string {
			if geoInfo != nil {
				return geoInfo.ISP
			}
			return ""
		}(),
		Hostname: "", // Could be populated from reverse DNS lookup if needed
		Failures: failures,
		Timezone: func() string {
			if geoInfo != nil {
				return geoInfo.Timezone
			}
			return ""
		}(),
		Latitude: func() float64 {
			if geoInfo != nil {
				return geoInfo.Lat
			}
			return 0.0
		}(),
		Longitude: func() float64 {
			if geoInfo != nil {
				return geoInfo.Lon
			}
			return 0.0
		}(),
	}

	if cfg.Debug {
		logger.Printf("Notification data: %+v", notificationData)
	}

	// Get enabled connectors
	enabledConnectors := cfg.GetEnabledConnectors()
	if len(enabledConnectors) == 0 {
		logger.Printf("Warning: No connectors enabled. Edit %s to enable notification services.", cfg.ConnectorPath)
		return
	}

	if cfg.Debug {
		logger.Printf("Found %d enabled connectors", len(enabledConnectors))
	}

	// Execute all enabled connectors
	connectorManager := connectors.NewManager(cfg, logger)
	execErr := connectorManager.ExecuteAll(&notificationData)
	if execErr != nil {
		logger.Printf("Connector execution completed with errors: %v", execErr)
		// Don't exit with error code as some connectors may have succeeded
		// The connector manager logs individual failures
	} else if cfg.Debug {
		logger.Printf("All connectors executed successfully")
	}

	if cfg.Debug {
		logger.Printf("Notification processing completed for IP %s", ip)
	}
}

func main() {
	// Initialize build information
	version.InitBuildInfo()

	var (
		ip          = flag.String("ip", "", "IP address that was banned/unbanned")
		jail        = flag.String("jail", "", "Fail2ban jail name")
		action      = flag.String("action", ActionBan, "Action performed (ban/unban)")
		failures    = flag.Int("failures", 0, "Number of failures")
		configPath  = flag.String("config", "/etc/fail2ban/fail2ban-notify.json", "Path to configuration file")
		initConfig  = flag.Bool("init", false, "Initialize configuration file")
		discover    = flag.Bool("discover", false, "Discover available connectors")
		test        = flag.String("test", "", "Test specific connector")
		status      = flag.Bool("status", false, "Show connector status")
		debug       = flag.Bool("debug", false, "Enable debug logging")
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

	// Handle different command modes
	switch {
	case *initConfig:
		handleInitConfig(*configPath, cfg, logger)
	case *discover:
		handleDiscoverConnectors(*configPath, cfg, logger)
	case *status:
		handleConnectorStatus(cfg, logger)
	case *test != "":
		handleTestConnector(*test, cfg, logger)
	default:
		// Process notification
		handleNotification(*ip, *jail, *action, *failures, cfg, logger)
	}
}
