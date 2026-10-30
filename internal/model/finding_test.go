package model_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

func TestFindingKeyIsStableAcrossScans(t *testing.T) {
	a := model.Finding{Kind: "vulnerability", Subject: "plugin:woocommerce", Identifier: "uuid-1", Version: "9.8.1", Level: model.LevelUrgent}
	b := a
	b.Version, b.Facts = "9.8.2", map[string]string{"cvss": "9.8"}
	if a.Key() != b.Key() {
		t.Errorf("Key() changes with the version or the facts: %q, %q", a.Key(), b.Key())
	}
	c := a
	c.Identifier = "uuid-2"
	if a.Key() == c.Key() {
		t.Errorf("two vulnerabilities share the key %q", a.Key())
	}
}

func TestIgnoreUntilTheNextVersion(t *testing.T) {
	f := model.Finding{Kind: "vulnerability", Subject: "plugin:contact-form-7", Identifier: "uuid-3", Version: "6.0.4"}
	ignore := model.Ignore{Key: f.Key(), UntilVersion: "6.0.4"}
	if err := ignore.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: a reason is optional until the next version", err)
	}
	if !ignore.Covers(f) {
		t.Error("Covers() the version it was made for = false")
	}
	f.Version = "6.0.5"
	if ignore.Covers(f) {
		t.Error("Covers() another version = true, want the ignore to lapse")
	}
}

func TestIgnorePermanentlyNeedsAReason(t *testing.T) {
	f := model.Finding{Kind: "setting", Subject: "site", Identifier: "blog_public"}
	if err := (model.Ignore{Key: f.Key(), Reason: "  "}).Validate(); !errors.Is(err, model.ErrReasonRequired) {
		t.Fatalf("Validate() without a reason = %v, want ErrReasonRequired", err)
	}
	ignore := model.Ignore{Key: f.Key(), Reason: "staging copy, hidden on purpose"}
	if err := ignore.Validate(); err != nil {
		t.Fatal(err)
	}
	if !ignore.Covers(f) {
		t.Error("a permanent ignore does not cover its finding")
	}
	if (model.Ignore{Key: f.Key(), Reason: "x"}).Covers(model.Finding{Kind: "setting", Subject: "site", Identifier: "DISALLOW_FILE_EDIT"}) {
		t.Error("an ignore covers another finding")
	}
}

func TestIgnoresAreAppliedToTheirSiteOnly(t *testing.T) {
	site, other := model.NewID(), model.NewID()
	vuln := model.Finding{Kind: model.KindVulnerability, Subject: "plugin:akismet", Identifier: "u1", Version: "5.7.2", Level: model.LevelUrgent}
	files := model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention}
	s := model.Scan{SiteID: site, Outcome: model.ScanCompleted, Findings: []model.Finding{vuln, files}}
	s.ApplyIgnores([]model.Ignore{
		{SiteID: site, Key: vuln.Key(), UntilVersion: "5.7.2"},
		{SiteID: other, Key: files.Key(), Reason: "another site's"},
	})
	if s.Findings[0].Ignore == nil || s.Findings[1].Ignore != nil {
		t.Fatalf("ignored = %v, %v; want the vulnerability only", s.Findings[0].Ignore, s.Findings[1].Ignore)
	}
	if c := s.CountFindings(); c != (model.FindingCounts{Attention: 1}) {
		t.Errorf("CountFindings() = %+v, want the one not ignored", c)
	}
	// The next version: the ignore lapses.
	s.Findings[0].Version = "5.7.3"
	s.ApplyIgnores([]model.Ignore{{SiteID: site, Key: vuln.Key(), UntilVersion: "5.7.2"}})
	if s.Findings[0].Ignore != nil || s.Status() != model.StatusUrgent {
		t.Errorf("after the next version: ignored %v, status %q", s.Findings[0].Ignore, s.Status())
	}
}
