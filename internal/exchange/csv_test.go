package exchange_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/exchange"
)

// A list as a spreadsheet in French writes it: a byte order mark, semicolons, quotes around a
// note on two lines.
const frenchList = "\uFEFFfolder;Host;Port;User;Path;URL;Color;Notes\n" +
	"Clients/Valais;ssh.example.test;2222;u12ab;~/sites/atelier;https://atelier-exemple.ch;blue;\"contrat ; 2026\nrenouvelé\"\n" +
	"Clients/Valais;SSH.example.test;2222;u12ab;~/sites/cave/;https://cave-exemple.ch;;\n" +
	"Clients;ssh.example.test;;deploy;~/www;;;\n" +
	";ssh.example.test;2222;u12ab;~/sites/autre;;;\n" +
	"Clients;;;;~/www;;;\n" +
	"Clients;other.example.test;99999;wp;~/www;;;\n" +
	"Clients;other.example.test;22;wp;~/www;;purple;\n" +
	"Clients/Valais;ssh.example.test;2222;u12ab;~/sites/atelier/;;;\n" +
	"Clients;other.example.test;22\n"

func TestASiteListFromASpreadsheet(t *testing.T) {
	f, report, err := exchange.ReadCSV(strings.NewReader(frenchList))
	refused := report.Refused
	if err != nil {
		t.Fatal(err)
	}
	var folders, hosts, sites []string
	for _, x := range f.Folders {
		folders = append(folders, x.Name)
	}
	for _, x := range f.Hosts {
		hosts = append(hosts, x.Name+" "+x.User)
	}
	for _, x := range f.Sites {
		sites = append(sites, x.Name+" "+x.Color)
	}
	if !slices.Equal(folders, []string{"Clients", "Valais"}) || f.Folders[1].Parent != f.Folders[0].ID {
		t.Errorf("folders = %q", folders)
	}
	// One account for the first two lines, whatever the case of its hostname.
	if !slices.Equal(hosts, []string{"ssh.example.test u12ab", "ssh.example.test deploy"}) {
		t.Errorf("hosts = %q", hosts)
	}
	if !slices.Equal(sites, []string{"atelier-exemple.ch blue", "cave-exemple.ch none", "www none"}) {
		t.Errorf("sites = %q", sites)
	}
	if f.Sites[0].Notes != "contrat ; 2026\nrenouvelé" {
		t.Errorf("notes = %q", f.Sites[0].Notes)
	}
	// Lines counted in the file: the note takes two.
	want := []exchange.CSVRefusal{
		{Line: 6, Reason: exchange.CSVOtherFolder, Text: ";ssh.example.test;2222;u12ab;~/sites/autre;;;"},
		{Line: 7, Reason: exchange.CSVNoConnection, Text: "Clients;;;;~/www;;;"},
		{Line: 8, Reason: exchange.CSVBadPort, Value: "99999", Text: "Clients;other.example.test;99999;wp;~/www;;;"},
		{Line: 9, Reason: exchange.CSVBadColor, Value: "purple", Text: "Clients;other.example.test;22;wp;~/www;;purple;"},
		{Line: 10, Reason: exchange.CSVSameSite, Value: "~/sites/atelier/", Text: "Clients/Valais;ssh.example.test;2222;u12ab;~/sites/atelier/;;;"},
		{Line: 11, Reason: exchange.CSVColumns, Text: "Clients;other.example.test;22"},
	}
	if report.Lines != 9 || report.Separator != ";" {
		t.Errorf("report = %d lines, separator %q; want 9 and ;", report.Lines, report.Separator)
	}
	if !slices.Equal(refused, want) {
		t.Errorf("refused =\n%+v\nwant\n%+v", refused, want)
	}
	// What it gives is an export like any other.
	var buf bytes.Buffer
	if err := exchange.Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if _, err := exchange.Read(&buf); err != nil {
		t.Errorf("Read(the list as an export) = %v", err)
	}
}

func TestAListOfAliases(t *testing.T) {
	data := "alias,path,name\nshared,~/www/atelier,\nshared,~/www/cave,Cave\nstaging,~/www,\nother,,x\nweb,~/a,A/B\n"
	f, report, err := exchange.ReadCSV(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	refused := report.Refused
	if len(f.Hosts) != 2 || f.Hosts[0].Alias != "shared" || f.Hosts[0].Hostname != "" || len(f.Sites) != 3 || f.Sites[1].Name != "Cave" {
		t.Errorf("file = %+v", f)
	}
	want := []exchange.CSVRefusal{{Line: 5, Reason: exchange.CSVNoPath, Text: "other,,x"}, {Line: 6, Reason: exchange.CSVBadName, Value: "A/B", Text: "web,~/a,A/B"}}
	if !slices.Equal(refused, want) {
		t.Errorf("refused = %+v, want %+v", refused, want)
	}
}

func TestWhatIsNotASiteList(t *testing.T) {
	for name, data := range map[string]string{
		"empty":             "",
		"an unknown column": "host,user,path,password\n",
		"no path":           "host,user,url\n",
		"a host, no user":   "host,path\n",
		"a column twice":    "alias,path,path\n",
		"an open quote":     "alias,path\nshared,\"~/www\n",
	} {
		if _, _, err := exchange.ReadCSV(strings.NewReader(data)); !errors.Is(err, exchange.ErrNotACSV) {
			t.Errorf("%s: ReadCSV() = %v, want ErrNotACSV", name, err)
		}
	}
}

// Whatever the bytes, ReadCSV answers without panicking, and what it makes holds together.
func FuzzReadCSV(f *testing.F) {
	f.Add([]byte(frenchList))
	f.Add([]byte("alias,path\nshared,~/www\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		file, _, err := exchange.ReadCSV(bytes.NewReader(data))
		if err != nil {
			return
		}
		if err := file.Check(); err != nil {
			t.Fatalf("the export made from a list does not hold together: %v", err)
		}
	})
}
