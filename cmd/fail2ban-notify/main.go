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
}
