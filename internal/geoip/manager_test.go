package geoip

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func TestManagerInvalidAndPrivateIPs(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	if _, err := m.Lookup("not-an-ip"); !errors.Is(err, ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP, got %v", err)
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "::1", "fe80::1", "::ffff:192.168.1.1", "203.0.113.1"} {
		info, err := m.Lookup(ip)
		if err != nil {
			t.Fatalf("Lookup(%s) failed: %v", ip, err)
		}
		if info.Country != "Private Network" {
			t.Errorf("Lookup(%s) = %+v, want private-network marker", ip, info)
		}
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("non-public IPs caused %d external requests", got)
	}
}

func TestManagerDisabledDoesNotLookup(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	cfg.Enabled = false
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	info, err := m.Lookup("8.8.8.8")
	if err != nil || info.IP != "8.8.8.8" {
		t.Fatalf("unexpected disabled lookup result: %+v, %v", info, err)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("disabled GeoIP made %d requests", got)
	}
}

func TestManagerUnknownProvider(t *testing.T) {
	cfg := testManagerConfig(t)
	cfg.Service = "does-not-exist"
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()

	if _, err := m.Lookup("8.8.8.8"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
	if err := m.ValidateService("does-not-exist"); err == nil {
		t.Fatal("expected validation error for unknown provider")
	}
}

func TestManagerFailuresAreNotCached(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&requests, 1)
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, `{"ip":"8.8.8.8","success":true,"country":"United States","latitude":1,"longitude":1}`)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	if _, err := m.Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected first lookup to fail")
	}
	info, err := m.Lookup("8.8.8.8")
	if err != nil || info.Country != "United States" {
		t.Fatalf("second lookup failed: %+v, %v", info, err)
	}
	if _, err := m.Lookup("8.8.8.8"); err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("provider requests = %d, want 2 (failure must not be cached)", got)
	}
}

func TestManagerLookupUsesPersistentCache(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		writeJSON(w, `{"ip":"8.8.8.8","success":true,"country":"United States","latitude":1,"longitude":1}`)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m1 := NewManager(cfg, testLogger())
	m1.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)
	if _, err := m1.Lookup("8.8.8.8"); err != nil {
		t.Fatalf("first lookup failed: %v", err)
	}
	_ = m1.Close()

	m2 := NewManager(cfg, testLogger())
	m2.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)
	info, err := m2.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	_ = m2.Close()

	if info.Country != "United States" {
		t.Fatalf("unexpected cached info: %+v", info)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("provider requests = %d, want 1 across manager instances", got)
	}
}

func TestManagerCacheStatsAndClear(t *testing.T) {
	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()

	m.setCached("8.8.8.8", &Info{IP: "8.8.8.8"})
	stats := m.GetCacheStats()
	if stats["enabled"] != true || stats["persistent"] != true {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats["entries"] != 1 {
		t.Fatalf("entries = %v, want 1", stats["entries"])
	}
	if stats["service"] != config.GeoIPServiceIPWhois {
		t.Errorf("service = %v", stats["service"])
	}
	if stats["cache_path"] != cfg.CachePath {
		t.Errorf("cache_path = %v", stats["cache_path"])
	}

	m.ClearCache()
	if got := m.getCached("8.8.8.8"); got != nil {
		t.Fatal("expected cache to be cleared")
	}
	if stats := m.GetCacheStats(); stats["entries"] != 0 {
		t.Fatalf("entries after clear = %v, want 0", stats["entries"])
	}
}

func TestManagerBatchLookupDeduplicatesAndOmitsFailures(t *testing.T) {
	var requestsCounter int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsCounter, 1)
		if strings.Contains(r.URL.Path, "9.9.9.9") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		ip := "8.8.8.8"
		if strings.Contains(r.URL.Path, "1.1.1.1") {
			ip = "1.1.1.1"
		}
		writeJSON(w, `{"ip":"`+ip+`","success":true,"country":"Test","latitude":1,"longitude":1}`)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	results := m.BatchLookup([]string{"8.8.8.8", "8.8.8.8", "1.1.1.1", "9.9.9.9"})
	if _, ok := results["9.9.9.9"]; ok {
		t.Error("failed lookup should be omitted from batch results")
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v, want two successful lookups", results)
	}
	if got := atomic.LoadInt32(&requestsCounter); got != 3 {
		t.Fatalf("requests = %d, want 3 (duplicate IP should be deduplicated)", got)
	}
}

func TestManagerValidateServiceDoesNotConsumeNetwork(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	if err := m.ValidateService(config.GeoIPServiceIPWhois); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("ValidateService made %d network requests", got)
	}
}

func TestManagerValidateGeoLite2(t *testing.T) {
	cfg := testManagerConfig(t)
	cfg.Service = config.GeoIPServiceGeoLite2
	cfg.CityDBPath = cityFixture(t)
	cfg.ASNDBPath = asnFixture(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()

	if err := m.ValidateService(config.GeoIPServiceGeoLite2); err != nil {
		t.Fatalf("valid GeoLite2 configuration failed validation: %v", err)
	}

	missingCfg := cfg
	missingCfg.CityDBPath = cfg.CityDBPath + ".missing"
	missing := NewManager(missingCfg, testLogger())
	defer func() { _ = missing.Close() }()
	if err := missing.ValidateService(config.GeoIPServiceGeoLite2); err == nil {
		t.Fatal("expected missing GeoLite2 database to fail validation")
	}
}

func TestManagerAvailableServices(t *testing.T) {
	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()

	services := m.GetAvailableServices()
	want := []string{
		config.GeoIPServiceGeoLite2,
		config.GeoIPServiceIPAPI,
		config.GeoIPServiceIPGeolocation,
		config.GeoIPServiceIPWhois,
	}
	if strings.Join(services, ",") != strings.Join(want, ",") {
		t.Fatalf("services = %v, want %v", services, want)
	}
	if err := m.ValidateService(config.GeoIPServiceIPGeolocation); err == nil {
		t.Fatal("ipgeolocation without an API key should fail validation")
	}

	cfg.APIKey = "key"
	withKey := NewManager(cfg, testLogger())
	defer func() { _ = withKey.Close() }()
	if err := withKey.ValidateService(config.GeoIPServiceIPGeolocation); err != nil {
		t.Fatalf("ipgeolocation with an API key failed validation: %v", err)
	}
}

func TestManagerLegacyIPAPIWithoutConsent(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	cfg.Service = config.GeoIPServiceIPAPI
	cfg.AllowInsecureHTTP = false
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPAPI] = newIPAPIService(testClient(server, true), server.URL, false)

	if _, err := m.Lookup("8.8.8.8"); !errors.Is(err, ErrInsecureHTTP) {
		t.Fatalf("expected ErrInsecureHTTP, got %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("legacy ipapi made %d requests without explicit consent", got)
	}
	if err := m.ValidateService(config.GeoIPServiceIPAPI); !errors.Is(err, ErrInsecureHTTP) {
		t.Fatalf("expected validation to require consent, got %v", err)
	}
}

func TestManagerLegacyIPAPIWithConsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"status":"success","query":"8.8.8.8","country":"United States","lat":1,"lon":1}`)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	cfg.Service = config.GeoIPServiceIPAPI
	cfg.AllowInsecureHTTP = true
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPAPI] = newIPAPIService(server.Client(), server.URL, true)

	info, err := m.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if info.Country != "United States" {
		t.Fatalf("unexpected info: %+v", info)
	}
}
