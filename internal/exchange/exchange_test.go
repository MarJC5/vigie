package exchange_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
)

// sample is a fictional export with every part filled.
func sample() exchange.File {
	at := time.Date(2026, 10, 30, 14, 12, 0, 0, time.UTC)
	done := at.Add(31 * time.Second)
	return exchange.File{
		Exported: at, App: "0.6.0", Scope: "",
		Folders: []exchange.Folder{
			{ID: "01a18cd5-0000-7000-8000-000000000001", Name: "Clients", Color: "blue", Notes: "**Contrats** à jour"},
			{ID: "01a18cd5-0000-7000-8000-000000000002", Parent: "01a18cd5-0000-7000-8000-000000000001", Name: "Valais", Color: "none", Position: 1},
		},
		Hosts: []exchange.Host{{
			ID: "01a18cd5-0000-7000-8000-000000000003", Folder: "01a18cd5-0000-7000-8000-000000000002", Name: "Shared hosting",
			Hostname: "ssh.example-host.ch", Port: 22, User: "u12ab", WPCLI: "wp-cli", MaxConcurrent: 2, Color: "orange",
			TrustedKeys: []exchange.HostKey{{Name: "ssh.example-host.ch", Type: "ssh-ed25519", Key: "AAAAC3NzaC1lZDI1NTE5AAAAIFictionalKeyForTestsOnly0000000000000"}},
		}},
		Sites: []exchange.Site{{
			ID: "01a18cd5-0000-7000-8000-000000000004", Host: "01a18cd5-0000-7000-8000-000000000003", Name: "atelier-exemple.ch",
			Path: "~/sites/atelier", URL: "https://atelier-exemple.ch", Color: "none", Tags: []string{"client"}, Multisite: true,
			Subsites: []exchange.Subsite{{BlogID: 1, URL: "https://atelier-exemple.ch", Public: true}},
		}},
		Ignores: []exchange.Ignore{{Site: "01a18cd5-0000-7000-8000-000000000004", Key: "file-editing|site|", Reason: "staging copy", Created: at}},
		History: &exchange.History{
			Scans: []exchange.Scan{{
				ID: "01a18cd5-0000-7000-8000-000000000005", Site: "01a18cd5-0000-7000-8000-000000000004", Started: at, Finished: done,
				Outcome: "completed", WordPress: "7.1.4", FilesChecked: &done,
				Items:    []exchange.Item{{Kind: "plugin", Slug: "akismet", Version: "5.7.2", Update: "none", OnWordPressOrg: true}},
				Findings: []exchange.Finding{{Kind: "file-editing", Subject: "site", Level: "attention", Facts: map[string]string{}}},
			}},
			Operations: []exchange.Operation{{
				ID: "01a18cd5-0000-7000-8000-000000000006", Site: "01a18cd5-0000-7000-8000-000000000004", Kind: "update",
				Started: at, Finished: &done, Outcome: "completed", Summary: "1 of 1",
				Steps: []exchange.Step{{Step: "update", ItemKind: "plugin", Slug: "akismet", VersionBefore: "5.7.1", VersionAfter: "5.7.2", Started: at, Finished: &done, Outcome: "done"}},
			}},
		},
	}
}

func encode(t *testing.T, f exchange.File) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := exchange.Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAnExportReadsBackAsItWasWritten(t *testing.T) {
	want := sample()
	data := encode(t, want)
	got, err := exchange.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want.Format, want.Version = exchange.Format, exchange.Version
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(Write(f)) =\n%+v\nwant\n%+v", got, want)
	}
	if !strings.HasPrefix(string(data), "{\n  \"format\": \"vigie\",\n  \"version\": 1,") {
		t.Errorf("an export starts with its format and version, indented:\n%.80s", data)
	}
}

func TestWhatIsNotAnExportOrTooNew(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		want       error
	}{
		{"not JSON", "<?xml version=\"1.0\"?><FileZilla3/>", exchange.ErrNotAnExport},
		{"another format", `{"format": "other", "version": 1}`, exchange.ErrNotAnExport},
		{"no version", `{"format": "vigie"}`, exchange.ErrNotAnExport},
		{"newer", `{"format": "vigie", "version": 2, "folders": []}`, exchange.ErrNewerVersion},
		{"a field this Vigie does not know", `{"format": "vigie", "version": 1, "folders": [], "groups": []}`, exchange.ErrInvalid},
		{"two objects", `{"format": "vigie", "version": 1} {}`, exchange.ErrInvalid},
	} {
		if _, err := exchange.Read(strings.NewReader(tc.data)); !errors.Is(err, tc.want) {
			t.Errorf("%s: Read() = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestAnExportMustHoldTogether(t *testing.T) {
	for name, change := range map[string]func(*exchange.File){
		"an ID that is not one":     func(f *exchange.File) { f.Sites[0].ID = "atelier" },
		"an ID twice":               func(f *exchange.File) { f.Hosts[0].ID = f.Folders[0].ID },
		"a parent not in the file":  func(f *exchange.File) { f.Folders[1].Parent = "01a18cd5-0000-7000-8000-0000000000ff" },
		"a host in no folder of it": func(f *exchange.File) { f.Hosts[0].Folder = "01a18cd5-0000-7000-8000-0000000000ff" },
		"a site without its host":   func(f *exchange.File) { f.Sites[0].Host = "01a18cd5-0000-7000-8000-0000000000ff" },
		"a folder without a name":   func(f *exchange.File) { f.Folders[0].Name = "" },
		"folders in a loop":         func(f *exchange.File) { f.Folders[0].Parent = f.Folders[1].ID },
		"an ignore of another site": func(f *exchange.File) { f.Ignores[0].Site = "01a18cd5-0000-7000-8000-0000000000ff" },
		"a scan of another site":    func(f *exchange.File) { f.History.Scans[0].Site = "01a18cd5-0000-7000-8000-0000000000ff" },
		"an unknown color":          func(f *exchange.File) { f.Sites[0].Color = "purple" },
		"an unknown key type":       func(f *exchange.File) { f.Hosts[0].TrustedKeys[0].Type = "ssh-dss" },
		"an unknown level":          func(f *exchange.File) { f.History.Scans[0].Findings[0].Level = "critical" },
	} {
		f := sample()
		change(&f)
		if _, err := exchange.Read(bytes.NewReader(encode(t, f))); !errors.Is(err, exchange.ErrInvalid) {
			t.Errorf("%s: Read() = %v, want ErrInvalid", name, err)
		}
	}
	// A site without a path, as Vigie stores it, reads back.
	f := sample()
	f.Sites[0].Path = ""
	if _, err := exchange.Read(bytes.NewReader(encode(t, f))); err != nil {
		t.Errorf("a site without a path = %v", err)
	}
	// Without history, the export of a host alone: its folder is outside the file.
	f = sample()
	f.History, f.Folders, f.Hosts[0].Folder, f.Scope = nil, nil, "", "Clients/Valais/Shared hosting"
	if _, err := exchange.Read(bytes.NewReader(encode(t, f))); err != nil {
		t.Errorf("the export of a host alone = %v", err)
	}
}

// An operation still running where it was exported runs nowhere on the machine that imports it.
func TestAnOperationStillRunningComesBackInterrupted(t *testing.T) {
	op := sample().History.Operations[0]
	op.Outcome, op.Finished = "running", nil
	if got := op.Model(model.NewID()); got.Outcome != model.OperationInterrupted || !got.Finished.IsZero() || len(got.Steps) != 1 {
		t.Errorf("Model() = %+v", got)
	}
}

// The JSON Schema in docs describes exactly the fields Vigie writes: same names, the optional
// ones optional, nothing else allowed.
func TestTheSchemaDescribesTheFormat(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "export-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		object
		Defs map[string]object `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for name, typ := range map[string]reflect.Type{
		"":          reflect.TypeFor[exchange.File](),
		"folder":    reflect.TypeFor[exchange.Folder](),
		"host":      reflect.TypeFor[exchange.Host](),
		"hostKey":   reflect.TypeFor[exchange.HostKey](),
		"site":      reflect.TypeFor[exchange.Site](),
		"subsite":   reflect.TypeFor[exchange.Subsite](),
		"ignore":    reflect.TypeFor[exchange.Ignore](),
		"history":   reflect.TypeFor[exchange.History](),
		"scan":      reflect.TypeFor[exchange.Scan](),
		"item":      reflect.TypeFor[exchange.Item](),
		"finding":   reflect.TypeFor[exchange.Finding](),
		"operation": reflect.TypeFor[exchange.Operation](),
		"step":      reflect.TypeFor[exchange.Step](),
	} {
		o := schema.object
		if name != "" {
			o = schema.Defs[name]
		}
		var fields, required []string
		for f := range typ.Fields() {
			tag, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			fields = append(fields, tag)
			if opts != "omitempty" {
				required = append(required, tag)
			}
		}
		var props []string
		for p := range o.Properties {
			props = append(props, p)
		}
		slices.Sort(fields)
		slices.Sort(props)
		slices.Sort(required)
		slices.Sort(o.Required)
		if !slices.Equal(fields, props) || !slices.Equal(required, o.Required) || o.AdditionalProperties == nil || *o.AdditionalProperties {
			t.Errorf("schema %q: properties %v, required %v; want %v, %v, nothing else allowed", name, props, o.Required, fields, required)
		}
	}
}

type object struct {
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

// Whatever the bytes, Read answers without panicking, and what it accepts writes back to
// something it reads the same.
func FuzzRead(f *testing.F) {
	var buf bytes.Buffer
	if err := exchange.Write(&buf, sample()); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	f.Add([]byte(`{"format":"vigie","version":1}`))
	f.Add([]byte(`{"format":"vigie","version":1,"folders":[{"id":"01a18cd5-0000-7000-8000-000000000001","parent":"01a18cd5-0000-7000-8000-000000000001","name":"x"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := exchange.Read(bytes.NewReader(data))
		if err != nil {
			return
		}
		var again bytes.Buffer
		if err := exchange.Write(&again, got); err != nil {
			t.Fatal(err)
		}
		back, err := exchange.Read(&again)
		if err != nil || !reflect.DeepEqual(back, got) {
			t.Fatalf("an accepted export does not read back: %v", err)
		}
	})
}
