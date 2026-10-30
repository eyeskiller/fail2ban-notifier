package geoip

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func cacheAt(path string, ttl time.Duration) *persistentCache {
	return newPersistentCache(path, ttl, 100, testLogger())
}

func putCacheItem(t *testing.T, cache *persistentCache, item cacheItem) {
	t.Helper()
	err := cache.withLock(func() error {
		doc, err := cache.loadLocked()
		if err != nil {
			return err
		}
		doc.Entries[cacheKey(item.Provider, item.IP)] = item
		return cache.writeLocked(doc)
	})
	if err != nil {
		t.Fatalf("failed to write cache fixture: %v", err)
	}
}

func TestPersistentCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	cache := cacheAt(path, time.Hour)
	info := &Info{IP: "8.8.8.8", Country: "United States", City: "Mountain View"}

	if got := cache.Get("ipwhois", "8.8.8.8", ""); got != nil {
		t.Fatal("expected cache miss")
	}
	cache.Set("ipwhois", "8.8.8.8", "", info)
	got := cache.Get("ipwhois", "8.8.8.8", "")
	if got == nil || got.Country != "United States" {
		t.Fatalf("unexpected cache hit: %+v", got)
	}

	second := cacheAt(path, time.Hour)
	if got := second.Get("ipwhois", "8.8.8.8", ""); got == nil {
		t.Fatal("cache did not persist to a new cache instance")
	}
}

func TestPersistentCacheProviderSpecificKeys(t *testing.T) {
	cache := cacheAt(filepath.Join(t.TempDir(), "geoip.json"), time.Hour)
	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8", Country: "One"})
	cache.Set("ipapi", "8.8.8.8", "", &Info{IP: "8.8.8.8", Country: "Two"})

	if got := cache.Get("ipwhois", "8.8.8.8", ""); got == nil || got.Country != "One" {
		t.Errorf("ipwhois entry = %+v", got)
	}
	if got := cache.Get("ipapi", "8.8.8.8", ""); got == nil || got.Country != "Two" {
		t.Errorf("ipapi entry = %+v", got)
	}
}

func TestPersistentCacheTTLExpirationAndCleanup(t *testing.T) {
	cache := cacheAt(filepath.Join(t.TempDir(), "geoip.json"), time.Second)
	putCacheItem(t, cache, cacheItem{
		IP:       "8.8.8.8",
		Provider: "ipwhois",
		CachedAt: time.Now().Add(-2 * time.Second),
		Info:     &Info{IP: "8.8.8.8", Country: "Old"},
	})
	putCacheItem(t, cache, cacheItem{
		IP:       "1.1.1.1",
		Provider: "ipwhois",
		CachedAt: time.Now().Add(-2 * time.Second),
		Info:     &Info{IP: "1.1.1.1", Country: "Old"},
	})

	if got := cache.Get("ipwhois", "8.8.8.8", ""); got != nil {
		t.Fatal("expected expired entry to be a miss")
	}

	cache.Set("ipwhois", "9.9.9.9", "", &Info{IP: "9.9.9.9", Country: "New"})
	total, valid := cache.Stats()
	if total != 1 || valid != 1 {
		t.Fatalf("expected expired entries to be pruned, got total=%d valid=%d", total, valid)
	}
}

func TestPersistentCacheFingerprintInvalidation(t *testing.T) {
	cache := cacheAt(filepath.Join(t.TempDir(), "geoip.json"), time.Hour)
	cache.Set("geolite2", "8.8.8.8", "db-v1", &Info{IP: "8.8.8.8", City: "Old"})
	if got := cache.Get("geolite2", "8.8.8.8", "db-v2"); got != nil {
		t.Fatal("expected database fingerprint change to invalidate cache")
	}
	if got := cache.Get("geolite2", "8.8.8.8", "db-v1"); got == nil {
		t.Fatal("expected matching fingerprint to hit")
	}
}

func TestPersistentCacheMaxEntries(t *testing.T) {
	cache := newPersistentCache(filepath.Join(t.TempDir(), "geoip.json"), time.Hour, 3, testLogger())
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "4.4.4.4", "5.5.5.5"} {
		cache.Set("ipwhois", ip, "", &Info{IP: ip})
	}
	total, _ := cache.Stats()
	if total != 3 {
		t.Fatalf("expected cache to be capped at 3 entries, got %d", total)
	}
}

func TestPersistentCacheRecoversFromCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := cacheAt(path, time.Hour)
	if got := cache.Get("ipwhois", "8.8.8.8", ""); got != nil {
		t.Fatal("expected corrupted cache to be treated as a miss")
	}
	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8", Country: "Recovered"})
	if got := cache.Get("ipwhois", "8.8.8.8", ""); got == nil || got.Country != "Recovered" {
		t.Fatalf("cache did not recover: %+v", got)
	}
}

func TestPersistentCacheAtomicWriteAndPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "geoip.json")
	cache := cacheAt(path, time.Hour)
	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8"})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != config.FilePermission {
		t.Errorf("cache mode = %o, want %o", info.Mode().Perm(), config.FilePermission)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != config.DirPermission {
		t.Errorf("cache dir mode = %o, want %o", dirInfo.Mode().Perm(), config.DirPermission)
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were left behind: %v", matches)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc cacheDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("cache file is not valid JSON: %v", err)
	}
}

func TestPersistentCacheInvalidPermissionsFailGracefully(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parentFile, "geoip.json")
	cache := cacheAt(path, time.Hour)

	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8"})
	if got := cache.Get("ipwhois", "8.8.8.8", ""); got != nil {
		t.Fatal("expected no cache entry when directory creation fails")
	}
	total, valid := cache.Stats()
	if total != 0 || valid != 0 {
		t.Fatalf("unexpected stats: total=%d valid=%d", total, valid)
	}
}

func TestPersistentCacheRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	path := filepath.Join(dir, "geoip.json")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cache := cacheAt(path, time.Hour)
	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8"})
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("cache followed a symlink and wrote its target (err=%v)", err)
	}
}

func TestPersistentCacheConcurrentAccess(t *testing.T) {
	cache := cacheAt(filepath.Join(t.TempDir(), "geoip.json"), time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := fmt.Sprintf("8.8.8.%d", i+1)
			cache.Set("ipwhois", ip, "", &Info{IP: ip, Country: "Country"})
			_ = cache.Get("ipwhois", ip, "")
		}(i)
	}
	wg.Wait()

	total, _ := cache.Stats()
	if total == 0 {
		t.Fatal("expected concurrent writes to produce entries")
	}
	if data, err := os.ReadFile(cache.path); err != nil || !json.Valid(data) {
		t.Fatalf("cache file is corrupted after concurrent access: %v", err)
	}
}

func TestPersistentCacheClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	cache := cacheAt(path, time.Hour)
	cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8"})
	cache.Clear()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected cache file to be removed, got %v", err)
	}
	if total, _ := cache.Stats(); total != 0 {
		t.Fatalf("expected empty stats after clear, got %d", total)
	}
}

func TestCacheDisabledDoesNotPersist(t *testing.T) {
	cfg := testManagerConfig(t)
	cfg.Cache = false
	path := cfg.CachePath
	m := NewManager(cfg, testLogger())
	defer func() { _ = m.Close() }()

	m.setCached("8.8.8.8", &Info{IP: "8.8.8.8"})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cache file was created while cache was disabled: %v", err)
	}
	stats := m.GetCacheStats()
	if stats["enabled"] != false {
		t.Errorf("cache reported enabled: %+v", stats)
	}
}

func TestPersistentCacheAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip.json")
	runCacheHelper(t, path, "write")

	cache := cacheAt(path, time.Hour)
	if got := cache.Get("ipwhois", "8.8.8.8", ""); got == nil || got.Country != "Cross Process" {
		t.Fatalf("cache written by another process was not visible: %+v", got)
	}

	runCacheHelper(t, path, "read")
}

func runCacheHelper(t *testing.T, path, action string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestCacheHelperProcess")
	cmd.Env = append(os.Environ(),
		"F2B_CACHE_HELPER=1",
		"F2B_CACHE_PATH="+path,
		"F2B_CACHE_ACTION="+action,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cache helper %s failed: %v\n%s", action, err, output)
	}
}

func TestCacheHelperProcess(t *testing.T) {
	if os.Getenv("F2B_CACHE_HELPER") != "1" {
		return
	}
	cache := cacheAt(os.Getenv("F2B_CACHE_PATH"), time.Hour)
	switch os.Getenv("F2B_CACHE_ACTION") {
	case "write":
		cache.Set("ipwhois", "8.8.8.8", "", &Info{IP: "8.8.8.8", Country: "Cross Process"})
	case "read":
		if cache.Get("ipwhois", "8.8.8.8", "") == nil {
			t.Fatal("expected cache entry written by parent process")
		}
	}
}
