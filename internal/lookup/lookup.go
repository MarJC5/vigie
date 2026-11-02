// Package lookup asks outside services, from the user's machine, what a security scan cannot read
// on the server (ADR-0014): known vulnerabilities, wordpress.org's view of a plugin, domain expiry,
// a site's certificate. Each request sends the least, is identified, and is kept in a cache; a
// destination the user turned off, or did not agree to, is never asked.
package lookup

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/MarJC5/vigie/internal/store"
)

// Destination is a service Vigie may ask; each can be turned off in the settings.
type Destination string

const (
	WPVulnerability Destination = "wpvulnerability"
	WordPressOrg    Destination = "wordpress.org"
	Registry        Destination = "registry" // the IANA bootstrap and the RDAP servers of the registries
	Site            Destination = "site"     // the site itself: its certificate, its debug.log
	// GitHub is where Vigie learns of its own new versions (ADR-0038), not asked by a scan.
	GitHub Destination = "github"
)

// ErrOff is returned for a destination the user turned off or has not agreed to yet: its checks
// are "not checked".
var ErrOff = errors.New("this lookup is turned off")

// Cache keeps the last answer of each lookup (store.DB).
type Cache interface {
	Lookup(ctx context.Context, source, key string) (store.LookupAnswer, bool, error)
	PutLookup(ctx context.Context, source, key string, a store.LookupAnswer) error
}

// maxBody bounds an answer read: the largest seen (a plugin with 43 vulnerabilities) is 130 KB.
const maxBody = 4 << 20

// Client makes the lookups.
type Client struct {
	HTTP      *http.Client
	UserAgent string // "Vigie/<version> (+<repository>)"
	Cache     Cache
	Now       func() time.Time
	// Allowed tells whether a destination may be asked: agreed to, and not turned off.
	Allowed func(Destination) bool
	// Dial reaches a site for its certificate; nil dials it directly.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// Roots are the authorities a certificate must chain to; nil uses the system's, as browsers do.
	Roots *x509.CertPool
}

// get answers from the cache while it is younger than maxAge, else asks url and keeps the answer.
// An answer that could not be fetched falls back on the cached one, however old: the caller shows
// its age.
func (c Client) get(ctx context.Context, dest Destination, key, url string, maxAge time.Duration) (store.LookupAnswer, error) {
	if c.Allowed == nil || !c.Allowed(dest) {
		return store.LookupAnswer{}, fmt.Errorf("%s: %w", dest, ErrOff)
	}
	cached, ok, err := c.Cache.Lookup(ctx, string(dest), key)
	if err != nil {
		return store.LookupAnswer{}, err
	}
	if ok && c.Now().Sub(cached.Fetched) < maxAge {
		return cached, nil
	}
	answer, err := c.fetch(ctx, url)
	if err != nil {
		if ok {
			return cached, nil
		}
		return store.LookupAnswer{}, err
	}
	if err := c.Cache.PutLookup(ctx, string(dest), key, answer); err != nil {
		return store.LookupAnswer{}, err
	}
	return answer, nil
}

// fetch makes one identified GET; any status is an answer (a 404 tells something too), except a
// server error.
func (c Client) fetch(ctx context.Context, url string) (store.LookupAnswer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return store.LookupAnswer{}, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return store.LookupAnswer{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return store.LookupAnswer{}, err
	}
	if res.StatusCode >= 500 {
		return store.LookupAnswer{}, fmt.Errorf("%s answers %d", req.URL.Host, res.StatusCode)
	}
	return store.LookupAnswer{Status: res.StatusCode, Body: body, Fetched: c.Now()}, nil
}
