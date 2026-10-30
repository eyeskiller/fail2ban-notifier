package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultGeoIPConfigIsSecure(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.GeoIP.Service != GeoIPServiceIPWhois {
		t.Errorf("default service = %q, want %q", cfg.GeoIP.Service, GeoIPServiceIPWhois)
	}
	if cfg.GeoIP.TTL != DefaultGeoIPTTL {
		t.Errorf("default TTL = %d, want %d", cfg.GeoIP.TTL, DefaultGeoIPTTL)
	}
	if cfg.GeoIP.CachePath != DefaultGeoIPCachePath {
		t.Errorf("default cache path = %q, want %q", cfg.GeoIP.CachePath, DefaultGeoIPCachePath)
	}
	if cfg.GeoIP.AllowInsecureHTTP {
		t.Error("insecure HTTP must be disabled by default")
	}
}

func TestValidateGeoIPProviderNames(t *testing.T) {
	for _, service := range []string{
		GeoIPServiceIPWhois,
		GeoIPServiceGeoLite2,
		GeoIPServiceIPAPI,
		GeoIPServiceIPGeolocation,
	} {
		cfg := DefaultConfig()
		cfg.GeoIP.Service = service
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("service %q should be valid: %v", service, err)
		}
	}

	cfg := DefaultConfig()
	cfg.GeoIP.Service = "typo-service"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("unknown provider must not prevent notifications: %v", err)
	}
	if cfg.GeoIP.Service != "typo-service" {
		t.Fatal("validation must not silently replace an unknown provider")
	}
	if !containsWarning(cfg.Warnings, "unsupported provider") {
		t.Fatalf("expected unsupported provider warning, got %v", cfg.Warnings)
	}
}

func TestValidateGeoIPOmittedValuesGetDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GeoIP = GeoIPConfig{Enabled: true}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	if cfg.GeoIP.Service != GeoIPServiceIPWhois {
		t.Errorf("service = %q", cfg.GeoIP.Service)
	}
	if cfg.GeoIP.TTL != DefaultGeoIPTTL {
		t.Errorf("TTL = %d", cfg.GeoIP.TTL)
	}
	if cfg.GeoIP.CachePath != DefaultGeoIPCachePath {
		t.Errorf("cache path = %q", cfg.GeoIP.CachePath)
	}
	if cfg.GeoIP.CityDBPath != DefaultGeoLite2CityDB || cfg.GeoIP.ASNDBPath != DefaultGeoLite2ASNDB {
		t.Errorf("database paths were not defaulted: %+v", cfg.GeoIP)
	}
}

func TestValidateGeoIPReplacesRelativePathsWithWarning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GeoIP.CachePath = "relative/cache.json"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("relative cache path must not prevent notifications: %v", err)
	}
	if cfg.GeoIP.CachePath != DefaultGeoIPCachePath {
		t.Errorf("cache path = %q, want default", cfg.GeoIP.CachePath)
	}
	if !containsWarning(cfg.Warnings, "geoip.cache_path") {
		t.Fatalf("expected cache path warning, got %v", cfg.Warnings)
	}

	cfg = DefaultConfig()
	cfg.GeoIP.CityDBPath = "relative/City.mmdb"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("relative City database path must not prevent notifications: %v", err)
	}
	if cfg.GeoIP.CityDBPath != DefaultGeoLite2CityDB {
		t.Errorf("City database path = %q, want default", cfg.GeoIP.CityDBPath)
	}
	if !containsWarning(cfg.Warnings, "geoip.city_db_path") {
		t.Fatalf("expected City database warning, got %v", cfg.Warnings)
	}
}

func containsWarning(warnings []string, substring string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}

func TestLegacyIPAPIConfigPreservedWithoutConsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{
		"connector_path": "/etc/fail2ban/connectors",
		"geoip": {"enabled": true, "service": "ipapi", "cache": true, "ttl": 3600}
	}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("legacy config failed to load: %v", err)
	}
	if cfg.GeoIP.Service != GeoIPServiceIPAPI {
		t.Errorf("service = %q, want preserved ipapi", cfg.GeoIP.Service)
	}
	if cfg.GeoIP.AllowInsecureHTTP {
		t.Error("legacy config must not implicitly grant insecure HTTP consent")
	}
	if cfg.GeoIP.TTL != 3600 {
		t.Errorf("TTL = %d, want preserved 3600", cfg.GeoIP.TTL)
	}
	if cfg.GeoIP.CachePath != DefaultGeoIPCachePath {
		t.Errorf("cache path = %q, want default", cfg.GeoIP.CachePath)
	}
}

func TestGeoIPConfigJSONFieldNames(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GeoIP.AllowInsecureHTTP = true
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"service":"ipwhois"`,
		`"cache_path":"/var/cache/fail2ban-notifier/geoip.json"`,
		`"allow_insecure_http":true`,
		`"city_db_path":"/usr/share/GeoIP/GeoLite2-City.mmdb"`,
		`"asn_db_path":"/usr/share/GeoIP/GeoLite2-ASN.mmdb"`,
	} {
		if !strings.Contains(string(data), field) {
			t.Errorf("serialized config missing %s: %s", field, data)
		}
	}
}
