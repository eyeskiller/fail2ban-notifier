package geoip

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

const (
	cacheVersion     = 1
	maxCacheFileSize = 16 << 20 // 16 MiB
)

type cacheDocument struct {
	Version int                  `json:"version"`
	Entries map[string]cacheItem `json:"entries"`
}

type cacheItem struct {
	IP          string    `json:"ip"`
	Provider    string    `json:"provider"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	CachedAt    time.Time `json:"cached_at"`
	Info        *Info     `json:"info"`
}

type persistentCache struct {
	path       string
	ttl        time.Duration
	maxEntries int
	logger     *log.Logger
}

func newPersistentCache(path string, ttl time.Duration, maxEntries int, logger *log.Logger) *persistentCache {
	if ttl <= 0 {
		ttl = time.Duration(config.DefaultGeoIPTTL) * time.Second
	}
	if maxEntries <= 0 {
		maxEntries = config.DefaultGeoIPCacheMaxSize
	}
	return &persistentCache{
		path:       path,
		ttl:        ttl,
		maxEntries: maxEntries,
		logger:     logger,
	}
}

func cacheKey(provider, ip string) string {
	return provider + "|" + ip
}

func (c *persistentCache) Get(provider, ip, fingerprint string) *Info {
	if c == nil || c.path == "" {
		return nil
	}

	var result *Info
	err := c.withLock(func() error {
		doc, err := c.loadLocked()
		if err != nil {
			return err
		}
		item, ok := doc.Entries[cacheKey(provider, ip)]
		if !ok {
			return nil
		}
		if !c.validItem(item, provider, ip, fingerprint, time.Now()) {
			return nil
		}
		result = cloneInfo(item.Info)
		return nil
	})
	if err != nil && c.logger != nil {
		c.logger.Printf("Warning: GeoIP cache read failed: %v", err)
	}
	return result
}

func (c *persistentCache) Set(provider, ip, fingerprint string, info *Info) {
	if c == nil || c.path == "" || info == nil {
		return
	}

	err := c.withLock(func() error {
		doc, err := c.loadLocked()
		if err != nil {
			return err
		}

		now := time.Now()
		c.pruneLocked(doc, now)
		doc.Entries[cacheKey(provider, ip)] = cacheItem{
			IP:          ip,
			Provider:    provider,
			Fingerprint: fingerprint,
			CachedAt:    now,
			Info:        cloneInfo(info),
		}
		c.enforceSizeLocked(doc)
		return c.writeLocked(doc)
	})
	if err != nil && c.logger != nil {
		c.logger.Printf("Warning: GeoIP cache write failed: %v", err)
	}
}

func (c *persistentCache) Clear() {
	if c == nil || c.path == "" {
		return
	}

	err := c.withLock(func() error {
		if err := os.Remove(c.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	})
	if err != nil && c.logger != nil {
		c.logger.Printf("Warning: GeoIP cache clear failed: %v", err)
	}
}

func (c *persistentCache) Stats() (total, valid int) {
	if c == nil || c.path == "" {
		return 0, 0
	}

	err := c.withLock(func() error {
		doc, loadErr := c.loadLocked()
		if loadErr != nil {
			return loadErr
		}
		now := time.Now()
		total = len(doc.Entries)
		for _, item := range doc.Entries {
			if c.validItem(item, item.Provider, item.IP, item.Fingerprint, now) {
				valid++
			}
		}
		return nil
	})
	if err != nil && c.logger != nil {
		c.logger.Printf("Warning: GeoIP cache stats failed: %v", err)
	}
	return total, valid
}

func (c *persistentCache) withLock(fn func() error) error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, config.DirPermission); err != nil {
		return fmt.Errorf("cannot create cache directory: %w", err)
	}
	if info, err := os.Lstat(dir); err != nil {
		return fmt.Errorf("cannot inspect cache directory: %w", err)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("cache directory is not a real directory: %s", dir)
	}

	lock, err := acquireFileLock(c.path + ".lock")
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.Release()
	}()

	return fn()
}

func (c *persistentCache) loadLocked() (*cacheDocument, error) {
	doc := &cacheDocument{
		Version: cacheVersion,
		Entries: make(map[string]cacheItem),
	}

	file, err := openNoFollow(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open cache file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(file, maxCacheFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read cache file: %w", err)
	}
	if len(data) > maxCacheFileSize {
		if c.logger != nil {
			c.logger.Printf("Warning: GeoIP cache exceeds %d bytes and will be recreated", maxCacheFileSize)
		}
		return &cacheDocument{Version: cacheVersion, Entries: make(map[string]cacheItem)}, nil
	}
	if err := json.Unmarshal(data, doc); err != nil {
		if c.logger != nil {
			c.logger.Printf("Warning: GeoIP cache is corrupted and will be recreated: %v", err)
		}
		return &cacheDocument{Version: cacheVersion, Entries: make(map[string]cacheItem)}, nil
	}
	if doc.Version != cacheVersion {
		if c.logger != nil {
			c.logger.Printf("Warning: unsupported GeoIP cache version %d; recreating cache", doc.Version)
		}
		return &cacheDocument{Version: cacheVersion, Entries: make(map[string]cacheItem)}, nil
	}
	if doc.Entries == nil {
		doc.Entries = make(map[string]cacheItem)
	}

	for key, item := range doc.Entries {
		if item.Info == nil || item.IP == "" || item.Provider == "" {
			delete(doc.Entries, key)
			continue
		}
		if normalized, ok := normalizeIP(item.IP); !ok || normalized != item.IP {
			delete(doc.Entries, key)
		}
	}

	return doc, nil
}

func (c *persistentCache) validItem(item cacheItem, provider, ip, fingerprint string, now time.Time) bool {
	if item.Info == nil {
		return false
	}
	if item.Provider != provider || item.IP != ip {
		return false
	}
	if item.Fingerprint != fingerprint {
		return false
	}
	if item.CachedAt.IsZero() {
		return false
	}
	return now.Sub(item.CachedAt) <= c.ttl
}

func (c *persistentCache) pruneLocked(doc *cacheDocument, now time.Time) {
	for key, item := range doc.Entries {
		if !c.validItem(item, item.Provider, item.IP, item.Fingerprint, now) {
			delete(doc.Entries, key)
		}
	}
}

func (c *persistentCache) enforceSizeLocked(doc *cacheDocument) {
	if len(doc.Entries) <= c.maxEntries {
		return
	}

	type keyedTime struct {
		key string
		at  time.Time
	}
	keys := make([]keyedTime, 0, len(doc.Entries))
	for key, item := range doc.Entries {
		keys = append(keys, keyedTime{key: key, at: item.CachedAt})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].at.Before(keys[j].at)
	})

	for i := 0; i < len(keys)-c.maxEntries; i++ {
		delete(doc.Entries, keys[i].key)
	}
}

func (c *persistentCache) writeLocked(doc *cacheDocument) error {
	doc.Version = cacheVersion
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal cache: %w", err)
	}

	dir := filepath.Dir(c.path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(c.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("cannot create temporary cache file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(config.FilePermission); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot set cache permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot write cache: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot sync cache: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cannot close cache: %w", err)
	}
	if err := os.Rename(tempPath, c.path); err != nil {
		return fmt.Errorf("cannot replace cache atomically: %w", err)
	}
	return nil
}

func cloneInfo(info *Info) *Info {
	if info == nil {
		return nil
	}
	copy := *info
	return &copy
}
