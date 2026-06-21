// Package version exposes the build-time version string for the cylon binary.
package version

// Version is the build version, overridden at link time with
//
//	-ldflags "-X github.com/t0mer/cylon/internal/version.Version=<v>"
//
// It follows the YYYY.M.PATCH scheme (no leading zero on the month).
var Version = "dev"
