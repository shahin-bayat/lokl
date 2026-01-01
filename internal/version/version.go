// Package version provides build-time version information.
// The Version variable is set via ldflags during build:
//
//	go build -ldflags "-X github.com/shahin-bayat/devenv/internal/version.Version=1.0.0"
package version

// Version is the current version of devenv.
// Set to "dev" by default, overridden at build time.
var Version = "dev"
