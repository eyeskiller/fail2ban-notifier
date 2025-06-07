package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config" //nolint:depguard
)

// Info represents geolocation information for an IP address
type Info struct {
	IP       string  `json:"ip"`
	Country  string  `json:"country"`
	Region   string  `json:"region"`
	City     string  `json:"city"`
	ISP      string  `json:"isp"`
	Timezone string  `json:"timezone"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

// Service represents a GeoIP service provider
type Service interface {
	Lookup(ip string) (*Info, error)
	GetName() string
}

// Manager manages GeoIP lookups with caching
type Manager struct {
	config   config.GeoIPConfig
	cache    map[string]*cacheEntry
	cacheMu  sync.RWMutex
	logger   *log.Logger
	services map[string]Service
}

type cacheEntry struct {
	info      *Info
	timestamp time.Time
}

// NewManager creates a new GeoIP manager
func NewManager(cfg config.GeoIPConfig, logger *log.Logger) *Manager {
	if logger == nil {
		logger = log.New(os.Stdout, "[geoip] ", log.LstdFlags)
	}

	manager := &Manager{
		config:   cfg,
		cache:    make(map[string]*cacheEntry),
		logger:   logger,
		services: make(map[string]Service),
	}

	// Register available services
	manager.services["ipapi"] = &IPAPIService{client: &http.Client{Timeout: 10 * time.Second}}
	if cfg.APIKey != "" {
		manager.services["ipgeolocation"] = &IPGeolocationService{
			apiKey: cfg.APIKey,
			client: &http.Client{Timeout: 10 * time.Second},
		}
	}

	return manager
}

// Lookup performs a GeoIP lookup for the given IP address
func (m *Manager) Lookup(ip string) (*Info, error) {
	if !m.config.Enabled {
		return &Info{IP: ip}, nil
	}

	// Validate IP address
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Skip private/local IP addresses
	if isPrivateIP(ip) {
		return &Info{
			IP:      ip,
			Country: "Private Network",
			Region:  "Local",
			City:    "Internal",
			ISP:     "Private",
		}, nil
	}

	// Check cache first
	if m.config.Cache {
		if info := m.getCached(ip); info != nil {
			return info, nil
		}
	}

	// Get service
	service, ok := m.services[m.config.Service]
	if !ok {
		return nil, fmt.Errorf("unknown GeoIP service: %s", m.config.Service)
	}

	// Perform lookup
	info, err := service.Lookup(ip)
	if err != nil {
		m.logger.Printf("GeoIP lookup failed for %s: %v", ip, err)
		return &Info{IP: ip}, nil // Return empty info instead of error
	}

	// Cache the result
	if m.config.Cache {
		m.setCached(ip, info)
	}

	return info, nil
}

// getCached retrieves cached GeoIP information
func (m *Manager) getCached(ip string) *Info {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	entry, ok := m.cache[ip]
	if !ok {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(entry.timestamp) > time.Duration(m.config.TTL)*time.Second {
		return nil
	}

	return entry.info
}

// setCached stores GeoIP information in cache
func (m *Manager) setCached(ip string, info *Info) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	m.cache[ip] = &cacheEntry{
		info:      info,
		timestamp: time.Now(),
	}
}

// ClearCache clears the GeoIP cache
func (m *Manager) ClearCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache = make(map[string]*cacheEntry)
}

// GetCacheStats returns cache statistics
func (m *Manager) GetCacheStats() map[string]interface{} {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	stats := map[string]interface{}{
		"enabled":     m.config.Cache,
		"entries":     len(m.cache),
		"ttl_seconds": m.config.TTL,
		"service":     m.config.Service,
	}

	return stats
}

// isPrivateIP checks if an IP address is private/local
func isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for private IPv4 ranges
	private := []string{
		"127.0.0.0/8",    // localhost
		"10.0.0.0/8",     // private
		"172.16.0.0/12",  // private
		"192.168.0.0/16", // private
		"169.254.0.0/16", // link-local
	}

	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsedIP) {
			return true
		}
	}

	// Check for IPv6 private ranges
	if parsedIP.To4() == nil {
		// IPv6 loopback
		if parsedIP.IsLoopback() {
			return true
		}
		// IPv6 link-local
		if parsedIP.IsLinkLocalUnicast() {
			return true
		}
		// IPv6 unique local
		if len(parsedIP) >= 1 && (parsedIP[0]&0xfe) == 0xfc {
			return true
		}
	}

	return false
}

// IPAPIService implements the ip-api.com service
type IPAPIService struct {
	client *http.Client
}

func (s *IPAPIService) GetName() string {
	return "ip-api.com"
}

func (s *IPAPIService) Lookup(ip string) (*Info, error) {
	url := fmt.Sprintf("https://ip-api.com/json/%s?fields=status,country,regionName,city,isp,timezone,lat,lon", ip)

	// Create a new request with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeError := resp.Body.Close(); closeError != nil {
			err = fmt.Errorf("error closing response body: %v", closeError)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status     string  `json:"status"`
		Country    string  `json:"country"`
		RegionName string  `json:"regionName"`
		City       string  `json:"city"`
		ISP        string  `json:"isp"`
		Timezone   string  `json:"timezone"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("API returned status: %s", result.Status)
	}

	return &Info{
		IP:       ip,
		Country:  result.Country,
		Region:   result.RegionName,
		City:     result.City,
		ISP:      result.ISP,
		Timezone: result.Timezone,
		Lat:      result.Lat,
		Lon:      result.Lon,
	}, nil
}

// IPGeolocationService implements the ipgeolocation.io service
type IPGeolocationService struct {
	apiKey string
	client *http.Client
}

func (s *IPGeolocationService) GetName() string {
	return "ipgeolocation.io"
}

func (s *IPGeolocationService) Lookup(ip string) (*Info, error) {
	url := fmt.Sprintf("https://api.ipgeolocation.io/ipgeo?apiKey=%s&ip=%s", s.apiKey, ip)

	// Create a new request with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if clientError := resp.Body.Close(); clientError != nil {
			err = fmt.Errorf("error closing response body: %v", clientError)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		IP          string  `json:"ip"`
		CountryName string  `json:"country_name"`
		StateProv   string  `json:"state_prov"`
		City        string  `json:"city"`
		ISP         string  `json:"isp"`
		TimeZone    string  `json:"time_zone"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Message     string  `json:"message"` // Error message if any
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check for API errors
	if result.Message != "" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return &Info{
		IP:       ip,
		Country:  result.CountryName,
		Region:   result.StateProv,
		City:     result.City,
		ISP:      result.ISP,
		Timezone: result.TimeZone,
		Lat:      result.Latitude,
		Lon:      result.Longitude,
	}, nil
}

// BatchLookup performs multiple GeoIP lookups concurrently
func (m *Manager) BatchLookup(ips []string) map[string]*Info {
	results := make(map[string]*Info)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent requests
	semaphore := make(chan struct{}, 5)

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			info, err := m.Lookup(ip)
			mu.Lock()
			if err != nil {
				// Store empty info for failed lookups
				results[ip] = &Info{IP: ip}
			} else {
				results[ip] = info
			}
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}

// ValidateService checks if a GeoIP service is available and working
func (m *Manager) ValidateService(serviceName string) error {
	service, ok := m.services[serviceName]
	if !ok {
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	// Test with a known public IP (Google DNS)
	testIP := "8.8.8.8"
	_, err := service.Lookup(testIP)
	if err != nil {
		return fmt.Errorf("service validation failed: %w", err)
	}

	return nil
}

// GetAvailableServices returns a list of available GeoIP services
func (m *Manager) GetAvailableServices() []string {
	var services []string
	for name := range m.services {
		services = append(services, name)
	}
	return services
}
