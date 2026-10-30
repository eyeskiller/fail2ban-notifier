package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func (w *SetupWizard) configureGeoIP() error {
	fmt.Println("▶ GeoIP Configuration")
	fmt.Println("   GeoIP enriches notifications with approximate location data.")
	fmt.Println("   It is optional and never blocks notification delivery.")
	fmt.Println()

	if !w.promptConfirm("Enable GeoIP enrichment?", w.cfg.GeoIP.Enabled) {
		w.cfg.GeoIP.Enabled = false
		if err := config.SaveConfig(w.configPath, w.cfg); err != nil {
			return fmt.Errorf("failed to save GeoIP configuration: %w", err)
		}
		fmt.Println("  ⏭  GeoIP disabled")
		fmt.Println()
		return nil
	}
	w.cfg.GeoIP.Enabled = true

	fmt.Println("   Available providers:")
	fmt.Println("   1. ipwho.is        HTTPS, no API key, recommended (about 1,000 requests/day)")
	fmt.Println("   2. GeoLite2        Fully offline, requires local MaxMind MMDB files")
	fmt.Println("   3. ip-api.com      Legacy, HTTP-only, insecure, explicit consent required")
	fmt.Println("   4. ipgeolocation.io Paid API, requires an API key")
	fmt.Println()

	current := w.cfg.GeoIP.Service
	defaultChoice := "1"
	switch current {
	case config.GeoIPServiceGeoLite2:
		defaultChoice = "2"
	case config.GeoIPServiceIPAPI:
		defaultChoice = "3"
	case config.GeoIPServiceIPGeolocation:
		defaultChoice = "4"
	case config.GeoIPServiceIPWhois, "":
		defaultChoice = "1"
	}
	if current == "" {
		current = "unset"
	}
	fmt.Printf("Select provider [%s] (current: %s): ", defaultChoice, current)
	input, _ := w.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		input = defaultChoice
	}

	switch input {
	case "", "1", "ipwhois", "ipwho.is":
		w.cfg.GeoIP.Service = config.GeoIPServiceIPWhois
		w.cfg.GeoIP.AllowInsecureHTTP = false
	case "2", "geolite2", "maxmind":
		w.cfg.GeoIP.Service = config.GeoIPServiceGeoLite2
		w.cfg.GeoIP.AllowInsecureHTTP = false
		w.configureGeoLite2Paths()
	case "3", "ipapi", "ip-api", "ip-api.com":
		if w.promptConfirm("  ip-api.com sends queried IPs over unencrypted HTTP. Enable anyway?", false) {
			w.cfg.GeoIP.Service = config.GeoIPServiceIPAPI
			w.cfg.GeoIP.AllowInsecureHTTP = true
			fmt.Println("  ⚠  Insecure HTTP GeoIP enabled at your explicit request.")
		} else {
			w.cfg.GeoIP.Service = config.GeoIPServiceIPWhois
			w.cfg.GeoIP.AllowInsecureHTTP = false
			fmt.Println("  ⏭  Insecure provider declined; using ipwho.is instead.")
		}
	case "4", "ipgeolocation", "ipgeolocation.io":
		key := w.promptInput("  ipgeolocation.io API key: ", w.cfg.GeoIP.APIKey, true)
		if key == "" {
			w.cfg.GeoIP.Service = config.GeoIPServiceIPWhois
			w.cfg.GeoIP.APIKey = ""
			w.cfg.GeoIP.AllowInsecureHTTP = false
			fmt.Println("  ⚠  No API key provided; using ipwho.is instead.")
		} else {
			w.cfg.GeoIP.Service = config.GeoIPServiceIPGeolocation
			w.cfg.GeoIP.APIKey = key
			w.cfg.GeoIP.AllowInsecureHTTP = false
		}
	default:
		fmt.Printf("  ⚠  Unknown provider %q; keeping %s.\n", input, w.cfg.GeoIP.Service)
	}

	if w.cfg.GeoIP.TTL <= 0 {
		w.cfg.GeoIP.TTL = config.DefaultGeoIPTTL
	}
	if w.cfg.GeoIP.CachePath == "" {
		w.cfg.GeoIP.CachePath = config.DefaultGeoIPCachePath
	}
	if w.cfg.GeoIP.CityDBPath == "" {
		w.cfg.GeoIP.CityDBPath = config.DefaultGeoLite2CityDB
	}
	if w.cfg.GeoIP.ASNDBPath == "" {
		w.cfg.GeoIP.ASNDBPath = config.DefaultGeoLite2ASNDB
	}

	w.cfg.GeoIP.Cache = w.promptConfirm("Cache GeoIP results on disk?", w.cfg.GeoIP.Cache)

	if err := config.SaveConfig(w.configPath, w.cfg); err != nil {
		return fmt.Errorf("failed to save GeoIP configuration: %w", err)
	}

	fmt.Printf("  ✅  GeoIP provider set to %s\n", w.cfg.GeoIP.Service)
	if w.cfg.GeoIP.Service == config.GeoIPServiceIPAPI && !w.cfg.GeoIP.AllowInsecureHTTP {
		fmt.Println("  ⚠  ipapi requires geoip.allow_insecure_http=true before lookups will run.")
	}
	fmt.Println()
	return nil
}

func (w *SetupWizard) configureGeoLite2Paths() {
	w.cfg.GeoIP.CityDBPath = w.promptInput(
		"  GeoLite2 City database path",
		defaultIfEmpty(w.cfg.GeoIP.CityDBPath, config.DefaultGeoLite2CityDB),
		false,
	)
	w.cfg.GeoIP.ASNDBPath = w.promptInput(
		"  GeoLite2 ASN database path (optional)",
		defaultIfEmpty(w.cfg.GeoIP.ASNDBPath, config.DefaultGeoLite2ASNDB),
		false,
	)

	if _, err := os.Stat(w.cfg.GeoIP.CityDBPath); err != nil {
		fmt.Printf("  ⚠  City database not found at %s. Install it before relying on GeoIP.\n", w.cfg.GeoIP.CityDBPath)
	}
	if _, err := os.Stat(w.cfg.GeoIP.ASNDBPath); err != nil {
		fmt.Printf("  ⚠  ASN database not found at %s. City-only lookups will still work.\n", w.cfg.GeoIP.ASNDBPath)
	}
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
