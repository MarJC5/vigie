package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcliinstall"
)

// installBench is a workspace whose download of WP-CLI returns phar, or fails with err.
func installBench(t *testing.T, phar []byte, err error) (*workspace.Workspace, *sshxtest.Fake, func() []model.WPCLIInstall) {
	t.Helper()
	db := emptyDB(t)
	fake := sshxtest.NewFake()
	ws := workspace.New(db, workspace.Deps{
		Runner: fake, Now: time.Now,
		DownloadWPCLI: func(_ context.Context, progress func(wpcliinstall.Progress)) ([]byte, error) {
			progress(wpcliinstall.Progress{Step: wpcliinstall.StepDownload})
			if err != nil {
				return nil, err
			}
			progress(wpcliinstall.Progress{Step: wpcliinstall.StepCheck})
			return phar, nil
		},
	})
	installs := func() []model.WPCLIInstall {
		got, err := db.WPCLIInstalls(context.Background(), time.Now().Add(-time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	return ws, fake, installs
}

func command(t *testing.T, make func() (string, error)) string {
	t.Helper()
	r, err := make()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// An install on a stored host: the phar sent, the command run, the host's command set, the
// install kept for the journal.
func TestInstallWPCLIOnAStoredHost(t *testing.T) {
	phar := []byte("<?php the phar")
	ws, fake, installs := installBench(t, phar, nil)
	ctx := context.Background()
	host := model.Host{ID: model.NewID(), Name: "Shared hosting", Hostname: "ssh.example.test", User: "u12ab", WPCLI: "wp"}
	if err := ws.DB().SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	send := command(t, wpcliinstall.SendCommand)
	fake.Set(send, sshx.Result{})
	info := command(t, func() (string, error) { return wpcli.CLIInfo().Remote(wpcliinstall.Command, "") })
	fake.Set(info, sshx.Result{Stdout: []byte(`{"php_version":"8.4.27","wp_cli_version":"2.12.0"}`)})

	var steps []wpcliinstall.Step
	answer, failed, err := ws.InstallWPCLI(ctx, host, true, func(p wpcliinstall.Progress) { steps = append(steps, p.Step) })
	if err != nil || failed != "" || answer.Command != "~/bin/wp" {
		t.Fatalf("InstallWPCLI = %+v, %q, %v", answer, failed, err)
	}
	if string(fake.Input(send)) != string(phar) {
		t.Errorf("sent %q", fake.Input(send))
	}
	if steps[len(steps)-1] != wpcliinstall.StepTest {
		t.Errorf("steps %q", steps)
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := tree.Host(host.ID); h.WPCLI != "~/bin/wp" {
		t.Errorf("the host's command is %q", h.WPCLI)
	}
	got := installs()
	if len(got) != 1 || !got[0].Completed || got[0].Account != "u12ab@ssh.example.test:22" || got[0].HostName != "Shared hosting" || got[0].Command != "~/bin/wp" {
		t.Errorf("kept %+v", got)
	}
}

// A failed install says at which step, keeps that for the journal, and changes no host.
func TestInstallWPCLIFailures(t *testing.T) {
	host := model.Host{ID: model.NewID(), Name: "VPS", Alias: "vps", WPCLI: "wp"}

	ws, _, installs := installBench(t, nil, wpcliinstall.ErrDigest)
	_, failed, err := ws.InstallWPCLI(context.Background(), host, false, nil)
	if !errors.Is(err, wpcliinstall.ErrDigest) || failed != wpcliinstall.StepDownload {
		t.Fatalf("a refused download: %q, %v", failed, err)
	}
	if got := installs(); len(got) != 1 || got[0].Completed || got[0].FailedStep != "download" || got[0].Account != "vps" {
		t.Errorf("kept %+v", got)
	}

	ws, fake, installs := installBench(t, []byte("phar"), nil)
	fake.Set(command(t, wpcliinstall.SendCommand), sshx.Result{ExitCode: 3})
	_, failed, err = ws.InstallWPCLI(context.Background(), host, false, nil)
	if !errors.Is(err, wpcliinstall.ErrExists) || failed != wpcliinstall.StepSend {
		t.Fatalf("~/bin/wp there: %q, %v", failed, err)
	}
	if got := installs(); len(got) != 1 || got[0].FailedStep != "send" || got[0].Message == "" {
		t.Errorf("kept %+v", got)
	}
}
