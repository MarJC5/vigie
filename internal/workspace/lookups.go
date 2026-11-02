package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/store"
)

// The settings of the lookups made from the user's machine: when the user agreed to them, when
// they last answered "not now", and the destinations turned off, comma separated. Times are
// RFC 3339; unset until then.
const (
	lookupsAgreedSetting = "lookups.agreed"
	lookupsAskedSetting  = "lookups.asked"
	lookupsOffSetting    = "lookups.off"
)

// Destinations are the services Vigie may ask, in the order the interface lists them: those of a
// scan, then GitHub for Vigie's new versions (ADR-0038).
var Destinations = []lookup.Destination{lookup.WPVulnerability, lookup.WordPressOrg, lookup.Registry, lookup.Site, lookup.GitHub}

// LookupSettings are the user's choices about the lookups.
type LookupSettings struct {
	Agreed time.Time // zero: not agreed
	Asked  time.Time // the last "not now"; zero: never asked
	Off    []lookup.Destination
}

// LookupSettings reads the user's choices about the lookups.
func (w *Workspace) LookupSettings(ctx context.Context) (LookupSettings, error) {
	var ls LookupSettings
	for _, t := range []struct {
		key string
		at  *time.Time
	}{{lookupsAgreedSetting, &ls.Agreed}, {lookupsAskedSetting, &ls.Asked}} {
		value, _, err := w.db.Setting(ctx, t.key)
		if err != nil {
			return LookupSettings{}, err
		}
		if value == "" {
			continue
		}
		if *t.at, err = time.Parse(time.RFC3339, value); err != nil {
			return LookupSettings{}, fmt.Errorf("setting %s: %w", t.key, err)
		}
	}
	off, _, err := w.db.Setting(ctx, lookupsOffSetting)
	if err != nil {
		return LookupSettings{}, err
	}
	for _, d := range strings.Split(off, ",") {
		if slices.Contains(Destinations, lookup.Destination(d)) {
			ls.Off = append(ls.Off, lookup.Destination(d))
		}
	}
	return ls, nil
}

// SetLookupSettings stores the user's choices about the lookups; an unknown destination is refused.
func (w *Workspace) SetLookupSettings(ctx context.Context, ls LookupSettings) error {
	for _, d := range ls.Off {
		if !slices.Contains(Destinations, d) {
			return fmt.Errorf("destination %q: %w", d, ErrUnknownDestination)
		}
	}
	off := []string{}
	for _, d := range Destinations {
		if slices.Contains(ls.Off, d) {
			off = append(off, string(d))
		}
	}
	for key, value := range map[string]string{
		lookupsAgreedSetting: timeSetting(ls.Agreed), lookupsAskedSetting: timeSetting(ls.Asked), lookupsOffSetting: strings.Join(off, ","),
	} {
		if err := w.db.SetSetting(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// ErrUnknownDestination is returned for a destination Vigie does not ask.
var ErrUnknownDestination = errors.New("unknown destination")

func timeSetting(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// lookupTimeout bounds one outside request.
const lookupTimeout = 20 * time.Second

// newLookups is the client of the outside lookups, identified, with its answers kept in db, asking
// only the destinations allowed.
func newLookups(db *store.DB, version string, now func() time.Time, allowed func(lookup.Destination) bool) lookup.Client {
	return lookup.Client{
		HTTP: &http.Client{Timeout: lookupTimeout}, UserAgent: buildinfo.UserAgent(version),
		Cache: db, Now: now, Allowed: allowed,
	}
}

// LookupAllowed tells whether a destination may be asked: the user agreed to the lookups and did
// not turn this one off. Settings that cannot be read allow nothing.
func (w *Workspace) LookupAllowed(d lookup.Destination) bool {
	ls, err := w.LookupSettings(context.Background())
	return err == nil && !ls.Agreed.IsZero() && !slices.Contains(ls.Off, d)
}

// scanner runs the scans, with the outside lookups when the workspace has them.
func (w *Workspace) scanner() scan.Scanner {
	return scan.Scanner{Runner: w.deps.Runner, Limiter: w.limiter, Now: w.deps.Now, Lookups: w.deps.Lookups}
}
