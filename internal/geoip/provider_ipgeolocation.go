package geoip

import (
	"net/http"
	"net/url"
	"strings"
)

const ipGeolocationBaseURL = "https://api.ipgeolocation.io"

// IPGeolocationService implements the paid ipgeolocation.io provider.
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

type ipGeolocationResponse struct {
	IP          string  `json:"ip"`
	CountryName string  `json:"country_name"`
	StateProv   string  `json:"state_prov"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	TimeZone    string  `json:"time_zone"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Message     string  `json:"message"`
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
	u.Path = "/ipgeo"
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
		return nil, &ProviderError{
			Provider: "ipgeolocation.io",
			Message:  strings.ReplaceAll(err.Error(), s.apiKey, "[REDACTED]"),
		}
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
	if !validCoordinates(result.Latitude, result.Longitude) {
		return nil, &ProviderError{Provider: "ipgeolocation.io", Message: "response contained invalid coordinates"}
	}

	return &Info{
		IP:       normalized,
		Country:  result.CountryName,
		Region:   result.StateProv,
		City:     result.City,
		ISP:      result.ISP,
		Timezone: result.TimeZone,
		Lat:      result.Latitude,
		Lon:      result.Longitude,
	}, nil
}
