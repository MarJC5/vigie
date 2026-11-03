package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// An install of WP-CLI is kept with its account, done or stopped at a step (ADR-0039).
func TestAWPCLIInstallIsKept(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	at := time.Date(2026, 11, 3, 14, 38, 0, 0, time.UTC)
	done := model.WPCLIInstall{ID: model.NewID(), Account: "u12ab@ssh.example-host.ch:22", HostName: "Shared hosting",
		Started: at, Finished: at.Add(9 * time.Second), Completed: true, Version: "2.12.0", Command: "~/bin/wp"}
	failed := model.WPCLIInstall{ID: model.NewID(), Account: "vps", HostName: "VPS", Started: at.Add(-time.Hour),
		Finished: at.Add(-time.Hour), Version: "2.12.0", FailedStep: "send", Message: "connection closed"}
	must(t, db.SaveWPCLIInstall(ctx, done))
	must(t, db.SaveWPCLIInstall(ctx, failed))

	got, err := db.WPCLIInstalls(ctx, at.Add(-2*time.Hour))
	if err != nil || len(got) != 2 {
		t.Fatalf("WPCLIInstalls() = %+v, %v", got, err)
	}
	if !got[0].Started.Equal(failed.Started) || got[0].Completed || got[0].FailedStep != "send" || got[0].Message != "connection closed" {
		t.Errorf("first = %+v, want the failed one, oldest first", got[0])
	}
	if !got[1].Completed || got[1].Command != "~/bin/wp" || got[1].Account != done.Account || !got[1].Finished.Equal(done.Finished) {
		t.Errorf("second = %+v", got[1])
	}
	if later, _ := db.WPCLIInstalls(ctx, at.Add(-time.Minute)); len(later) != 1 {
		t.Errorf("since a minute before: %d installs, want 1", len(later))
	}
	bad := done
	bad.ID, bad.FailedStep = model.NewID(), "elsewhere"
	if err := db.SaveWPCLIInstall(ctx, bad); err == nil {
		t.Error("an unknown step was stored")
	}
}
