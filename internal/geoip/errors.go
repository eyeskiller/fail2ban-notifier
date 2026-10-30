package geoip

import (
	"context"
	"errors"
	"time"
)

const userAgent = "fail2ban-notify/2.0 (geoip)"

var (
	// ErrInvalidIP indicates that the supplied value is not a valid IP address.
	ErrInvalidIP = errors.New("invalid IP address")
	// ErrNoData indicates that a provider had no record for a valid IP.
	ErrNoData = errors.New("no GeoIP data available for IP")
	// ErrRateLimited indicates that a provider is in a persisted cooldown.
	ErrRateLimited = errors.New("GeoIP provider is rate limited")
	// ErrInsecureHTTP indicates that an HTTP-only provider was selected without
	// explicit user consent.
	ErrInsecureHTTP = errors.New("insecure HTTP GeoIP provider requires allow_insecure_http=true")
	// ErrProviderUnavailable indicates that a provider cannot be used, for
	// example because a local database is missing.
	ErrProviderUnavailable = errors.New("GeoIP provider unavailable")
	// ErrLookupInProgress is reserved for duplicate-request suppression.
	ErrLookupInProgress = errors.New("GeoIP lookup already in progress")
)

// ProviderError is a non-HTTP failure reported by a GeoIP provider.
type ProviderError struct {
	Provider string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Provider == "" {
		return e.Message
	}
	return e.Provider + ": " + e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func validCoordinates(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
