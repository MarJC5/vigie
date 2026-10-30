package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// rdapBootstrap is IANA's list of the registries' RDAP servers (RFC 9224), kept a day as its
// cache-control says.
const (
	rdapBootstrap    = "https://data.iana.org/rdap/dns.json"
	rdapBootstrapAge = 24 * time.Hour
	rdapDomainAge    = 7 * 24 * time.Hour
)

// Why a domain's expiry is not read.
const (
	NotPublished = "not-published" // the registry publishes no expiry date: nothing is asked
	NoService    = "no-service"    // no RDAP server for the TLD in IANA's list
	NotFound     = "not-found"     // the registry does not know the domain
)

// silentRegistries publish no expiry date (checked on 2026-10-30): SWITCH for .ch and .li, whose
// server is not in IANA's list, and DENIC for .de.
var silentRegistries = map[string]bool{"ch": true, "li": true, "de": true}

// DomainExpiry is when a domain's registration ends, as its registry publishes it.
type DomainExpiry struct {
	Domain  string    // the registered domain asked about
	Expires time.Time // zero unless Reason is empty
	Reason  string    // empty when read, else NotPublished, NoService or NotFound
	Fetched time.Time
}

// RegisteredDomain is the domain a registry knows for a host: the longest ICANN suffix and one
// label more ("misits.ch" for "streamsafe.misits.ch", "example.co.uk"), in its ASCII form.
func RegisteredDomain(host string) (string, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil || ascii == "" || net.ParseIP(ascii) != nil || !strings.Contains(ascii, ".") {
		return "", fmt.Errorf("host %q: %w", host, ErrBadInput)
	}
	labels := strings.Split(ascii, ".")
	// The longest suffix that ICANN manages; a private one (github.io) belongs to a registered
	// domain the registry knows.
	suffix := len(labels) - 1
	for i := len(labels) - 1; i >= 1; i-- {
		candidate := strings.Join(labels[i:], ".")
		if ps, icann := publicsuffix.PublicSuffix(candidate); icann && ps == candidate {
			suffix = i
		}
	}
	if suffix == 0 {
		return "", fmt.Errorf("host %q is a public suffix: %w", host, ErrBadInput)
	}
	return strings.Join(labels[suffix-1:], "."), nil
}

// DomainExpiry asks the registry of a host's domain when its registration ends; only the
// registered domain is sent, and nothing for a registry that publishes no date.
func (c Client) DomainExpiry(ctx context.Context, host string) (DomainExpiry, error) {
	domain, err := RegisteredDomain(host)
	if err != nil {
		return DomainExpiry{}, err
	}
	result := DomainExpiry{Domain: domain}
	tld := domain[strings.LastIndex(domain, ".")+1:]
	if silentRegistries[tld] {
		result.Reason = NotPublished
		return result, nil
	}
	base, err := c.rdapServer(ctx, tld)
	if err != nil {
		return DomainExpiry{}, err
	}
	if base == "" {
		result.Reason = NoService
		return result, nil
	}
	answer, err := c.get(ctx, Registry, "domain/"+domain, base+"domain/"+domain, rdapDomainAge)
	if err != nil {
		return DomainExpiry{}, err
	}
	result.Fetched = answer.Fetched
	switch answer.Status {
	case http.StatusOK:
	case http.StatusNotFound:
		result.Reason = NotFound
		return result, nil
	default:
		return DomainExpiry{}, fmt.Errorf("registry answers %d about %s", answer.Status, domain)
	}
	var body struct {
		Events []struct {
			Action string `json:"eventAction"`
			Date   string `json:"eventDate"`
		} `json:"events"`
	}
	if err := json.Unmarshal(answer.Body, &body); err != nil {
		return DomainExpiry{}, fmt.Errorf("registry answer about %s: %w", domain, err)
	}
	for _, e := range body.Events {
		if e.Action != "expiration" {
			continue
		}
		if result.Expires, err = time.Parse(time.RFC3339, e.Date); err != nil {
			return DomainExpiry{}, fmt.Errorf("expiry of %s: %w", domain, err)
		}
		result.Expires = result.Expires.UTC()
		return result, nil
	}
	result.Reason = NotPublished
	return result, nil
}

// rdapServer returns the RDAP base URL of a TLD from IANA's list, or "" when it has none.
func (c Client) rdapServer(ctx context.Context, tld string) (string, error) {
	answer, err := c.get(ctx, Registry, "bootstrap", rdapBootstrap, rdapBootstrapAge)
	if err != nil {
		return "", err
	}
	var list struct {
		Services [][][]string `json:"services"`
	}
	if err := json.Unmarshal(answer.Body, &list); err != nil {
		return "", fmt.Errorf("IANA RDAP list: %w", err)
	}
	for _, service := range list.Services {
		if len(service) != 2 || len(service[1]) == 0 {
			continue
		}
		for _, t := range service[0] {
			if t == tld {
				return httpsFirst(service[1]), nil
			}
		}
	}
	return "", nil
}

// httpsFirst picks an HTTPS base URL, with its trailing slash (RFC 9224, section 3).
func httpsFirst(urls []string) string {
	pick := urls[0]
	for _, u := range urls {
		if strings.HasPrefix(u, "https://") {
			pick = u
			break
		}
	}
	if !strings.HasSuffix(pick, "/") {
		pick += "/"
	}
	return pick
}
