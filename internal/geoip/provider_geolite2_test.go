package geoip

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeoLite2CityAndASNLookup(t *testing.T) {
	city := cityFixture(t)
	asn := asnFixture(t)
	service := newGeoLite2Service(city, asn, testLogger())
	defer func() { _ = service.Close() }()

	info, err := service.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if info.Country != "United States" || info.Region != "California" || info.City != "Mountain View" {
		t.Errorf("unexpected city mapping: %+v", info)
	}
	if info.Timezone != "America/Los_Angeles" || info.Lat != 37.4056 || info.Lon != -122.0775 {
		t.Errorf("unexpected location mapping: %+v", info)
	}
	if info.ISP != "Google LLC" || info.ASN != "AS15169" {
		t.Errorf("unexpected ASN mapping: %+v", info)
	}

	info6, err := service.Lookup("2001:4860:4860::8888")
	if err != nil {
		t.Fatalf("IPv6 lookup failed: %v", err)
	}
	if info6.Country != "United Kingdom" || info6.City != "London" {
		t.Errorf("unexpected IPv6 mapping: %+v", info6)
	}
}

func TestGeoLite2CityOnlyOperation(t *testing.T) {
	service := newGeoLite2Service(cityFixture(t), filepath.Join(t.TempDir(), "missing-asn.mmdb"), testLogger())
	defer func() { _ = service.Close() }()

	info, err := service.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("City-only lookup failed: %v", err)
	}
	if info.Country != "United States" {
		t.Errorf("unexpected country: %+v", info)
	}
	if info.ISP != "" || info.ASN != "" {
		t.Errorf("expected no ASN enrichment, got %+v", info)
	}
}

func TestGeoLite2CorruptASNIsNonFatal(t *testing.T) {
	asnPath := filepath.Join(t.TempDir(), "bad-asn.mmdb")
	if err := os.WriteFile(asnPath, []byte("not an mmdb"), 0600); err != nil {
		t.Fatal(err)
	}
	service := newGeoLite2Service(cityFixture(t), asnPath, testLogger())
	defer func() { _ = service.Close() }()

	info, err := service.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("lookup should continue with City only: %v", err)
	}
	if info.ISP != "" {
		t.Errorf("unexpected ISP from corrupt ASN database: %+v", info)
	}
}

func TestGeoLite2MissingAndCorruptCityDatabase(t *testing.T) {
	missing := newGeoLite2Service(filepath.Join(t.TempDir(), "missing.mmdb"), "", testLogger())
	if err := missing.Validate(); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected missing database error, got %v", err)
	}
	if _, err := missing.Lookup("8.8.8.8"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected missing database lookup error, got %v", err)
	}
	_ = missing.Close()

	path := filepath.Join(t.TempDir(), "corrupt.mmdb")
	if err := os.WriteFile(path, []byte("not an mmdb"), 0600); err != nil {
		t.Fatal(err)
	}
	corrupt := newGeoLite2Service(path, "", testLogger())
	if err := corrupt.Validate(); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected corrupt database error, got %v", err)
	}
	_ = corrupt.Close()
}

func TestGeoLite2NoRecord(t *testing.T) {
	service := newGeoLite2Service(cityFixture(t), asnFixture(t), testLogger())
	defer func() { _ = service.Close() }()

	if _, err := service.Lookup("1.1.1.1"); !errors.Is(err, ErrNoData) {
		t.Fatalf("expected ErrNoData, got %v", err)
	}
}

func TestGeoLite2ValidateDoesNotNeedNetworkAndCanReopen(t *testing.T) {
	city := cityFixture(t)
	asn := asnFixture(t)
	service := newGeoLite2Service(city, asn, testLogger())
	if err := service.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if _, err := service.Lookup("8.8.8.8"); err != nil {
		t.Fatalf("lookup after reopen failed: %v", err)
	}
}

func TestGeoLite2FingerprintChangesWithDatabase(t *testing.T) {
	city := cityFixture(t)
	service := newGeoLite2Service(city, "", testLogger())
	first := service.CacheFingerprint()

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(city, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	second := service.CacheFingerprint()
	if first == second {
		t.Fatal("fingerprint did not change after database update")
	}
	if strings.Contains(first, "missing") {
		t.Fatalf("unexpected fingerprint: %s", first)
	}
}
