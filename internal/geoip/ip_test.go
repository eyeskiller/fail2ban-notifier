package geoip

import "testing"

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip     string
		public bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2001:4860:4860::8888", true},
		{"::ffff:8.8.8.8", true},
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"172.31.255.255", false},
		{"192.168.1.1", false},
		{"169.254.10.10", false},
		{"0.0.0.0", false},
		{"0.1.2.3", false},
		{"100.64.0.1", false},
		{"192.0.0.1", false},
		{"192.0.2.1", false},
		{"192.88.99.1", false},
		{"198.18.0.1", false},
		{"198.51.100.1", false},
		{"203.0.113.1", false},
		{"224.0.0.1", false},
		{"240.0.0.1", false},
		{"255.255.255.255", false},
		{"::", false},
		{"::1", false},
		{"fe80::1", false},
		{"fc00::1", false},
		{"fd12:3456::1", false},
		{"ff02::1", false},
		{"2001:db8::1", false},
		{"2001:10::1", false},
		{"2001:20::1", false},
		{"::ffff:192.168.1.1", false},
		{"invalid-ip", false},
		{"", false},
	}

	for _, test := range tests {
		if got := isPublicIP(test.ip); got != test.public {
			t.Errorf("isPublicIP(%q) = %t, want %t", test.ip, got, test.public)
		}
	}
}

func TestNormalizeIP(t *testing.T) {
	tests := map[string]string{
		"8.8.8.8":              "8.8.8.8",
		"::ffff:8.8.8.8":       "8.8.8.8",
		"2001:4860:4860::8888": "2001:4860:4860::8888",
	}
	for input, want := range tests {
		got, ok := normalizeIP(input)
		if !ok || got != want {
			t.Errorf("normalizeIP(%q) = %q, %t; want %q, true", input, got, ok, want)
		}
	}
	if _, ok := normalizeIP("not-an-ip"); ok {
		t.Error("normalizeIP accepted an invalid address")
	}
}

func TestIsPrivateIP(t *testing.T) {
	if !isPrivateIP("192.168.1.1") {
		t.Error("expected 192.168.1.1 to be private")
	}
	if isPrivateIP("8.8.8.8") {
		t.Error("expected 8.8.8.8 to be public")
	}
	if isPrivateIP("invalid") {
		t.Error("invalid addresses should not be classified as private")
	}
}
