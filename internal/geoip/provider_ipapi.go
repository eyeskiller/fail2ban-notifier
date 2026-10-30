package geoip

import (
	"net/http"
	"net/url"
)

const ipAPIBaseURL = "http://ip-api.com"

// IPAPIService implements the legacy ip-api.com provider. The free tier is
// HTTP-only, so callers must explicitly opt in to insecure transport.
type IPAPIService struct {
	client        *http.Client
	baseURL       string
	allowInsecure bool
}

func newIPAPIService(client *http.Client, baseURL string, allowInsecure bool) *IPAPIService {
	if client == nil {
		client = newHTTPClient(defaultHTTPTimeout, false)
	}
	if baseURL == "" {
		baseURL = ipAPIBaseURL
	}
	return &IPAPIService{client: client, baseURL: baseURL, allowInsecure: allowInsecure}
}

func (s *IPAPIService) GetName() string {
	return "ip-api.com"
}

func (s *IPAPIService) Validate() error {
	if !s.allowInsecure {
		return ErrInsecureHTTP
	}
	return nil
}

type ipAPIResponse struct {
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	Query      string  `json:"query"`
	Country    string  `json:"country"`
	RegionName string  `json:"regionName"`
	City       string  `json:"city"`
	ISP        string  `json:"isp"`
	Timezone   string  `json:"timezone"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
}

func (s *IPAPIService) Lookup(ip string) (*Info, error) {
	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil, ErrInvalidIP
	}
	if !s.allowInsecure {
		return nil, ErrInsecureHTTP
	}

	u, err := url.Parse(s.baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, ErrProviderUnavailable
	}
	u.Path = "/json/" + normalized
	query := u.Query()
	query.Set("fields", "status,message,query,country,regionName,city,isp,timezone,lat,lon")
	u.RawQuery = query.Encode()

	ctx, cancel := contextWithTimeout(defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	var result ipAPIResponse
	if err := doJSON(ctx, s.client, req, "ip-api.com", &result); err != nil {
		return nil, err
	}

	if result.Status != "success" {
		message := result.Message
		if message == "" {
			message = "lookup was not successful"
		}
		return nil, &ProviderError{Provider: "ip-api.com", Message: message}
	}

	if result.Query != "" {
		responseIP, valid := normalizeIP(result.Query)
		if !valid || responseIP != normalized {
			return nil, &ProviderError{Provider: "ip-api.com", Message: "response contained an invalid or mismatched IP address"}
		}
	}
	if !validCoordinates(result.Lat, result.Lon) {
		return nil, &ProviderError{Provider: "ip-api.com", Message: "response contained invalid coordinates"}
	}

	return &Info{
		IP:       normalized,
		Country:  result.Country,
		Region:   result.RegionName,
		City:     result.City,
		ISP:      result.ISP,
		Timezone: result.Timezone,
		Lat:      result.Lat,
		Lon:      result.Lon,
	}, nil
}
