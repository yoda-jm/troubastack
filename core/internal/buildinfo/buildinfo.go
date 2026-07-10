// Package buildinfo exposes the git version stamped into the binary at build
// time (T29). The Makefile's dist target injects both values via
//
//	-ldflags "-X troubastack/core/internal/buildinfo.version=$(git describe --always --dirty)
//	          -X troubastack/core/internal/buildinfo.builtAt=$(date -u +%Y-%m-%dT%H:%MZ)"
//
// A plain `go build`/`go run` (no ldflags) reports "dev" — never an error. This
// is display/diagnosis only; compatibility ENFORCEMENT is explicitly out of scope
// (the /api/version endpoint is the future hook).
package buildinfo

var (
	version string // set via -ldflags -X; empty on unstamped builds
	builtAt string
)

// Version returns the stamped `git describe --always --dirty`, or "dev".
func Version() string {
	if version == "" {
		return "dev"
	}
	return version
}

// BuiltAt returns the stamped UTC build timestamp, or "unknown".
func BuiltAt() string {
	if builtAt == "" {
		return "unknown"
	}
	return builtAt
}
