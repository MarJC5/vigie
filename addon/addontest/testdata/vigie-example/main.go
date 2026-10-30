// Command vigie-example is a fictional add-on for the tests of addontest: a source that writes
// one host per name given.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/addon/info"
)

func main() {
	describe := flag.Bool(strings.TrimPrefix(info.InfoFlag, "--"), false, "print the description")
	output := flag.String(strings.TrimPrefix(info.ExportFlag, "--"), "", "write the export there")
	names := flag.String("names", "", "the hosts, comma separated")
	flag.Parse()
	if *describe {
		if err := info.Write(os.Stdout, info.Info{
			Name: "example", Title: info.Text{"en": "Example"}, Author: "Someone", Release: "0.1.0", Kind: info.KindSource,
			Summary: info.Text{"en": "Hosts by name."}, Description: info.Text{"en": "Writes one host per name given."},
			Details:  []info.Detail{},
			Settings: []info.Setting{{ID: "names", Kind: info.SettingText, Label: info.Text{"en": "Names"}, Flag: "--names", Required: true}},
		}); err != nil {
			os.Exit(1)
		}
		return
	}
	f := export.File{Exported: time.Now().UTC().Truncate(time.Second), App: "vigie-example 0.1.0",
		Folders: []export.Folder{}, Hosts: []export.Host{}, Sites: []export.Site{}, Ignores: []export.Ignore{}}
	for i, name := range strings.Split(*names, ",") {
		f.Hosts = append(f.Hosts, export.Host{ID: uuid.NewV7().String(), Name: name, Alias: name, Color: "none",
			TrustedKeys: []export.HostKey{}, Position: i})
	}
	if err := export.WriteFile(*output, f); err != nil {
		fmt.Fprintln(os.Stderr, "vigie-example:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "%d hosts.\n", len(f.Hosts))
}
