package geoip

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

const (
	rateLimitVersion         = 1
	rateLimitFallback        = 15 * time.Minute
	rateLimitMaximumCooldown = 24 * time.Hour
	maxRateLimitFileSize     = 1 << 20 // 1 MiB
)

type rateLimitDocument struct {
	Version   int                  `json:"version"`
	Providers map[string]time.Time `json:"providers"`
}

type rateLimiter struct {
	path   string
	logger *log.Logger
}

func newRateLimiter(cachePath string, logger *log.Logger) *rateLimiter {
	if cachePath == "" {
		cachePath = config.DefaultGeoIPCachePath
	}
	return &rateLimiter{
		path:   filepath.Join(filepath.Dir(cachePath), "geoip-ratelimit.json"),
		logger: logger,
	}
}

func (r *rateLimiter) Until(provider string) time.Time {
	if r == nil || r.path == "" {
		return time.Time{}
	}

	var until time.Time
	err := r.withLock(func() error {
		doc, err := r.loadLocked()
		if err != nil {
			return err
		}
		until = doc.Providers[provider]
		return nil
	})
	if err != nil && r.logger != nil {
		r.logger.Printf("Warning: GeoIP rate-limit state read failed: %v", err)
	}
	return until
}

func (r *rateLimiter) Set(provider string, until time.Time) {
	if r == nil || r.path == "" {
		return
	}

	err := r.withLock(func() error {
		doc, err := r.loadLocked()
		if err != nil {
			return err
		}
		now := time.Now()
		for name, when := range doc.Providers {
			if !when.After(now) {
				delete(doc.Providers, name)
			}
		}
		if existing := doc.Providers[provider]; existing.After(until) {
			until = existing
		}
		doc.Providers[provider] = until.UTC()
		return r.writeLocked(doc)
	})
	if err != nil && r.logger != nil {
		r.logger.Printf("Warning: GeoIP rate-limit state write failed: %v", err)
	}
}

func (r *rateLimiter) Clear() {
	if r == nil || r.path == "" {
		return
	}
	err := r.withLock(func() error {
		if err := os.Remove(r.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	})
	if err != nil && r.logger != nil {
		r.logger.Printf("Warning: GeoIP rate-limit state clear failed: %v", err)
	}
}

func (r *rateLimiter) withLock(fn func() error) error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, config.DirPermission); err != nil {
		return fmt.Errorf("cannot create rate-limit directory: %w", err)
	}
	if info, err := os.Lstat(dir); err != nil {
		return fmt.Errorf("cannot inspect rate-limit directory: %w", err)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("rate-limit directory is not a real directory: %s", dir)
	}

	lock, err := acquireFileLock(r.path + ".lock")
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.Release()
	}()

	return fn()
}

func (r *rateLimiter) loadLocked() (*rateLimitDocument, error) {
	doc := &rateLimitDocument{
		Version:   rateLimitVersion,
		Providers: make(map[string]time.Time),
	}

	file, err := openNoFollow(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open rate-limit file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(file, maxRateLimitFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read rate-limit file: %w", err)
	}
	if len(data) > maxRateLimitFileSize {
		if r.logger != nil {
			r.logger.Printf("Warning: GeoIP rate-limit state exceeds %d bytes and will be recreated", maxRateLimitFileSize)
		}
		return &rateLimitDocument{Version: rateLimitVersion, Providers: make(map[string]time.Time)}, nil
	}
	if err := json.Unmarshal(data, doc); err != nil {
		if r.logger != nil {
			r.logger.Printf("Warning: GeoIP rate-limit state is corrupted and will be recreated: %v", err)
		}
		return &rateLimitDocument{Version: rateLimitVersion, Providers: make(map[string]time.Time)}, nil
	}
	if doc.Version != rateLimitVersion {
		return &rateLimitDocument{Version: rateLimitVersion, Providers: make(map[string]time.Time)}, nil
	}
	if doc.Providers == nil {
		doc.Providers = make(map[string]time.Time)
	}
	return doc, nil
}

func (r *rateLimiter) writeLocked(doc *rateLimitDocument) error {
	doc.Version = rateLimitVersion
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal rate-limit state: %w", err)
	}

	dir := filepath.Dir(r.path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(r.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("cannot create temporary rate-limit file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(config.FilePermission); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot set rate-limit permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot write rate-limit state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("cannot sync rate-limit state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cannot close rate-limit state: %w", err)
	}
	if err := os.Rename(tempPath, r.path); err != nil {
		return fmt.Errorf("cannot replace rate-limit state atomically: %w", err)
	}
	return nil
}

func rateLimitCooldown(err error) (time.Duration, bool) {
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || !statusErr.IsRateLimit() {
		return 0, false
	}

	cooldown := statusErr.RetryAfter
	if cooldown <= 0 {
		cooldown = rateLimitFallback
	}
	if cooldown > rateLimitMaximumCooldown {
		cooldown = rateLimitMaximumCooldown
	}
	return cooldown, true
}
