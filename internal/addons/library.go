package addons

import (
	"bytes"
	"encoding/json"
	"fmt"

	_ "embed"

	"github.com/MarJC5/vigie/addon/info"
)

// library.json lists the known add-ons, in the format of their own descriptions: it is data, so
// the code still names no add-on. It is generated from the installed add-ons, never edited by hand.
//
//go:generate go run gen_library.go
//go:embed library.json
var library []byte

// Known is an add-on of the Library: its description, without settings, and where to get it.
type Known struct {
	Info    info.Info
	Install string // where its page says how to install it
}

// Library returns the known add-ons, each description checked like an add-on's own.
func Library() ([]Known, error) {
	var entries []struct {
		Install string          `json:"install"`
		Info    json.RawMessage `json:"info"`
	}
	dec := json.NewDecoder(bytes.NewReader(library))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("library: %w", err)
	}
	known := make([]Known, 0, len(entries))
	for _, e := range entries {
		i, err := info.Read(bytes.NewReader(e.Info))
		if err != nil {
			return nil, fmt.Errorf("library: %w", err)
		}
		known = append(known, Known{Info: i, Install: e.Install})
	}
	return known, nil
}
