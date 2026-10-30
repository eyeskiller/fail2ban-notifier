package geoip

import (
	"net/http"
	"net/url"
)

const ipWhoisBaseURL = "https://ipwho.is" // #nosec G101 -- public API endpoint, not a credential

// IPWhoisService implements the ipwho.is provider. It only uses HTTPS and
// requires no API key.
type IPWhoisService struct {
	client  *http.Client
	baseURL string
}

func newIPWhoisService(client *http.Client, baseURL string) *IPWhoisService {
	if client == nil {
		client = newHTTPClient(defaultHTTPTimeout, true)
	}
	if baseURL == "" {
		baseURL = ipWhoisBaseURL
	}
	return &IPWhoisService{client: client, baseURL: baseURL}
}

func (s *IPWhoisService) GetName() string {
	return "ipwho.is"
}

type ipWhoisResponse struct {
	IP         string  `json:"ip"`
	Success    bool    `json:"success"`
	Message    string  `json:"message"`
	Country    string  `json:"country"`
	Region     string  `json:"region"`
	City       string  `json:"city"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Connection struct {
		ISP string `json:"isp"`
	} `json:"connection"`
	Timezone struct {
		ID string `json:"id"`
	} `json:"timezone"`
}

func (s *IPWhoisService) Lookup(ip string) (*Info, error) {
	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil, ErrInvalidIP
	}

	u, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	u.Path = "/" + normalized

	ctx, cancel := contextWithTimeout(defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	var result ipWhoisResponse
	if err := doJSON(ctx, s.client, req, "ipwho.is", &result); err != nil {
		return nil, err
	}

	if !result.Success {
		if result.Message != "" {
			return nil, &ProviderError{Provider: "ipwho.is", Message: result.Message}
		}
		return nil, &ProviderError{Provider: "ipwho.is", Message: "lookup was not successful"}
	}

	responseIP, valid := normalizeIP(result.IP)
	if !valid || responseIP != normalized {
		return nil, &ProviderError{Provider: "ipwho.is", Message: "response contained an invalid or mismatched IP address"}
	}
	if !validCoordinates(result.Latitude, result.Longitude) {
		return nil, &ProviderError{Provider: "ipwho.is", Message: "response contained invalid coordinates"}
	}

	return &Info{
		IP:       normalized,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.Connection.ISP,
		Timezone: result.Timezone.ID,
		Lat:      result.Latitude,
		Lon:      result.Longitude,
	}, nil
}
