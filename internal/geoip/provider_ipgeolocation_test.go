package geoip

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIPGeolocationLookup(t *testing.T) {
	const apiKey = "super-secret-key"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/ipgeo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("apiKey") != apiKey {
			t.Errorf("api key was not sent as a query parameter")
		}
		if r.URL.Query().Get("ip") != "8.8.8.8" {
			t.Errorf("unexpected IP query: %s", r.URL.Query().Get("ip"))
		}
		// Shape of the v3 free-plan response (trimmed).
		writeJSON(w, `{
			"ip":"8.8.8.8",
			"location":{"country_name":"United States","state_prov":"California","city":"Mountain View",
				"latitude":"37.42240","longitude":"-122.08421"},
			"asn":{"as_number":"AS15169","organization":"Google LLC","country":"US"},
			"time_zone":{"name":"America/Los_Angeles","offset":-8,"offset_with_dst":-7}
		}`)
	}))
	defer server.Close()

	service := newIPGeolocationService(testClient(server, true), server.URL, apiKey)
	info, err := service.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if info.Country != "United States" || info.Region != "California" || info.City != "Mountain View" {
		t.Errorf("unexpected mapping: %+v", info)
	}
	if info.ISP != "Google LLC" || info.ASN != "AS15169" || info.Timezone != "America/Los_Angeles" {
		t.Errorf("unexpected ISP/ASN/timezone mapping: %+v", info)
	}
	if info.Lat != 37.4224 || info.Lon != -122.08421 {
		t.Errorf("unexpected coordinates: %v, %v", info.Lat, info.Lon)
	}
}

func TestIPGeolocationInvalidCoordinates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"ip":"8.8.8.8","location":{"latitude":"not-a-number","longitude":"1"}}`)
	}))
	defer server.Close()

	if _, err := newIPGeolocationService(testClient(server, true), server.URL, "key").Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected invalid coordinates to fail")
	}
}

func TestIPGeolocationRateLimitIsDetected(t *testing.T) {
	const apiKey = "super-secret-key"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := newIPGeolocationService(testClient(server, true), server.URL, apiKey).Lookup("8.8.8.8")
	if _, limited := rateLimitCooldown(err); !limited {
		t.Fatalf("expected HTTP 429 to be detected as a rate limit, got %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("API key leaked in error: %v", err)
	}
}

func TestIPGeolocationRequiresAPIKey(t *testing.T) {
	service := newIPGeolocationService(nil, "", "")
	if err := service.Validate(); err == nil {
		t.Fatal("expected missing API key to fail validation")
	}
	if _, err := service.Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected lookup without API key to fail")
	}
}

func TestIPGeolocationErrorDoesNotLeakAPIKey(t *testing.T) {
	const apiKey = "super-secret-key"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := newIPGeolocationService(testClient(server, true), server.URL, apiKey).Lookup("8.8.8.8")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func TestIPGeolocationNetworkErrorDoesNotLeakAPIKey(t *testing.T) {
	const apiKey = "super-secret-key"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := testClient(server, true)
	baseURL := server.URL
	server.Close()

	_, err := newIPGeolocationService(client, baseURL, apiKey).Lookup("8.8.8.8")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("error leaked API key: %v", err)
	}
}

func TestIPGeolocationInvalidIP(t *testing.T) {
	service := newIPGeolocationService(nil, "", "key")
	if _, err := service.Lookup("invalid"); !errors.Is(err, ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP, got %v", err)
	}
}
