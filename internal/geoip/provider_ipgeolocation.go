package geoip

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const ipGeolocationBaseURL = "https://api.ipgeolocation.io"

// IPGeolocationService implements the ipgeolocation.io provider (v3 IP Location API).
// An API key is required; the free plan is sufficient for Fail2Ban notifications.
type IPGeolocationService struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

func newIPGeolocationService(client *http.Client, baseURL, apiKey string) *IPGeolocationService {
	if client == nil {
		client = newHTTPClient(defaultHTTPTimeout, true)
	}
	if baseURL == "" {
		baseURL = ipGeolocationBaseURL
	}
	return &IPGeolocationService{apiKey: apiKey, client: client, baseURL: baseURL}
}

func (s *IPGeolocationService) GetName() string {
	return "ipgeolocation.io"
}

func (s *IPGeolocationService) Validate() error {
	if s.apiKey == "" {
		return &ProviderError{Provider: "ipgeolocation.io", Message: "api_key is required"}
	}
	return nil
}

// ipGeolocationResponse maps the v3 /v3/ipgeo response. Location fields are
// nested under "location", coordinates are returned as strings, the time zone
// is an object, and the ISP name is reported as the ASN organization.
type ipGeolocationResponse struct {
	IP       string `json:"ip"`
	Location struct {
		CountryName string `json:"country_name"`
		StateProv   string `json:"state_prov"`
		City        string `json:"city"`
		Latitude    string `json:"latitude"`
		Longitude   string `json:"longitude"`
	} `json:"location"`
	ASN struct {
		ASNumber     string `json:"as_number"`
		Organization string `json:"organization"`
	} `json:"asn"`
	TimeZone struct {
		Name string `json:"name"`
	} `json:"time_zone"`
	Message string `json:"message"`
}

func (s *IPGeolocationService) Lookup(ip string) (*Info, error) {
	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil, ErrInvalidIP
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}

	u, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	u.Path = "/v3/ipgeo"
	query := u.Query()
	query.Set("apiKey", s.apiKey)
	query.Set("ip", normalized)
	u.RawQuery = query.Encode()

	ctx, cancel := contextWithTimeout(defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	var result ipGeolocationResponse
	if err := doJSON(ctx, s.client, req, "ipgeolocation.io", &result); err != nil {
		providerErr := &ProviderError{
			Provider: "ipgeolocation.io",
			Message:  strings.ReplaceAll(err.Error(), s.apiKey, "[REDACTED]"),
		}
		// Keep the HTTP status (which never contains the key) so the manager can
		// detect HTTP 429 and apply its rate-limit cooldown.
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) {
			providerErr.Err = statusErr
		}
		return nil, providerErr
	}

	if result.Message != "" {
		return nil, &ProviderError{Provider: "ipgeolocation.io", Message: result.Message}
	}

	if result.IP != "" {
		responseIP, valid := normalizeIP(result.IP)
		if !valid || responseIP != normalized {
			return nil, &ProviderError{Provider: "ipgeolocation.io", Message: "response contained an invalid or mismatched IP address"}
		}
	}
	lat, latErr := parseCoordinate(result.Location.Latitude)
	lon, lonErr := parseCoordinate(result.Location.Longitude)
	if latErr != nil || lonErr != nil || !validCoordinates(lat, lon) {
		return nil, &ProviderError{Provider: "ipgeolocation.io", Message: "response contained invalid coordinates"}
	}

	return &Info{
		IP:       normalized,
		Country:  result.Location.CountryName,
		Region:   result.Location.StateProv,
		City:     result.Location.City,
		ISP:      result.ASN.Organization,
		ASN:      result.ASN.ASNumber,
		Timezone: result.TimeZone.Name,
		Lat:      lat,
		Lon:      lon,
	}, nil
}

// parseCoordinate converts a string coordinate from the API. An empty value
// is treated as unknown (0).
func parseCoordinate(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}
