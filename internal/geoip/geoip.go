package geoip

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config" //nolint:depguard
)

// Info represents geolocation information for an IP address.
type Info struct {
	IP       string  `json:"ip"`
	Country  string  `json:"country"`
	Region   string  `json:"region"`
	City     string  `json:"city"`
	ISP      string  `json:"isp"`
	ASN      string  `json:"asn,omitempty"`
	Timezone string  `json:"timezone"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

// Service represents a GeoIP service provider.
type Service interface {
	Lookup(ip string) (*Info, error)
	GetName() string
}

type serviceValidator interface {
	Validate() error
}

type cacheFingerprinter interface {
	CacheFingerprint() string
}

type serviceCloser interface {
	Close() error
}

// Manager manages GeoIP lookups with persistent caching and rate-limit state.
type Manager struct {
	config      config.GeoIPConfig
	logger      *log.Logger
	services    map[string]Service
	cache       *persistentCache
	rateLimiter *rateLimiter
}

// NewManager creates a new GeoIP manager.
func NewManager(cfg config.GeoIPConfig, logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.New(os.Stderr, "[geoip] ", log.LstdFlags)
	}
	if cfg.Service == "" {
		cfg.Service = config.DefaultGeoIPService
	}
	if cfg.CachePath == "" {
		cfg.CachePath = config.DefaultGeoIPCachePath
	}
	if cfg.CityDBPath == "" {
		cfg.CityDBPath = config.DefaultGeoLite2CityDB
	}
	if cfg.ASNDBPath == "" {
		cfg.ASNDBPath = config.DefaultGeoLite2ASNDB
	}
	if cfg.TTL <= 0 {
		cfg.TTL = config.DefaultGeoIPTTL
	}

	manager := &Manager{
		config:      cfg,
		logger:      logger,
		services:    make(map[string]Service),
		rateLimiter: newRateLimiter(cfg.CachePath, logger),
	}

	manager.services[config.GeoIPServiceIPWhois] = newIPWhoisService(nil, "")
	manager.services[config.GeoIPServiceGeoLite2] = newGeoLite2Service(cfg.CityDBPath, cfg.ASNDBPath, logger)
	manager.services[config.GeoIPServiceIPAPI] = newIPAPIService(nil, "", cfg.AllowInsecureHTTP)
	manager.services[config.GeoIPServiceIPGeolocation] = newIPGeolocationService(nil, "", cfg.APIKey)

	if cfg.Cache {
		manager.cache = newPersistentCache(
			cfg.CachePath,
			time.Duration(cfg.TTL)*time.Second,
			config.DefaultGeoIPCacheMaxSize,
			logger,
		)
	}

	manager.warnAboutProviderConfiguration()

	return manager
}

func (m *Manager) warnAboutProviderConfiguration() {
	if !m.config.Enabled || m.config.Service != config.GeoIPServiceIPAPI {
		return
	}
	if m.config.AllowInsecureHTTP {
		m.logger.Printf("Warning: GeoIP provider ipapi uses unencrypted HTTP. Queried IP addresses and responses can be observed or modified. Consider ipwhois or geolite2 instead.")
		return
	}
	m.logger.Printf("Warning: GeoIP provider ipapi is HTTP-only and allow_insecure_http is false; GeoIP enrichment will be skipped. Set geoip.allow_insecure_http=true to accept the risk, or migrate to ipwhois/geolite2.")
}

// Close releases local database resources.
func (m *Manager) Close() error {
	var firstErr error
	for _, service := range m.services {
		closer, ok := service.(serviceCloser)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) serviceID() string {
	if m.config.Service == "" {
		return config.DefaultGeoIPService
	}
	return m.config.Service
}

func (m *Manager) cacheFingerprint() string {
	service, ok := m.services[m.serviceID()]
	if !ok {
		return ""
	}
	if fingerprinter, ok := service.(cacheFingerprinter); ok {
		return fingerprinter.CacheFingerprint()
	}
	return ""
}

// Lookup performs a GeoIP lookup for the given IP address.
func (m *Manager) Lookup(ip string) (*Info, error) {
	if !m.config.Enabled {
		return &Info{IP: ip}, nil
	}

	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidIP, ip)
	}

	if !isPublicIP(normalized) {
		return &Info{
			IP:      normalized,
			Country: "Private Network",
			Region:  "Local",
			City:    "Internal",
			ISP:     "Private",
		}, nil
	}

	serviceID := m.serviceID()
	service, ok := m.services[serviceID]
	if !ok {
		return nil, fmt.Errorf("%w: unknown provider %q", ErrProviderUnavailable, serviceID)
	}

	if serviceID == config.GeoIPServiceIPAPI && !m.config.AllowInsecureHTTP {
		return nil, ErrInsecureHTTP
	}

	fingerprint := m.cacheFingerprint()
	if m.cache != nil {
		if info := m.cache.Get(serviceID, normalized, fingerprint); info != nil {
			return info, nil
		}
	}

	if until := m.rateLimiter.Until(serviceID); until.After(time.Now()) {
		return nil, fmt.Errorf("%w until %s", ErrRateLimited, until.UTC().Format(time.RFC3339))
	}

	info, err := service.Lookup(normalized)
	if err != nil {
		if cooldown, limited := rateLimitCooldown(err); limited {
			until := time.Now().Add(cooldown)
			m.rateLimiter.Set(serviceID, until)
			m.logger.Printf("Warning: GeoIP provider %s is rate limited; suppressing further lookups until %s", serviceID, until.UTC().Format(time.RFC3339))
			return nil, fmt.Errorf("%w: %v", ErrRateLimited, err)
		}
		m.logger.Printf("GeoIP lookup failed for %s using %s: %v", normalized, serviceID, err)
		return nil, err
	}

	if info == nil {
		return nil, &ProviderError{Provider: serviceID, Message: "provider returned no data"}
	}
	if !validCoordinates(info.Lat, info.Lon) {
		return nil, &ProviderError{Provider: serviceID, Message: "provider returned invalid coordinates"}
	}
	info.IP = normalized

	if m.cache != nil {
		m.cache.Set(serviceID, normalized, fingerprint, info)
	}

	return info, nil
}

// getCached retrieves cached GeoIP information for the configured provider.
func (m *Manager) getCached(ip string) *Info {
	if m.cache == nil {
		return nil
	}
	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil
	}
	return m.cache.Get(m.serviceID(), normalized, m.cacheFingerprint())
}

// setCached stores GeoIP information in the persistent cache.
func (m *Manager) setCached(ip string, info *Info) {
	if m.cache == nil || info == nil {
		return
	}
	normalized, ok := normalizeIP(ip)
	if !ok {
		return
	}
	m.cache.Set(m.serviceID(), normalized, m.cacheFingerprint(), info)
}

// ClearCache clears the persistent GeoIP cache.
func (m *Manager) ClearCache() {
	if m.cache != nil {
		m.cache.Clear()
	}
}

// GetCacheStats returns cache statistics.
func (m *Manager) GetCacheStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":       m.cache != nil,
		"persistent":    m.cache != nil,
		"entries":       0,
		"total_entries": 0,
		"ttl_seconds":   m.config.TTL,
		"service":       m.serviceID(),
		"cache_path":    m.config.CachePath,
	}

	if m.cache != nil {
		total, valid := m.cache.Stats()
		stats["entries"] = valid
		stats["total_entries"] = total
	}

	return stats
}

// BatchLookup performs multiple GeoIP lookups concurrently. Failed lookups are
// omitted from the result map rather than being represented as empty successes.
func (m *Manager) BatchLookup(ips []string) map[string]*Info {
	results := make(map[string]*Info)
	var mu sync.Mutex
	var wg sync.WaitGroup

	seen := make(map[string]struct{}, len(ips))
	unique := make([]string, 0, len(ips))
	for _, ip := range ips {
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		unique = append(unique, ip)
	}

	semaphore := make(chan struct{}, 5)
	for _, ip := range unique {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info, err := m.Lookup(ip)
			if err != nil {
				return
			}
			mu.Lock()
			results[ip] = info
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}

// ValidateService checks whether a provider is registered and usable without
// consuming API quota or making network requests.
func (m *Manager) ValidateService(serviceName string) error {
	service, ok := m.services[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}
	if validator, ok := service.(serviceValidator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("service validation failed: %w", err)
		}
	}
	return nil
}

// GetAvailableServices returns a sorted list of registered GeoIP services.
func (m *Manager) GetAvailableServices() []string {
	services := make([]string, 0, len(m.services))
	for name := range m.services {
		services = append(services, name)
	}
	sort.Strings(services)
	return services
}
