package geoip

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.10.10", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"invalid-ip", false},
	}

	for _, test := range tests {
		result := isPrivateIP(test.ip)
		if result != test.expected {
			t.Errorf("expected isPrivateIP(%s) to be %t, got %t", test.ip, test.expected, result)
		}
	}
}

func TestGeoIPCache(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	cfg := config.GeoIPConfig{
		Enabled: true,
		Service: "ipapi",
		Cache:   true,
		TTL:     1, // 1 second TTL
	}

	m := NewManager(cfg, logger)

	ip := "8.8.8.8"
	info := &Info{
		IP:      ip,
		Country: "United States",
		Region:  "California",
		City:    "Mountain View",
		ISP:     "Google LLC",
	}

	// Should be nil initially
	if cached := m.getCached(ip); cached != nil {
		t.Fatal("expected cache to be empty initially")
	}

	// Set cache
	m.setCached(ip, info)

	// Get cached
	cached := m.getCached(ip)
	if cached == nil {
		t.Fatal("expected cached info to not be nil")
	}
	if cached.Country != "United States" {
		t.Errorf("expected cached country 'United States', got '%s'", cached.Country)
	}

	// Wait for TTL expiration
	time.Sleep(1500 * time.Millisecond)

	if expired := m.getCached(ip); expired != nil {
		t.Error("expected cache entry to be expired")
	}

	// Clear cache
	m.setCached(ip, info)
	m.ClearCache()
	if cleared := m.getCached(ip); cleared != nil {
		t.Error("expected cache to be cleared")
	}
}

func TestGetCacheStats(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	cfg := config.GeoIPConfig{
		Enabled: true,
		Service: "ipapi",
		Cache:   true,
		TTL:     300,
	}

	m := NewManager(cfg, logger)

	stats := m.GetCacheStats()
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}

	if stats["enabled"] != true {
		t.Errorf("enabled = %v, want true", stats["enabled"])
	}
	if stats["entries"] != 0 {
		t.Errorf("entries = %v, want 0", stats["entries"])
	}
	if stats["ttl_seconds"] != 300 {
		t.Errorf("ttl_seconds = %v, want 300", stats["ttl_seconds"])
	}
	if stats["service"] != "ipapi" {
		t.Errorf("service = %v, want ipapi", stats["service"])
	}
}

func TestGetCacheStatsWithEntries(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.GeoIPConfig{Service: "ipapi", Cache: true, TTL: 60}, logger)

	m.setCached("8.8.8.8", &Info{IP: "8.8.8.8"})
	m.setCached("1.1.1.1", &Info{IP: "1.1.1.1"})

	stats := m.GetCacheStats()
	if stats["entries"] != 2 {
		t.Errorf("entries = %v, want 2", stats["entries"])
	}
}

func TestGetAvailableServicesDefault(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.GeoIPConfig{Service: "ipapi"}, logger)

	services := m.GetAvailableServices()
	serviceSet := make(map[string]bool)
	for _, s := range services {
		serviceSet[s] = true
	}

	if !serviceSet["ipapi"] {
		t.Errorf("expected 'ipapi' in services, got %v", services)
	}
	if serviceSet["ipgeolocation"] {
		t.Errorf("'ipgeolocation' should not be available without API key")
	}
}

func TestGetAvailableServicesWithAPIKey(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.GeoIPConfig{
		Service: "ipapi",
		APIKey:  "test-key",
	}, logger)

	services := m.GetAvailableServices()
	serviceSet := make(map[string]bool)
	for _, s := range services {
		serviceSet[s] = true
	}

	if !serviceSet["ipapi"] {
		t.Errorf("expected 'ipapi' in services, got %v", services)
	}
	if !serviceSet["ipgeolocation"] {
		t.Errorf("expected 'ipgeolocation' in services, got %v", services)
	}
}

func TestServiceGetName(t *testing.T) {
	ipapi := &IPAPIService{}
	if name := ipapi.GetName(); name != "ip-api.com" {
		t.Errorf("IPAPIService.GetName() = %q, want %q", name, "ip-api.com")
	}

	ipgeo := &IPGeolocationService{apiKey: "test"}
	if name := ipgeo.GetName(); name != "ipgeolocation.io" {
		t.Errorf("IPGeolocationService.GetName() = %q, want %q", name, "ipgeolocation.io")
	}
}

func TestValidateServiceUnknown(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", log.LstdFlags)
	m := NewManager(config.GeoIPConfig{}, logger)

	err := m.ValidateService("nonexistent")
	if err == nil {
		t.Error("expected error for unknown service")
	}
}


