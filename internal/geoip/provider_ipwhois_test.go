package geoip

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ipWhoisSuccessHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "8.8.8.8"):
			writeJSON(w, `{
				"ip":"8.8.8.8","success":true,"country":"United States","region":"California",
				"city":"Mountain View","latitude":37.4056,"longitude":-122.0775,
				"connection":{"isp":"Google LLC"},"timezone":{"id":"America/Los_Angeles"}
			}`)
		case strings.Contains(r.URL.Path, "2001:4860:4860::8888"):
			writeJSON(w, `{
				"ip":"2001:4860:4860::8888","success":true,"country":"United States","region":"California",
				"city":"Mountain View","latitude":37.4056,"longitude":-122.0775,
				"connection":{"isp":"Google LLC"},"timezone":{"id":"America/Los_Angeles"}
			}`)
		default:
			http.NotFound(w, r)
		}
	}
}

func TestIPWhoisLookupIPv4AndIPv6(t *testing.T) {
	server := httptest.NewTLSServer(ipWhoisSuccessHandler(t))
	defer server.Close()

	service := newIPWhoisService(testClient(server, true), server.URL)
	for _, ip := range []string{"8.8.8.8", "2001:4860:4860::8888"} {
		info, err := service.Lookup(ip)
		if err != nil {
			t.Fatalf("Lookup(%s) failed: %v", ip, err)
		}
		if info.IP != ip {
			t.Errorf("IP = %q, want %q", info.IP, ip)
		}
		if info.Country != "United States" || info.Region != "California" || info.City != "Mountain View" {
			t.Errorf("unexpected location: %+v", info)
		}
		if info.ISP != "Google LLC" || info.Timezone != "America/Los_Angeles" {
			t.Errorf("unexpected ISP/timezone: %+v", info)
		}
		if info.Lat != 37.4056 || info.Lon != -122.0775 {
			t.Errorf("unexpected coordinates: %+v", info)
		}
	}
}

func TestIPWhoisInvalidIP(t *testing.T) {
	service := newIPWhoisService(nil, "")
	if _, err := service.Lookup("not-an-ip"); !errors.Is(err, ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP, got %v", err)
	}
}

func TestIPWhoisAPIFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"ip":"8.8.8.8","success":false,"message":"reserved range"}`)
	}))
	defer server.Close()

	_, err := newIPWhoisService(testClient(server, true), server.URL).Lookup("8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "reserved range") {
		t.Fatalf("expected API failure message, got %v", err)
	}
}

func TestIPWhoisRejectsMismatchedAndInvalidResponses(t *testing.T) {
	tests := map[string]string{
		"mismatched": `{"ip":"1.1.1.1","success":true,"latitude":1,"longitude":1}`,
		"bad coords": `{"ip":"8.8.8.8","success":true,"latitude":999,"longitude":1}`,
		"empty":      ``,
		"invalid":    `{not json`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, body)
			}))
			defer server.Close()
			if _, err := newIPWhoisService(testClient(server, true), server.URL).Lookup("8.8.8.8"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestIPWhoisHTTPErrorsAndRetryAfter(t *testing.T) {
	tests := []struct {
		status     int
		retryAfter string
		wantRate   bool
		wantDelay  time.Duration
	}{
		{http.StatusForbidden, "", false, 0},
		{http.StatusInternalServerError, "", false, 0},
		{http.StatusTooManyRequests, "120", true, 2 * time.Minute},
		{http.StatusTooManyRequests, "", true, 0},
	}
	for _, test := range tests {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if test.retryAfter != "" {
				w.Header().Set("Retry-After", test.retryAfter)
			}
			w.WriteHeader(test.status)
		}))
		_, err := newIPWhoisService(testClient(server, true), server.URL).Lookup("8.8.8.8")
		server.Close()
		if err == nil {
			t.Fatalf("status %d: expected error", test.status)
		}
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) {
			t.Fatalf("status %d: unexpected error type %T", test.status, err)
		}
		if statusErr.IsRateLimit() != test.wantRate {
			t.Errorf("status %d: IsRateLimit = %t, want %t", test.status, statusErr.IsRateLimit(), test.wantRate)
		}
		if test.wantDelay > 0 && statusErr.RetryAfter != test.wantDelay {
			t.Errorf("RetryAfter = %v, want %v", statusErr.RetryAfter, test.wantDelay)
		}
	}
}

func TestIPWhoisOversizedResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"ip":"8.8.8.8","success":true,"padding":"`+strings.Repeat("a", maxResponseBodySize+100)+`"}`)
	}))
	defer server.Close()

	_, err := newIPWhoisService(testClient(server, true), server.URL).Lookup("8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestIPWhoisRefusesHTTPSDowngradeRedirect(t *testing.T) {
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"ip":"8.8.8.8","success":true,"latitude":1,"longitude":1}`)
	}))
	defer insecure.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL+"/8.8.8.8", http.StatusFound)
	}))
	defer secure.Close()

	_, err := newIPWhoisService(testClient(secure, true), secure.URL).Lookup("8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("expected HTTPS downgrade rejection, got %v", err)
	}
}

func TestIPWhoisRefusesCrossHostRedirect(t *testing.T) {
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"ip":"8.8.8.8","success":true,"latitude":1,"longitude":1}`)
	}))
	defer other.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/8.8.8.8", http.StatusFound)
	}))
	defer secure.Close()

	_, err := newIPWhoisService(testClient(secure, true), secure.URL).Lookup("8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "unexpected host") {
		t.Fatalf("expected cross-host redirect rejection, got %v", err)
	}
}

func TestIPWhoisNetworkFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := testClient(server, true)
	baseURL := server.URL
	server.Close()

	if _, err := newIPWhoisService(client, baseURL).Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected network failure")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("30"); got != 30*time.Second {
		t.Errorf("seconds: got %v", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Errorf("garbage: got %v", got)
	}
	if got := parseRetryAfter("-1"); got != 0 {
		t.Errorf("negative: got %v", got)
	}
	date := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(date)
	if got < 80*time.Second || got > 100*time.Second {
		t.Errorf("HTTP date: got %v", got)
	}
}
