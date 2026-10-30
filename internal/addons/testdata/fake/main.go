// Command fake stands for add-ons in the tests of package addons: it behaves after the name it is
// copied under (vigie-good, vigie-slow, vigie-fails, vigie-broken, vigie-other, vigie-nothing).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/addon/info"
)

func main() {
	name := strings.TrimPrefix(filepath.Base(os.Args[0]), "vigie-")
	describe := flag.Bool(strings.TrimPrefix(info.InfoFlag, "--"), false, "")
	output := flag.String(strings.TrimPrefix(info.ExportFlag, "--"), "", "")
	names := flag.String("names", "", "")
	flag.Parse()
	if *describe {
		switch name {
		case "broken":
			fmt.Println("Usage: vigie-broken [options]")
			return
		case "other":
			name = "someone-else"
		}
		err := info.Write(os.Stdout, info.Info{
			Name: name, Title: info.Text{"en": "Fake " + name}, Author: "Tests", Release: "0.0.1", Kind: info.KindSource,
			Summary: info.Text{"en": "Hosts by name."}, Description: info.Text{"en": "Writes one host per name."}, Details: []info.Detail{},
			Settings: []info.Setting{{ID: "names", Kind: info.SettingText, Label: info.Text{"en": "Names"}, Flag: "--names", Default: "shop"}},
		})
		if err != nil {
			os.Exit(1)
		}
		return
	}
	switch name {
	case "slow":
		time.Sleep(time.Minute)
	case "fails":
		fmt.Fprintln(os.Stderr, "vigie-fails: the list cannot be read")
		os.Exit(3)
	case "nothing":
		return
	}
	f := export.File{Exported: time.Now().UTC().Truncate(time.Second), App: "fake",
		Folders: []export.Folder{}, Hosts: []export.Host{}, Sites: []export.Site{}, Ignores: []export.Ignore{}}
	for i, n := range strings.Split(*names, ",") {
		f.Hosts = append(f.Hosts, export.Host{ID: uuid.NewV7().String(), Name: n, Alias: n, Color: "none", TrustedKeys: []export.HostKey{}, Position: i})
	}
	if err := export.WriteFile(*output, f); err != nil {
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "%d hosts.\n", len(f.Hosts))
}
