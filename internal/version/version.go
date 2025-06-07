package version

import (
	"fmt"
	"runtime"
	"time"
)

// Build information. Populated at build-time.
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = runtime.Version()
)

// GetBuildInfo returns formatted build information string.
func GetBuildInfo() string {
	return fmt.Sprintf("fail2ban-notifier %s (commit: %s, built at: %s, using: %s)",
		Version, Commit, Date, GoVersion)
}

// InitBuildInfo initializes build information if not already set
func InitBuildInfo() {
	// If Date is still "unknown", set it to current time
	if Date == "unknown" {
		Date = time.Now().Format(time.RFC3339)
	}
}
