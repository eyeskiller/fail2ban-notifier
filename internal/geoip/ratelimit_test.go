package geoip

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func TestRateLimiterPersistsPerProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	limiter := newRateLimiter(path, testLogger())
	until := time.Now().Add(time.Hour)
	limiter.Set(config.GeoIPServiceIPWhois, until)

	second := newRateLimiter(path, testLogger())
	got := second.Until(config.GeoIPServiceIPWhois)
	if got.Before(until.Add(-time.Second)) {
		t.Fatalf("cooldown was not persisted: got %v want %v", got, until)
	}
	if other := second.Until(config.GeoIPServiceIPAPI); !other.IsZero() {
		t.Fatalf("cooldown leaked to another provider: %v", other)
	}
}

func TestRateLimiterClearAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	limiter := newRateLimiter(path, testLogger())
	limiter.Set("ipwhois", time.Now().Add(time.Hour))
	limiter.Clear()
	if until := limiter.Until("ipwhois"); !until.IsZero() {
		t.Fatalf("expected cleared cooldown, got %v", until)
	}

	limitPath := filepath.Join(filepath.Dir(path), "geoip-ratelimit.json")
	if err := os.MkdirAll(filepath.Dir(limitPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(limitPath, []byte("{bad json"), 0600); err != nil {
		t.Fatal(err)
	}
	if until := limiter.Until("ipwhois"); !until.IsZero() {
		t.Fatalf("expected corrupt state to be ignored, got %v", until)
	}
	limiter.Set("ipwhois", time.Now().Add(time.Hour))
	if until := limiter.Until("ipwhois"); until.IsZero() {
		t.Fatal("limiter did not recover from corrupt state")
	}
}

func TestManagerRateLimitIsPersistedAcrossManagers(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	cfg.Cache = false
	m1 := NewManager(cfg, testLogger())
	m1.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)
	if _, err := m1.Lookup("8.8.8.8"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	_ = m1.Close()

	m2 := NewManager(cfg, testLogger())
	m2.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)
	if _, err := m2.Lookup("8.8.8.8"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected persisted ErrRateLimited, got %v", err)
	}
	_ = m2.Close()

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("provider was called %d times during cooldown, want 1", got)
	}
}

func TestManagerRateLimitFallbackWithoutHeader(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	cfg.Cache = false
	m := NewManager(cfg, testLogger())
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)
	if _, err := m.Lookup("8.8.8.8"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	defer func() { _ = m.Close() }()

	until := m.rateLimiter.Until(config.GeoIPServiceIPWhois)
	remaining := time.Until(until)
	if remaining < 14*time.Minute || remaining > 16*time.Minute {
		t.Fatalf("fallback cooldown = %v, want about 15m", remaining)
	}
}

func TestRateLimitDoesNotBlockCacheHits(t *testing.T) {
	var requests int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	cfg := testManagerConfig(t)
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()
	m.services[config.GeoIPServiceIPWhois] = newIPWhoisService(testClient(server, true), server.URL)

	m.setCached("8.8.8.8", &Info{IP: "8.8.8.8", Country: "Cached"})
	m.rateLimiter.Set(config.GeoIPServiceIPWhois, time.Now().Add(time.Hour))

	info, err := m.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("cache hit should bypass rate limit: %v", err)
	}
	if info.Country != "Cached" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("provider was called %d times for a cache hit", got)
	}
}

func TestRateLimitCooldownBounds(t *testing.T) {
	cooldown, ok := rateLimitCooldown(&HTTPStatusError{StatusCode: http.StatusTooManyRequests, RetryAfter: 48 * time.Hour})
	if !ok || cooldown != rateLimitMaximumCooldown {
		t.Fatalf("cooldown = %v, ok=%t; want %v", cooldown, ok, rateLimitMaximumCooldown)
	}
	if _, ok := rateLimitCooldown(&HTTPStatusError{StatusCode: http.StatusForbidden}); ok {
		t.Fatal("403 should not be treated as a rate limit")
	}
	if _, ok := rateLimitCooldown(errors.New("network")); ok {
		t.Fatal("generic errors should not be treated as rate limits")
	}
}
