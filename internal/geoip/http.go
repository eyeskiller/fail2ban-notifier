package geoip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout  = 10 * time.Second
	maxResponseBodySize = 1 << 20 // 1 MiB
	maxRedirects        = 3
)

// HTTPStatusError describes a non-success HTTP response without including the
// request URL (which may contain an API key).
type HTTPStatusError struct {
	Provider   string
	StatusCode int
	Status     string
	RetryAfter time.Duration
}

func (e *HTTPStatusError) Error() string {
	if e.Provider == "" {
		return fmt.Sprintf("HTTP error: %s", e.Status)
	}
	return fmt.Sprintf("%s HTTP error: %s", e.Provider, e.Status)
}

// IsRateLimit reports whether the response was an HTTP 429 response.
func (e *HTTPStatusError) IsRateLimit() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

func newHTTPClient(timeout time.Duration, requireHTTPS bool) *http.Client {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			previous := via[len(via)-1]
			if previous.URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("refusing to downgrade GeoIP request from HTTPS to HTTP")
			}
			if requireHTTPS && req.URL.Scheme != "https" {
				return errors.New("refusing non-HTTPS GeoIP redirect")
			}
			if req.URL.Host != via[0].URL.Host {
				return fmt.Errorf("refusing GeoIP redirect to unexpected host %q", req.URL.Host)
			}
			return nil
		},
	}
}

func doJSON(ctx context.Context, client *http.Client, req *http.Request, provider string, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", provider, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &HTTPStatusError{
			Provider:   provider,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return fmt.Errorf("%s response read failed: %w", provider, err)
	}
	if len(body) > maxResponseBodySize {
		return fmt.Errorf("%s response exceeds %d bytes", provider, maxResponseBodySize)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("%s returned an empty response", provider)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", provider, err)
	}

	return nil
}

// parseRetryAfter supports both delay-seconds and HTTP-date formats.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay <= 0 {
			return 0
		}
		return delay
	}

	return 0
}
