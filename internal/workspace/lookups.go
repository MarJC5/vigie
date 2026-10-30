package workspace

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/store"
)

// The settings of the lookups made from the user's machine: when the user agreed to them (unset
// until then), and the destinations turned off, comma separated.
const (
	lookupsAgreedSetting = "lookups.agreed"
	lookupsOffSetting    = "lookups.off"
)

// repository is where a service asked by Vigie can learn what it is.
const repository = "https://github.com/MarJC5/vigie"

// lookupTimeout bounds one outside request.
const lookupTimeout = 20 * time.Second

// newLookups is the client of the outside lookups, identified, with its answers kept in db, asking
// only the destinations allowed.
func newLookups(db *store.DB, version string, now func() time.Time, allowed func(lookup.Destination) bool) lookup.Client {
	return lookup.Client{
		HTTP: &http.Client{Timeout: lookupTimeout}, UserAgent: "Vigie/" + version + " (+" + repository + ")",
		Cache: db, Now: now, Allowed: allowed,
	}
}

// LookupAllowed tells whether a destination may be asked: the user agreed to the lookups and did
// not turn this one off. A setting that cannot be read allows nothing.
func (w *Workspace) LookupAllowed(d lookup.Destination) bool {
	ctx := context.Background()
	agreed, ok, err := w.db.Setting(ctx, lookupsAgreedSetting)
	if err != nil || !ok || agreed == "" {
		return false
	}
	off, _, err := w.db.Setting(ctx, lookupsOffSetting)
	return err == nil && !slices.Contains(strings.Split(off, ","), string(d))
}

// scanner runs the scans, with the outside lookups when the workspace has them.
func (w *Workspace) scanner() scan.Scanner {
	return scan.Scanner{Runner: w.deps.Runner, Limiter: w.limiter, Now: w.deps.Now, Lookups: w.deps.Lookups}
}
