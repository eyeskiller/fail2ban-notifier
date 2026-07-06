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
