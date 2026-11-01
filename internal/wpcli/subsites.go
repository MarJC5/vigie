package wpcli

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/sshx"
)

//go:embed subsites.php
var subsitesScript []byte

// SubsitesScript is the read-only script Subsites runs, sent on its standard input (ADR-0029).
func SubsitesScript() []byte {
	return bytes.Clone(subsitesScript)
}

// Subsites reads, in one go, what each subsite of a network has active and who administers it:
// WP-CLI runs the script it reads on its standard input, SubsitesScript.
func Subsites() Command {
	return Command{args: []string{"eval-file", "-"}, skipPlugins: true, skipThemes: true}
}

// Network is what the subsites script read of a network.
type Network struct {
	Plugins []string       // slugs of the plugins active network-wide
	Supers  []SubsiteAdmin // the super administrators
	Sites   []Subsite
}

// Subsite is one subsite, not archived, spam nor deleted.
type Subsite struct {
	ID                             int
	Plugins                        []string // slugs of the plugins active on it, besides the network's
	Theme, ThemeName, ThemeVersion string
	Email                          string // its admin email
	Admins                         []SubsiteAdmin
}

// SubsiteAdmin is an administrator of a subsite, with the two-factor methods they turned on.
type SubsiteAdmin struct {
	Account
	TwoFactor TwoFactor
}

// ParseSubsites reads Subsites' result.
func ParseSubsites(res sshx.Result) (Network, error) {
	if err := Check(res); err != nil {
		return Network{}, err
	}
	type account struct {
		ID         int             `json:"id"`
		Login      string          `json:"login"`
		Email      string          `json:"email"`
		Registered string          `json:"registered"`
		Providers  json.RawMessage `json:"providers"` // a list, one name, or "" when none
		WP2FA      string          `json:"wp2fa"`
	}
	var raw struct {
		Plugins []string  `json:"plugins"`
		Supers  []account `json:"supers"`
		Sites   []struct {
			ID      int      `json:"id"`
			Plugins []string `json:"plugins"`
			Theme   struct {
				Slug    string `json:"slug"`
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"theme"`
			Email  string    `json:"email"`
			Admins []account `json:"admins"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &raw); err != nil {
		return Network{}, fmt.Errorf("subsites: %w: %w", ErrUnexpectedOutput, err)
	}
	admin := func(a account) (SubsiteAdmin, error) {
		admin := SubsiteAdmin{Account: Account{ID: a.ID, Login: a.Login, Email: a.Email}, TwoFactor: TwoFactor{WP2FA: a.WP2FA}}
		if a.Registered != "" && a.Registered != "0000-00-00 00:00:00" {
			at, err := time.ParseInLocation(time.DateTime, a.Registered, time.UTC)
			if err != nil {
				return admin, fmt.Errorf("user %s registered %q: %w", a.Login, a.Registered, ErrUnexpectedOutput)
			}
			admin.Registered = at
		}
		var list []string
		var one string
		switch {
		case len(a.Providers) == 0 || string(a.Providers) == "null":
		case json.Unmarshal(a.Providers, &list) == nil:
			admin.TwoFactor.Providers = list
		case json.Unmarshal(a.Providers, &one) == nil:
			if one != "" {
				admin.TwoFactor.Providers = []string{one}
			}
		default:
			return admin, fmt.Errorf("two-factor providers of %s: %w", a.Login, ErrUnexpectedOutput)
		}
		return admin, nil
	}
	network := Network{Plugins: slugs(raw.Plugins), Supers: []SubsiteAdmin{}, Sites: []Subsite{}}
	for _, a := range raw.Supers {
		super, err := admin(a)
		if err != nil {
			return Network{}, err
		}
		network.Supers = append(network.Supers, super)
	}
	for _, s := range raw.Sites {
		if s.ID <= 0 {
			return Network{}, fmt.Errorf("subsite %d: %w", s.ID, ErrUnexpectedOutput)
		}
		site := Subsite{ID: s.ID, Plugins: slugs(s.Plugins), Theme: s.Theme.Slug, ThemeName: s.Theme.Name,
			ThemeVersion: s.Theme.Version, Email: s.Email, Admins: []SubsiteAdmin{}}
		for _, a := range s.Admins {
			admin, err := admin(a)
			if err != nil {
				return Network{}, err
			}
			site.Admins = append(site.Admins, admin)
		}
		network.Sites = append(network.Sites, site)
	}
	return network, nil
}

// slugs turns plugin files ("akismet/akismet.php", "hello.php") into the slugs WP-CLI names them by.
func slugs(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		if dir, _, ok := strings.Cut(f, "/"); ok {
			out = append(out, dir)
		} else {
			out = append(out, strings.TrimSuffix(path.Base(f), ".php"))
		}
	}
	return out
}
