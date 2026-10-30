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
		if r.URL.Query().Get("apiKey") != apiKey {
			t.Errorf("api key was not sent as a query parameter")
		}
		if r.URL.Query().Get("ip") != "8.8.8.8" {
			t.Errorf("unexpected IP query: %s", r.URL.Query().Get("ip"))
		}
		writeJSON(w, `{
			"ip":"8.8.8.8","country_name":"United States","state_prov":"California","city":"Mountain View",
			"isp":"Google LLC","time_zone":"America/Los_Angeles","latitude":37.4,"longitude":-122.1
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
