package version

import "fmt"

// These variables are set at build time using -ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String returns a formatted version string
func String() string {
	return fmt.Sprintf("slack-shell %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// Short returns just the version number
func Short() string {
	return Version
}
