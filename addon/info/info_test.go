package info_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/addon/info"
)

// svg is a fictional image for the tests.
var svg = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8"/></svg>`))

func sample() info.Info {
	return info.Info{
		Name: "example", Title: info.Text{"en": "Example", "fr": "Exemple"}, Author: "Someone", Release: "0.1.0",
		Kind:        info.KindSource,
		Summary:     info.Text{"en": "Brings hosts from a list."},
		Description: info.Text{"en": "Reads a list and writes a Vigie export.", "fr": "Lit une liste et écrit un export Vigie."},
		Details:     []info.Detail{{Label: info.Text{"en": "Network"}, Value: info.Text{"en": "Connects to nothing"}}},
		Image:       svg,
		Settings: []info.Setting{
			{ID: "file", Kind: info.SettingFile, Label: info.Text{"en": "File"}, Flag: "--file", Required: true},
			{ID: "all", Kind: info.SettingCheckbox, Label: info.Text{"en": "Every server"}, Flag: "--all", Default: "false"},
			{ID: "mode", Kind: info.SettingChoice, Label: info.Text{"en": "Mode"}, Flag: "--mode", Default: "fast",
				Choices: []info.Choice{{Value: "fast", Label: info.Text{"en": "Fast"}}, {Value: "full", Label: info.Text{"en": "Full"}}}},
		},
	}
}

func encode(t *testing.T, i info.Info) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := info.Write(&buf, i); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestADescriptionReadsBackAsItWasWritten(t *testing.T) {
	want := sample()
	got, err := info.Read(bytes.NewReader(encode(t, want)))
	if err != nil {
		t.Fatal(err)
	}
	want.Format, want.Version = info.Format, info.Version
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(Write(i)) =\n%+v\nwant\n%+v", got, want)
	}
	if got.Title.In("fr") != "Exemple" || got.Summary.In("fr") != "Brings hosts from a list." {
		t.Errorf("texts in French: %q, %q", got.Title.In("fr"), got.Summary.In("fr"))
	}
}

func TestADescriptionMustHoldTogether(t *testing.T) {
	big := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, info.MaxImage+1))
	for name, change := range map[string]func(*info.Info){
		"a name that is not a program's": func(i *info.Info) { i.Name = "Example/../x" },
		"a title without English":        func(i *info.Info) { i.Title = info.Text{"fr": "Exemple"} },
		"an unknown kind":                func(i *info.Info) { i.Kind = "filter" },
		"an image of another type":       func(i *info.Info) { i.Image = "data:text/html;base64,PGI+" },
		"an image not in base64":         func(i *info.Info) { i.Image = "data:image/svg+xml,<svg/>" },
		"an image too large":             func(i *info.Info) { i.Image = big },
		"a flag of Vigie's":              func(i *info.Info) { i.Settings[0].Flag = info.ExportFlag },
		"a flag twice":                   func(i *info.Info) { i.Settings[1].Flag = "--file" },
		"a flag without dashes":          func(i *info.Info) { i.Settings[0].Flag = "file" },
		"an id twice":                    func(i *info.Info) { i.Settings[1].ID = "file" },
		"an unknown setting kind":        func(i *info.Info) { i.Settings[0].Kind = "password" },
		"a choice without choices":       func(i *info.Info) { i.Settings[2].Choices = nil },
		"a default that is not a choice": func(i *info.Info) { i.Settings[2].Default = "slow" },
		"a checkbox default":             func(i *info.Info) { i.Settings[1].Default = "yes" },
	} {
		i := sample()
		i.Settings = slices.Clone(i.Settings)
		change(&i)
		if _, err := info.Read(bytes.NewReader(encode(t, i))); !errors.Is(err, info.ErrInvalid) {
			t.Errorf("%s: Read() = %v, want ErrInvalid", name, err)
		}
	}
	for name, data := range map[string]string{
		"not JSON":       "Usage: vigie-example [-o file]",
		"another format": `{"format": "vigie", "version": 1}`,
	} {
		if _, err := info.Read(strings.NewReader(data)); !errors.Is(err, info.ErrNotAnInfo) {
			t.Errorf("%s: Read() = %v, want ErrNotAnInfo", name, err)
		}
	}
	if _, err := info.Read(strings.NewReader(`{"format": "vigie-addon", "version": 2}`)); !errors.Is(err, info.ErrNewerVersion) {
		t.Errorf("a newer version: %v", err)
	}
}

// Each value is its own argument, after the export's path; a checkbox is its flag alone.
func TestTheArgumentsOfARun(t *testing.T) {
	i := sample()
	got, err := i.Args("/tmp/export.json", map[string]string{"file": "/Users/someone/a list.xml", "all": "true"})
	want := []string{"--vigie-export", "/tmp/export.json", "--file", "/Users/someone/a list.xml", "--all", "--mode", "fast"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Args() = %q, %v; want %q", got, err, want)
	}
	if _, err := i.Args("/tmp/export.json", nil); !errors.Is(err, info.ErrInvalid) {
		t.Errorf("a required value missing: %v", err)
	}
	if _, err := i.Args("/tmp/export.json", map[string]string{"file": "a.xml", "mode": "slow"}); !errors.Is(err, info.ErrInvalid) {
		t.Errorf("a value that is not a choice: %v", err)
	}
}

// The embedded JSON Schema describes exactly the fields written, and lists the package's values.
func TestTheSchemaDescribesTheFormat(t *testing.T) {
	type object struct {
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties json.RawMessage            `json:"additionalProperties"`
	}
	var schema struct {
		object
		Defs map[string]object `json:"$defs"`
	}
	if err := json.Unmarshal(info.Schema, &schema); err != nil {
		t.Fatal(err)
	}
	for name, typ := range map[string]reflect.Type{
		"":        reflect.TypeFor[info.Info](),
		"detail":  reflect.TypeFor[info.Detail](),
		"setting": reflect.TypeFor[info.Setting](),
		"choice":  reflect.TypeFor[info.Choice](),
	} {
		o := schema.object
		if name != "" {
			o = schema.Defs[name]
		}
		var fields, required, props []string
		for f := range typ.Fields() {
			tag, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			fields = append(fields, tag)
			if opts != "omitempty" {
				required = append(required, tag)
			}
		}
		for p := range o.Properties {
			props = append(props, p)
		}
		slices.Sort(fields)
		slices.Sort(props)
		slices.Sort(required)
		slices.Sort(o.Required)
		if !slices.Equal(fields, props) || !slices.Equal(required, o.Required) || string(o.AdditionalProperties) != "false" {
			t.Errorf("schema %q: properties %v, required %v; want %v, %v, nothing else allowed", name, props, o.Required, fields, required)
		}
	}
	var lists struct {
		Properties struct {
			Kind struct {
				Enum []string `json:"enum"`
			} `json:"kind"`
		} `json:"properties"`
		Defs struct {
			Setting struct {
				Properties struct {
					Kind struct {
						Enum []string `json:"enum"`
					} `json:"kind"`
				} `json:"properties"`
			} `json:"setting"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(info.Schema, &lists); err != nil {
		t.Fatal(err)
	}
	if got := lists.Properties.Kind.Enum; !slices.Equal(got, []string{info.KindSource, info.KindDestination}) {
		t.Errorf("schema kinds %q", got)
	}
	if got := lists.Defs.Setting.Properties.Kind.Enum; !slices.Equal(got, []string{info.SettingFile, info.SettingText, info.SettingCheckbox, info.SettingChoice}) {
		t.Errorf("schema setting kinds %q", got)
	}
}

// Whatever the bytes, Read answers without panicking, and what it accepts writes back the same.
func FuzzRead(f *testing.F) {
	var buf bytes.Buffer
	if err := info.Write(&buf, sample()); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := info.Read(bytes.NewReader(data))
		if err != nil {
			return
		}
		var again bytes.Buffer
		if err := info.Write(&again, got); err != nil {
			t.Fatal(err)
		}
		if back, err := info.Read(&again); err != nil || !reflect.DeepEqual(back, got) {
			t.Fatalf("an accepted description does not read back: %v", err)
		}
	})
}
