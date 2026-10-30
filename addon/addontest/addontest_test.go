package addontest_test

import (
	"strings"
	"testing"

	"github.com/MarJC5/vigie/addon/addontest"
)

// The checks an add-on runs in its own tests, on a fictional one.
func TestAnAddOnThatKeepsTheContract(t *testing.T) {
	program := addontest.Build(t, "./testdata/vigie-example")
	if i := addontest.Info(t, program); i.Name != "example" || len(i.Settings) != 1 {
		t.Errorf("description = %+v", i)
	}
	f, report := addontest.Export(t, program, map[string]string{"names": "shop,blog"})
	if len(f.Hosts) != 2 || f.Hosts[1].Alias != "blog" || !strings.Contains(report, "2 hosts.") {
		t.Errorf("export = %+v, report %q", f, report)
	}
}
