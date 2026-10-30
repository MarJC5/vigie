//go:build ignore

// gen_library refreshes library.json from the add-ons themselves: each entry takes the description
// its installed add-on prints with --vigie-info, without settings, so the Library is never written
// by hand. An add-on of the Library that is not installed, or a development build of one, stops
// it. A new entry needs only its install address and its name:
// {"install": "...", "info": {"name": "..."}}.
//
// Run with go generate ./internal/addons, or make library.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/MarJC5/vigie/addon/info"
	"github.com/MarJC5/vigie/internal/addons"
)

func main() {
	if err := run("library.json"); err != nil {
		fmt.Fprintln(os.Stderr, "gen_library:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []struct {
		Install string `json:"install"`
		Info    struct {
			Name string `json:"name"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	home, _ := os.UserHomeDir()
	installed := map[string]addons.Installed{}
	for _, a := range addons.Find(context.Background(), addons.Folders(os.Getenv("PATH"), home)) {
		installed[a.Name] = a
	}
	type entry struct {
		Install string    `json:"install"`
		Info    info.Info `json:"info"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		a, ok := installed[e.Info.Name]
		switch {
		case !ok:
			return fmt.Errorf("%s%s is not installed: install it to refresh its entry", addons.Prefix, e.Info.Name)
		case a.Err != nil:
			return fmt.Errorf("%s: %w", a.Path, a.Err)
		case a.Info.Release == "dev":
			// The Library names released versions only.
			return fmt.Errorf("%s is a development build: install a release of it", a.Path)
		}
		i := a.Info
		// The Library tells what an add-on is; its settings are asked once it is installed.
		i.Settings = []info.Setting{}
		i.Format, i.Version = info.Format, info.Version
		out = append(out, entry{Install: e.Install, Info: i})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
