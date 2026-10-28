// Package locales embeds the translation catalogs (en.json, fr.json). The same files are read by
// the Go side (menus, notifications, command line) and by the interface.
package locales

import (
	"embed"
	"io/fs"
)

//go:embed en.json fr.json
var files embed.FS

// Files returns the catalogs, one <language>.json per language.
func Files() fs.FS {
	return files
}
