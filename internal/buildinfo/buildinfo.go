// Package buildinfo exposes the version of the running Vigie build.
package buildinfo

// version is set at build time with -ldflags "-X github.com/MarJC5/vigie/internal/buildinfo.version=vX.Y.Z".
var version = "dev"

// Version returns the version of this build, "dev" for development builds.
func Version() string {
	return version
}
