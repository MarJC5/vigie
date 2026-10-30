// Package info is what an add-on says about itself, for Vigie's Add-ons page: `vigie-<name>
// --vigie-info` prints it as JSON and exits. Vigie draws the add-on's page from it with its own
// components: no code of the add-on runs in the window. Version 1; its JSON Schema is embedded
// (Schema).
package info

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	_ "embed"
)

// The flags Vigie passes to an add-on; an add-on keeps every other one for its own use.
const (
	// InfoFlag asks for the description, printed on the standard output.
	InfoFlag = "--vigie-info"
	// ExportFlag gives a source the path of the export to write, readable by its owner only.
	ExportFlag = "--vigie-export"
)

// The format's name and the version this package writes.
const (
	Format  = "vigie-addon"
	Version = 1
)

// The kinds of add-ons.
const (
	KindSource      = "source"      // writes a Vigie export, which Vigie imports with its preview
	KindDestination = "destination" // takes what Vigie writes somewhere else
)

// The kinds of settings a run asks for.
const (
	SettingFile     = "file"
	SettingText     = "text"
	SettingCheckbox = "checkbox"
	SettingChoice   = "choice"
)

// MaxImage bounds an add-on's image, as decoded.
const MaxImage = 256 << 10

// RunTimeout is how long Vigie lets a run take before it stops the program.
const RunTimeout = 2 * time.Minute

// The images an add-on can show, as data URIs.
var imageTypes = []string{"image/png", "image/svg+xml"}

var (
	// ErrNotAnInfo is returned for an output that is not a description of an add-on.
	ErrNotAnInfo = errors.New("not the description of a Vigie add-on")
	// ErrNewerVersion is returned for a description written for a newer Vigie.
	ErrNewerVersion = errors.New("written for a newer Vigie")
	// ErrInvalid is returned for a description that does not hold together.
	ErrInvalid = errors.New("invalid description of an add-on")
)

// Schema is the JSON Schema of version 1, checked against the types of this package.
//
//go:embed info-v1.schema.json
var Schema []byte

// Text is a text in each language the add-on gives: "en" always, "fr" when it has one. Vigie
// shows the user's language, else English.
type Text map[string]string

// In is the text in a language, else in English.
func (t Text) In(lang string) string {
	if s, ok := t[lang]; ok && s != "" {
		return s
	}
	return t["en"]
}

// Info describes an add-on.
type Info struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	// Name is the program's name after "vigie-": "example" for vigie-example.
	Name    string `json:"name"`
	Title   Text   `json:"title"`
	Author  string `json:"author"`
	Release string `json:"release"` // the add-on's own version, "0.1.0"
	Kind    string `json:"kind"`
	// Summary is a sentence for its card, Description the text of its page.
	Summary     Text     `json:"summary"`
	Description Text     `json:"description"`
	Details     []Detail `json:"details"`
	// Image is a data URI, PNG or SVG, MaxImage at most decoded; "" shows Vigie's default image.
	Image    string    `json:"image,omitempty"`
	Settings []Setting `json:"settings"`
}

// Detail is a fact of the add-on's page: what it reads, writes, connects to.
type Detail struct {
	Label Text `json:"label"`
	Value Text `json:"value"`
}

// Setting is something a run asks for, passed to the program as Flag and its value (a checkbox:
// Flag alone when checked).
type Setting struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Label    Text     `json:"label"`
	Help     Text     `json:"help,omitempty"`
	Flag     string   `json:"flag"`
	Default  string   `json:"default"`
	Required bool     `json:"required"`
	Choices  []Choice `json:"choices,omitempty"` // for a choice
}

// Choice is one value of a choice.
type Choice struct {
	Value string `json:"value"`
	Label Text   `json:"label"`
}

// Write writes a description, indented.
func Write(w io.Writer, i Info) error {
	i.Format, i.Version = Format, Version
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(i)
}

// maxSize bounds what is read: an image and its texts.
const maxSize = 1 << 20

// Read reads a description and checks it. A field this Vigie does not know is refused.
func Read(r io.Reader) (Info, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return Info{}, err
	}
	if len(data) > maxSize {
		return Info{}, fmt.Errorf("larger than %d MB: %w", maxSize>>20, ErrInvalid)
	}
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&head); err != nil || head.Format != Format {
		return Info{}, ErrNotAnInfo
	}
	switch {
	case head.Version > Version:
		return Info{}, fmt.Errorf("version %d: %w", head.Version, ErrNewerVersion)
	case head.Version < 1:
		return Info{}, fmt.Errorf("version %d: %w", head.Version, ErrNotAnInfo)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var i Info
	if err := dec.Decode(&i); err != nil {
		return Info{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if dec.More() {
		return Info{}, fmt.Errorf("data after the description: %w", ErrInvalid)
	}
	return i, i.Check()
}

var (
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	flagPattern = regexp.MustCompile(`^--[a-z0-9][a-z0-9-]*$`)
)

// Check refuses a description that does not hold together: a name that is not a program's, a
// text without English, an unknown kind, an image too large or of another type, a setting
// without a flag of its own or using one of Vigie's.
func (i Info) Check() error {
	invalid := func(what string) error { return fmt.Errorf("%s: %w", what, ErrInvalid) }
	if !namePattern.MatchString(i.Name) {
		return invalid("name " + i.Name)
	}
	for what, t := range map[string]Text{"title": i.Title, "summary": i.Summary, "description": i.Description} {
		if strings.TrimSpace(t["en"]) == "" {
			return invalid(what + " in English")
		}
	}
	if i.Kind != KindSource && i.Kind != KindDestination {
		return invalid("kind " + i.Kind)
	}
	for _, d := range i.Details {
		if d.Label["en"] == "" || d.Value["en"] == "" {
			return invalid("detail in English")
		}
	}
	if i.Image != "" {
		if err := checkImage(i.Image); err != nil {
			return err
		}
	}
	ids, flags := map[string]bool{}, map[string]bool{}
	for _, s := range i.Settings {
		switch {
		case !idPattern.MatchString(s.ID) || ids[s.ID]:
			return invalid("setting id " + s.ID)
		case !flagPattern.MatchString(s.Flag) || s.Flag == InfoFlag || s.Flag == ExportFlag || flags[s.Flag]:
			return invalid("setting flag " + s.Flag)
		case s.Label["en"] == "":
			return invalid("setting " + s.ID + " label in English")
		}
		ids[s.ID], flags[s.Flag] = true, true
		switch s.Kind {
		case SettingFile, SettingText:
		case SettingCheckbox:
			if s.Default != "" && s.Default != "true" && s.Default != "false" {
				return invalid("checkbox " + s.ID + " default")
			}
		case SettingChoice:
			if len(s.Choices) == 0 {
				return invalid("choice " + s.ID + " without choices")
			}
			values := []string{}
			for _, c := range s.Choices {
				if c.Value == "" || c.Label["en"] == "" || slices.Contains(values, c.Value) {
					return invalid("choice " + s.ID)
				}
				values = append(values, c.Value)
			}
			if s.Default != "" && !slices.Contains(values, s.Default) {
				return invalid("choice " + s.ID + " default")
			}
		default:
			return invalid("setting kind " + s.Kind)
		}
	}
	return nil
}

// checkImage accepts a base64 data URI of a PNG or SVG image, MaxImage at most decoded.
func checkImage(uri string) error {
	head, data, ok := strings.Cut(strings.TrimPrefix(uri, "data:"), ",")
	mediaType, encoding, _ := strings.Cut(head, ";")
	if !strings.HasPrefix(uri, "data:") || !ok || encoding != "base64" || !slices.Contains(imageTypes, mediaType) {
		return fmt.Errorf("image: a base64 data URI of a PNG or SVG: %w", ErrInvalid)
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("image: %w", ErrInvalid)
	}
	if len(decoded) > MaxImage {
		return fmt.Errorf("image larger than %d KB: %w", MaxImage>>10, ErrInvalid)
	}
	return nil
}

// Args are the arguments of a run, the export's path and the values chosen in their settings'
// order: each value its own argument, a checkbox its flag alone when checked. A value not given
// takes its default; a required one without either is refused.
func (i Info) Args(exportPath string, values map[string]string) ([]string, error) {
	args := []string{ExportFlag, exportPath}
	for _, s := range i.Settings {
		v, ok := values[s.ID]
		if !ok {
			v = s.Default
		}
		switch {
		case s.Kind == SettingCheckbox:
			if v == "true" {
				args = append(args, s.Flag)
			}
		case v == "" && s.Required:
			return nil, fmt.Errorf("setting %s: a value is required: %w", s.ID, ErrInvalid)
		case v != "":
			if s.Kind == SettingChoice && !slices.ContainsFunc(s.Choices, func(c Choice) bool { return c.Value == v }) {
				return nil, fmt.Errorf("setting %s: %q is not a choice: %w", s.ID, v, ErrInvalid)
			}
			args = append(args, s.Flag, v)
		}
	}
	return args, nil
}
