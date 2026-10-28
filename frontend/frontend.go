// Package frontend embeds the built interface (frontend/dist, produced by `npm run build`) in the
// Vigie binary. go:embed only reaches files under its own directory, hence this package here.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built interface, with index.html at its root.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}
