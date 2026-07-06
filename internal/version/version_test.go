package version

import (
	"strings"
	"testing"
	"time"
)

func TestGetBuildInfo(t *testing.T) {
	// Save and restore globals
	savedVersion := Version
	savedCommit := Commit
	savedDate := Date
	defer func() {
		Version = savedVersion
		Commit = savedCommit
		Date = savedDate
	}()

	Version = "1.1.2"
	Commit = "abc1234"
	Date = "2026-07-06T12:00:00Z"

	info := GetBuildInfo()

	if !strings.Contains(info, "fail2ban-notifier") {
		t.Errorf("GetBuildInfo() missing project name, got: %s", info)
	}
	if !strings.Contains(info, "1.1.2") {
		t.Errorf("GetBuildInfo() missing version, got: %s", info)
	}
	if !strings.Contains(info, "abc1234") {
		t.Errorf("GetBuildInfo() missing commit, got: %s", info)
	}
	if !strings.Contains(info, "2026-07-06") {
		t.Errorf("GetBuildInfo() missing date, got: %s", info)
	}
}

func TestInitBuildInfoSetsDate(t *testing.T) {
	savedDate := Date
	savedGoVersion := GoVersion
	defer func() {
		Date = savedDate
		GoVersion = savedGoVersion
	}()

	Date = "unknown"

	InitBuildInfo()

	if Date == "unknown" {
		t.Error("InitBuildInfo() did not update Date")
	}
	if Date == "" {
		t.Error("InitBuildInfo() set Date to empty string")
	}
	_, err := time.Parse(time.RFC3339, Date)
	if err != nil {
		t.Errorf("InitBuildInfo() set Date to non-RFC3339 format %q: %v", Date, err)
	}
}

func TestInitBuildInfoPreservesExistingDate(t *testing.T) {
	savedDate := Date
	defer func() { Date = savedDate }()

	Date = "2026-01-01T00:00:00Z"

	InitBuildInfo()

	if Date != "2026-01-01T00:00:00Z" {
		t.Errorf("InitBuildInfo() overwrote existing Date, got %q", Date)
	}
}
