package geoip

import (
	"fmt"
	"log"
	"net/netip"
	"os"
	"strconv"
	"sync"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoLite2Service implements fully offline lookups using local MaxMind
// GeoLite2 City and ASN databases.
type GeoLite2Service struct {
	cityPath string
	asnPath  string
	logger   *log.Logger

	mu      sync.Mutex
	opened  bool
	city    *maxminddb.Reader
	asn     *maxminddb.Reader
	openErr error
}

func newGeoLite2Service(cityPath, asnPath string, logger *log.Logger) *GeoLite2Service {
	return &GeoLite2Service{
		cityPath: cityPath,
		asnPath:  asnPath,
		logger:   logger,
	}
}

func (s *GeoLite2Service) GetName() string {
	return "GeoLite2"
}

// CacheFingerprint changes when either local database file changes, so cached
// results are invalidated after a GeoLite2 update.
func (s *GeoLite2Service) CacheFingerprint() string {
	return fileFingerprint(s.cityPath) + "|" + fileFingerprint(s.asnPath)
}

func fileFingerprint(path string) string {
	if path == "" {
		return "none"
	}
	info, err := os.Stat(path)
	if err != nil {
		return path + ":missing"
	}
	return fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
}

// Validate opens the configured databases without performing a network lookup.
// The ASN database is optional.
func (s *GeoLite2Service) Validate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openLocked()
}

func (s *GeoLite2Service) openLocked() error {
	if s.opened {
		return s.openErr
	}
	s.opened = true

	if s.cityPath == "" {
		s.openErr = fmt.Errorf("%w: city_db_path is not configured", ErrProviderUnavailable)
		return s.openErr
	}

	city, err := maxminddb.Open(s.cityPath)
	if err != nil {
		s.openErr = fmt.Errorf("%w: cannot open GeoLite2 City database %s: %v", ErrProviderUnavailable, s.cityPath, err)
		return s.openErr
	}
	s.city = city

	if s.asnPath != "" {
		asn, asnErr := maxminddb.Open(s.asnPath)
		if asnErr != nil {
			if s.logger != nil {
				s.logger.Printf("Warning: GeoLite2 ASN database unavailable (%s): %v; continuing with City data only", s.asnPath, asnErr)
			}
		} else {
			s.asn = asn
		}
	}

	return nil
}

func (s *GeoLite2Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	if s.city != nil {
		if err := s.city.Close(); err != nil {
			firstErr = err
		}
		s.city = nil
	}
	if s.asn != nil {
		if err := s.asn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.asn = nil
	}
	s.opened = false
	s.openErr = nil
	return firstErr
}

type geoLite2CityRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

type geoLite2ASNRecord struct {
	Number       uint64 `maxminddb:"autonomous_system_number"`
	Organization string `maxminddb:"autonomous_system_organization"`
}

func (s *GeoLite2Service) Lookup(ip string) (*Info, error) {
	normalized, ok := normalizeIP(ip)
	if !ok {
		return nil, ErrInvalidIP
	}
	addr, err := netip.ParseAddr(normalized)
	if err != nil {
		return nil, ErrInvalidIP
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.openLocked(); err != nil {
		return nil, err
	}

	cityResult := s.city.Lookup(addr)
	if err := cityResult.Err(); err != nil {
		return nil, fmt.Errorf("%w: GeoLite2 City lookup failed: %v", ErrProviderUnavailable, err)
	}
	if !cityResult.Found() {
		return nil, ErrNoData
	}

	var cityRecord geoLite2CityRecord
	if err := cityResult.Decode(&cityRecord); err != nil {
		return nil, fmt.Errorf("%w: cannot decode GeoLite2 City record: %v", ErrProviderUnavailable, err)
	}

	info := &Info{
		IP:       normalized,
		Country:  localizedName(cityRecord.Country.Names, cityRecord.Country.ISOCode),
		Region:   lastSubdivisionName(cityRecord.Subdivisions),
		City:     localizedName(cityRecord.City.Names, ""),
		Timezone: cityRecord.Location.TimeZone,
		Lat:      cityRecord.Location.Latitude,
		Lon:      cityRecord.Location.Longitude,
	}

	if s.asn != nil {
		asnResult := s.asn.Lookup(addr)
		if err := asnResult.Err(); err == nil && asnResult.Found() {
			var asnRecord geoLite2ASNRecord
			if err := asnResult.Decode(&asnRecord); err == nil {
				if asnRecord.Organization != "" {
					// The ASN organization is not necessarily the consumer ISP.
					info.ISP = asnRecord.Organization
				}
				if asnRecord.Number != 0 {
					info.ASN = "AS" + strconv.FormatUint(asnRecord.Number, 10)
				}
			}
		}
	}
	return info, nil
}

func localizedName(names map[string]string, fallback string) string {
	if name := names["en"]; name != "" {
		return name
	}
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return fallback
}

func lastSubdivisionName(subdivisions []struct {
	Names map[string]string `maxminddb:"names"`
}) string {
	if len(subdivisions) == 0 {
		return ""
	}
	return localizedName(subdivisions[len(subdivisions)-1].Names, "")
}
