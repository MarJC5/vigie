// Package buildinfo exposes the version of the running Vigie build.
package buildinfo

// version is set at build time with -ldflags "-X github.com/MarJC5/vigie/internal/buildinfo.version=vX.Y.Z".
var version = "dev"

// Version returns the version of this build, "dev" for development builds.
func Version() string {
	return version
}

// Repository is where Vigie lives: what a service asked by Vigie can read to learn what it is.
const Repository = "https://github.com/MarJC5/vigie"

// UserAgent is how every request of Vigie says what it is: "Vigie/<version> (+<repository>)".
func UserAgent(version string) string {
	return "Vigie/" + version + " (+" + Repository + ")"
}
