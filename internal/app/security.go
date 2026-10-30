package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// Security serves the user's choices about security scans: the lookups made from their machine,
// and the findings they set aside.
type Security struct {
	ws  *workspace.Workspace
	now func() time.Time
}

// NewSecurity returns the service; now dates the agreement and the ignores.
func NewSecurity(ws *workspace.Workspace, now func() time.Time) *Security {
	return &Security{ws: ws, now: now}
}

// LookupsView is the user's choices about the lookups.
type LookupsView struct {
	Agreed       string            `json:"agreed"` // RFC 3339; "" while not agreed
	Asked        string            `json:"asked"`  // the last "not now"
	Destinations []DestinationView `json:"destinations"`
}

// DestinationView is a service a scan may ask, and whether it is turned on.
type DestinationView struct {
	ID string `json:"id"` // wpvulnerability, wordpress.org, registry, site
	On bool   `json:"on"`
}

// Lookups returns the user's choices about the lookups.
func (s *Security) Lookups(ctx context.Context) (LookupsView, error) {
	ls, err := s.ws.LookupSettings(ctx)
	if err != nil {
		return LookupsView{}, err
	}
	v := LookupsView{Agreed: rfc3339(ls.Agreed), Asked: rfc3339(ls.Asked), Destinations: []DestinationView{}}
	for _, d := range workspace.Destinations {
		v.Destinations = append(v.Destinations, DestinationView{ID: string(d), On: !slices.Contains(ls.Off, d)})
	}
	return v, nil
}

// Agree stores the user's agreement, with the destinations they left on.
func (s *Security) Agree(ctx context.Context, on []string) *Problem {
	return ProblemOf(s.change(ctx, func(ls *workspace.LookupSettings) error {
		ls.Agreed, ls.Off = s.now(), nil
		for _, d := range workspace.Destinations {
			if !slices.Contains(on, string(d)) {
				ls.Off = append(ls.Off, d)
			}
		}
		for _, id := range on {
			if !slices.Contains(workspace.Destinations, lookup.Destination(id)) {
				return fmt.Errorf("destination %q: %w", id, workspace.ErrUnknownDestination)
			}
		}
		return nil
	}))
}

// NotNow remembers that the user did not agree yet: nothing is sent, and the question is not asked
// again at each scan.
func (s *Security) NotNow(ctx context.Context) *Problem {
	return ProblemOf(s.change(ctx, func(ls *workspace.LookupSettings) error {
		ls.Asked = s.now()
		return nil
	}))
}

// Withdraw removes the agreement: no lookup is made any more.
func (s *Security) Withdraw(ctx context.Context) *Problem {
	return ProblemOf(s.change(ctx, func(ls *workspace.LookupSettings) error {
		ls.Agreed = time.Time{}
		return nil
	}))
}

// SetDestination turns a destination on or off.
func (s *Security) SetDestination(ctx context.Context, id string, on bool) *Problem {
	return ProblemOf(s.change(ctx, func(ls *workspace.LookupSettings) error {
		d := lookup.Destination(id)
		ls.Off = slices.DeleteFunc(ls.Off, func(o lookup.Destination) bool { return o == d })
		if !on {
			ls.Off = append(ls.Off, d)
		}
		return nil
	}))
}

func (s *Security) change(ctx context.Context, fn func(*workspace.LookupSettings) error) error {
	ls, err := s.ws.LookupSettings(ctx)
	if err != nil {
		return err
	}
	if err := fn(&ls); err != nil {
		return err
	}
	return s.ws.SetLookupSettings(ctx, ls)
}

// Ignore sets a finding of a site aside: until the version given changes, or permanently ("")
// with a reason.
func (s *Security) Ignore(ctx context.Context, siteID, key, untilVersion, reason string) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(fmt.Errorf("site %q: %w", siteID, errNodeNotFound))
	}
	return ProblemOf(s.ws.DB().SaveIgnore(ctx, model.Ignore{
		SiteID: id, Key: key, UntilVersion: untilVersion, Reason: strings.TrimSpace(reason), Created: s.now(),
	}))
}

// Unignore puts a finding of a site back in its status.
func (s *Security) Unignore(ctx context.Context, siteID, key string) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(fmt.Errorf("site %q: %w", siteID, errNodeNotFound))
	}
	return ProblemOf(s.ws.DB().DeleteIgnore(ctx, id, key))
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
