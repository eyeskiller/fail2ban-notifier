package geoip

import (
	"net/netip"
	"strings"
)

// specialPurposePrefixes contains non-globally-routable ranges that are not
// covered by the netip convenience methods. Sending addresses from these ranges
// to a third-party GeoIP API would waste quota and leak internal topology.
//
// Sources: RFC 6890, RFC 5737, RFC 6598, RFC 3849, RFC 7526.
var specialPurposePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),  // deprecated 6to4 relay anycast
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved (includes broadcast)
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("2001:10::/28"),    // ORCHID (deprecated)
	netip.MustParsePrefix("2001:20::/28"),    // ORCHIDv2
}

// normalizeIP parses an IP and canonicalizes IPv4-mapped IPv6 addresses to
// their IPv4 form. It returns false for malformed input.
func normalizeIP(raw string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}

// isPublicIP reports whether an IP is globally routable and therefore safe to
// send to an external GeoIP provider. Invalid input is never public.
func isPublicIP(raw string) bool {
	normalized, ok := normalizeIP(raw)
	if !ok {
		return false
	}
	addr, err := netip.ParseAddr(normalized)
	if err != nil {
		return false
	}

	if addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() {
		return false
	}

	if !addr.IsGlobalUnicast() {
		return false
	}

	for _, prefix := range specialPurposePrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}

	return true
}

// isPrivateIP is kept for callers that need the inverse classification. It
// returns false for malformed addresses, matching the historical behavior.
func isPrivateIP(raw string) bool {
	if _, ok := normalizeIP(raw); !ok {
		return false
	}
	return !isPublicIP(raw)
}
