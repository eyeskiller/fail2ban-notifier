package geoip

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIPAPIRequiresExplicitInsecureOptIn(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	service := newIPAPIService(server.Client(), server.URL, false)
	if _, err := service.Lookup("8.8.8.8"); !errors.Is(err, ErrInsecureHTTP) {
		t.Fatalf("expected ErrInsecureHTTP, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("insecure provider made %d requests without consent", requests)
	}
	if err := service.Validate(); !errors.Is(err, ErrInsecureHTTP) {
		t.Fatalf("expected validation to reject insecure provider, got %v", err)
	}
}

func TestIPAPILookupWithOptIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Scheme != "" {
			t.Errorf("unexpected URL scheme on server request")
		}
		if !strings.Contains(r.URL.Path, "/json/8.8.8.8") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("fields") == "" {
			t.Error("expected fields query parameter")
		}
		writeJSON(w, `{
			"status":"success","query":"8.8.8.8","country":"United States","regionName":"California",
			"city":"Mountain View","isp":"Google LLC","timezone":"America/Los_Angeles","lat":37.4,"lon":-122.1
		}`)
	}))
	defer server.Close()

	service := newIPAPIService(server.Client(), server.URL, true)
	if err := service.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	info, err := service.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if info.Country != "United States" || info.Region != "California" || info.City != "Mountain View" {
		t.Errorf("unexpected mapping: %+v", info)
	}
	if info.ISP != "Google LLC" || info.Timezone != "America/Los_Angeles" {
		t.Errorf("unexpected mapping: %+v", info)
	}
}

func TestIPAPIFailureResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{"api failure", http.StatusOK, `{"status":"fail","message":"private range"}`, "private range"},
		{"forbidden", http.StatusForbidden, ``, "HTTP error"},
		{"server error", http.StatusInternalServerError, ``, "HTTP error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := newIPAPIService(server.Client(), server.URL, true).Lookup("8.8.8.8")
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestIPAPIInvalidIP(t *testing.T) {
	service := newIPAPIService(nil, "", true)
	if _, err := service.Lookup("invalid"); !errors.Is(err, ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP, got %v", err)
	}
}
