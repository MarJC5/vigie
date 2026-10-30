package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// pluginInformation is wordpress.org's plugin API, authoritative for closed plugins (ADR-0008).
const pluginInformation = "https://api.wordpress.org/plugins/info/1.2/"

// A day per slug (ADR-0014).
const wordpressOrgAge = 24 * time.Hour

// unusedFields are turned off in each request: the answer keeps what Vigie reads (version, last
// release, closure), 0.7 KB instead of 55.
var unusedFields = []string{
	"active_installs", "banners", "contributors", "description", "donate_link", "icons",
	"ratings", "reviews", "screenshots", "sections", "tags", "versions",
}

// PluginInfo is wordpress.org's view of a plugin.
type PluginInfo struct {
	OnWordPressOrg bool   // false for a premium or custom plugin
	Latest         string // the latest version published there
	LastUpdated    time.Time
	Closed         bool
	ClosedDate     string // YYYY-MM-DD
	ClosedReason   string // security-issue, author-request, guideline-violation...
	Fetched        time.Time
}

// Abandoned tells whether a plugin had no release on wordpress.org for two years (plan 05,
// decision 7); a closed plugin is reported as closed instead.
func (p PluginInfo) Abandoned(now time.Time) bool {
	return p.OnWordPressOrg && !p.Closed && !p.LastUpdated.IsZero() && p.LastUpdated.Before(now.AddDate(-2, 0, 0))
}

// PluginInfo asks wordpress.org about a plugin, by its slug.
func (c Client) PluginInfo(ctx context.Context, slug string) (PluginInfo, error) {
	if !slugPattern.MatchString(slug) {
		return PluginInfo{}, fmt.Errorf("plugin %q: %w", slug, ErrBadInput)
	}
	q := url.Values{"action": {"plugin_information"}, "request[slug]": {slug}}
	for _, f := range unusedFields {
		q.Set("request[fields]["+f+"]", "0")
	}
	answer, err := c.get(ctx, WordPressOrg, "plugin/"+slug, pluginInformation+"?"+q.Encode(), wordpressOrgAge)
	if err != nil {
		return PluginInfo{}, err
	}
	var body struct {
		Error       text `json:"error"`
		Version     text `json:"version"`
		LastUpdated text `json:"last_updated"`
		Closed      flag `json:"closed"`
		ClosedDate  text `json:"closed_date"`
		Reason      text `json:"reason"`
	}
	if err := json.Unmarshal(answer.Body, &body); err != nil {
		return PluginInfo{}, fmt.Errorf("wordpress.org answer about %s: %w", slug, err)
	}
	info := PluginInfo{Fetched: answer.Fetched}
	switch {
	case answer.Status == http.StatusOK:
		info.OnWordPressOrg, info.Latest = true, string(body.Version)
		if body.LastUpdated != "" {
			// "2026-10-21 3:54pm GMT"
			if info.LastUpdated, err = time.Parse("2006-01-02 3:04pm MST", string(body.LastUpdated)); err != nil {
				return PluginInfo{}, fmt.Errorf("last release of %s: %w", slug, err)
			}
			info.LastUpdated = info.LastUpdated.UTC()
		}
	case answer.Status == http.StatusNotFound && body.Error == "closed":
		info.OnWordPressOrg, info.Closed = true, true
		info.ClosedDate, info.ClosedReason = string(body.ClosedDate), string(body.Reason)
	case answer.Status == http.StatusNotFound:
		// Not on wordpress.org: a premium or custom plugin.
	default:
		return PluginInfo{}, fmt.Errorf("wordpress.org answers %d about %s", answer.Status, slug)
	}
	return info, nil
}
