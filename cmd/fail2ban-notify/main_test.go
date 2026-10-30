package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
)

func TestNotificationDeliveryContinuesWhenGeoIPFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell connector test is not supported on Windows")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "notification.txt")
	script := filepath.Join(dir, "capture.sh")
	content := "#!/bin/bash\nprintf '%s|%s|%s' \"$F2B_IP\" \"$F2B_JAIL\" \"$F2B_ACTION\" > " + output + "\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.GeoIP.Enabled = true
	cfg.GeoIP.Service = config.GeoIPServiceGeoLite2
	cfg.GeoIP.CityDBPath = filepath.Join(dir, "missing-City.mmdb")
	cfg.GeoIP.ASNDBPath = filepath.Join(dir, "missing-ASN.mmdb")
	cfg.GeoIP.CachePath = filepath.Join(dir, "geoip.json")
	cfg.GeoIP.Cache = false
	cfg.Connectors = []config.ConnectorConfig{
		{
			Name:    "capture",
			Type:    config.ConnectorTypeScript,
			Enabled: true,
			Path:    script,
			Timeout: 5,
		},
	}

	logger := log.New(io.Discard, "", 0)
	handleNotification("8.8.8.8", "sshd", ActionBan, 3, cfg, logger)

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("notification connector did not run after GeoIP failure: %v", err)
	}
	if !strings.Contains(string(data), "8.8.8.8|sshd|ban") {
		t.Fatalf("unexpected notification data: %s", data)
	}
}
