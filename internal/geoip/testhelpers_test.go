package geoip

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func testManagerConfig(t *testing.T) config.GeoIPConfig {
	t.Helper()
	return config.GeoIPConfig{
		Enabled:    true,
		Service:    config.GeoIPServiceIPWhois,
		Cache:      true,
		TTL:        3600,
		CachePath:  filepath.Join(t.TempDir(), "geoip.json"),
		CityDBPath: config.DefaultGeoLite2CityDB,
		ASNDBPath:  config.DefaultGeoLite2ASNDB,
	}
}

func testClient(server *httptest.Server, requireHTTPS bool) *http.Client {
	client := server.Client()
	client.Timeout = 2 * time.Second
	client.CheckRedirect = newHTTPClient(0, requireHTTPS).CheckRedirect
	return client
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func writeMMDB(t *testing.T, filename, databaseType string, networks map[string]map[string]any) string {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType: databaseType,
		RecordSize:   24,
		IPVersion:    6,
	})
	if err != nil {
		t.Fatalf("failed to create MMDB writer: %v", err)
	}
	for cidr, record := range networks {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("invalid test CIDR %s: %v", cidr, err)
		}
		if err := tree.Insert(network, toMMDBType(record)); err != nil {
			t.Fatalf("failed to insert %s: %v", cidr, err)
		}
	}
	path := filepath.Join(t.TempDir(), filename)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create MMDB file: %v", err)
	}
	if _, err := tree.WriteTo(file); err != nil {
		_ = file.Close()
		t.Fatalf("failed to write MMDB: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("failed to close MMDB: %v", err)
	}
	return path
}

func toMMDBType(value any) mmdbtype.DataType {
	switch v := value.(type) {
	case string:
		return mmdbtype.String(v)
	case uint32:
		return mmdbtype.Uint32(v)
	case uint64:
		return mmdbtype.Uint64(v)
	case int:
		return mmdbtype.Int32(v)
	case float64:
		return mmdbtype.Float64(v)
	case map[string]any:
		result := mmdbtype.Map{}
		for key, item := range v {
			result[mmdbtype.String(key)] = toMMDBType(item)
		}
		return result
	case []any:
		result := mmdbtype.Slice{}
		for _, item := range v {
			result = append(result, toMMDBType(item))
		}
		return result
	default:
		panic("unsupported MMDB test value")
	}
}

func cityFixture(t *testing.T) string {
	t.Helper()
	return writeMMDB(t, "GeoLite2-City.mmdb", "GeoLite2-City", map[string]map[string]any{
		"8.8.8.0/24": {
			"country": map[string]any{
				"iso_code": "US",
				"names":    map[string]any{"en": "United States"},
			},
			"subdivisions": []any{
				map[string]any{"names": map[string]any{"en": "California"}},
			},
			"city": map[string]any{"names": map[string]any{"en": "Mountain View"}},
			"location": map[string]any{
				"latitude":  37.4056,
				"longitude": -122.0775,
				"time_zone": "America/Los_Angeles",
			},
		},
		"2001:4860:4860::/48": {
			"country": map[string]any{
				"iso_code": "GB",
				"names":    map[string]any{"en": "United Kingdom"},
			},
			"subdivisions": []any{
				map[string]any{"names": map[string]any{"en": "England"}},
			},
			"city": map[string]any{"names": map[string]any{"en": "London"}},
			"location": map[string]any{
				"latitude":  51.5074,
				"longitude": -0.1278,
				"time_zone": "Europe/London",
			},
		},
	})
}

func asnFixture(t *testing.T) string {
	t.Helper()
	return writeMMDB(t, "GeoLite2-ASN.mmdb", "GeoLite2-ASN", map[string]map[string]any{
		"8.8.8.0/24": {
			"autonomous_system_number":       uint32(15169),
			"autonomous_system_organization": "Google LLC",
		},
		"2001:4860:4860::/48": {
			"autonomous_system_number":       uint32(15169),
			"autonomous_system_organization": "Google LLC",
		},
	})
}
