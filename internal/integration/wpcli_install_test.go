//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/hostcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcliinstall"
)

// WP-CLI installed in the account's home over SSH (ADR-0039): the phar the bench itself installed
// from WP-CLI's release is the one Vigie pins; sent to ~/bin/wp, it answers, a second send leaves it
// alone, and the host's command ~/bin/wp then serves a connection test and a scan. A command that
// does not exist fails a scan as such.
func TestInstallWPCLIOverSSH(t *testing.T) {
	ctx := context.Background()
	a := sshAccess(t)
	r := a.runner(t, a.trusted(t))
	target := a.target(a.key)

	res, err := runAs(ctx, "cat", "/usr/local/bin/wp")
	if err != nil || res.exit != 0 {
		t.Fatal(err, res.stderr)
	}
	phar := []byte(res.stdout)
	if err := wpcliinstall.Verify(phar); err != nil {
		t.Fatalf("the bench's WP-CLI is not the one Vigie pins: %v", err)
	}
	t.Cleanup(func() { _, _ = runAs(ctx, "rm", "-f", "/home/vigie/bin/wp") })

	var sent int64
	if err := wpcliinstall.Send(ctx, r, target, phar, func(p wpcliinstall.Progress) {
		if p.Step == wpcliinstall.StepSend {
			sent = p.Done
		}
	}); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	if sent != int64(len(phar)) {
		t.Errorf("progress told %d bytes sent of %d", sent, len(phar))
	}
	answer, err := wpcliinstall.Test(ctx, r, target, func(wpcliinstall.Progress) {})
	if err != nil || answer != (wpcliinstall.Answer{Command: "~/bin/wp", Version: wpcliinstall.Version}) {
		t.Fatalf("Test() = %+v, %v", answer, err)
	}
	if err := wpcliinstall.Send(ctx, r, target, []byte("not wp"), func(wpcliinstall.Progress) {}); !errors.Is(err, wpcliinstall.ErrExists) {
		t.Fatalf("a second Send() = %v, want ErrExists", err)
	}
	if left, _ := runAs(ctx, "sh", "-c", "ls -A /home/vigie/bin; cmp -s /home/vigie/bin/wp /usr/local/bin/wp && echo same"); left.stdout != "wp\nsame\n" {
		t.Errorf("~/bin after the second send: %q", left.stdout)
	}

	report, err := hostcheck.Check(ctx, r, target, "~/bin/wp")
	if err != nil || report.WPCLI != "~/bin/wp" {
		t.Fatalf("Check(~/bin/wp) = %+v, %v", report, err)
	}

	s := scan.Scanner{Runner: r, Limiter: sshx.NewLimiter(), Now: timeNow}
	site := func() model.Site { return model.Site{ID: model.NewID(), Name: "plain", Path: "~/sites/plain"} }
	scanned, _ := s.Site(ctx, scan.Job{Site: site(), Target: target, HostKey: "bench", Max: 2, WPCLI: "~/bin/wp"}, nil)
	if scanned.Outcome != model.ScanCompleted {
		t.Fatalf("scan with ~/bin/wp: %q (%s)", scanned.Outcome, scanned.Error)
	}
	missing, _ := s.Site(ctx, scan.Job{Site: site(), Target: target, HostKey: "bench", Max: 2, WPCLI: "~/nowhere/wp"}, nil)
	if missing.Outcome != model.ScanFailed || missing.Failure != scan.FailureNoWPCLI {
		t.Fatalf("scan with a command that does not exist: %q, failure %q (%s)", missing.Outcome, missing.Failure, missing.Error)
	}
}
