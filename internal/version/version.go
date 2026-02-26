package version

import "fmt"

// These variables are set at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("talons %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
