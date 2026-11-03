package workspace

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcliinstall"
)

// ErrNoDownload is returned when the workspace has no means to download WP-CLI (a test's).
var ErrNoDownload = errors.New("no download of WP-CLI set")

// downloadTimeout bounds the download of WP-CLI on the user's machine (7 MB).
const downloadTimeout = 5 * time.Minute

// wpcliDownload downloads the pinned WP-CLI from its release, saying what Vigie is: the user's own
// action, not one of the lookups (ADR-0039).
func wpcliDownload(version string) func(ctx context.Context, progress func(wpcliinstall.Progress)) ([]byte, error) {
	client := &http.Client{Timeout: downloadTimeout}
	return func(ctx context.Context, progress func(wpcliinstall.Progress)) ([]byte, error) {
		return wpcliinstall.Download(ctx, client, wpcliinstall.URL, buildinfo.UserAgent(version), progress)
	}
}

// InstallWPCLI installs WP-CLI in the home of h's account (ADR-0039): downloaded and checked on
// this machine, sent to ~/bin/wp, then run once. The install is kept for the journal, done or
// failed; failed tells at which step. Once done, a stored host (stored true) gets the command
// that answered.
func (w *Workspace) InstallWPCLI(ctx context.Context, h model.Host, stored bool, progress func(wpcliinstall.Progress)) (answer wpcliinstall.Answer, failed wpcliinstall.Step, err error) {
	record := model.WPCLIInstall{ID: model.NewID(), Account: h.Account(), HostName: h.Name, Started: w.deps.Now(), Version: wpcliinstall.Version}
	step := wpcliinstall.StepDownload
	track := func(p wpcliinstall.Progress) {
		step = p.Step
		if progress != nil {
			progress(p)
		}
	}
	answer, err = w.installWPCLI(ctx, h, track)
	record.Finished = w.deps.Now()
	if err != nil {
		failed, record.FailedStep, record.Message = step, string(step), err.Error()
	} else {
		record.Completed, record.Command = true, answer.Command
	}
	// Kept even when the install was stopped: the journal says what was tried on the server.
	if serr := w.db.SaveWPCLIInstall(context.WithoutCancel(ctx), record); serr != nil && err == nil {
		return answer, "", serr
	}
	if err != nil || !stored {
		return answer, failed, err
	}
	err = w.edit(ctx, model.Node{Kind: model.KindHost, ID: h.ID}, nil, func(stored *model.Host) { stored.WPCLI = answer.Command }, nil)
	return answer, "", err
}

func (w *Workspace) installWPCLI(ctx context.Context, h model.Host, progress func(wpcliinstall.Progress)) (wpcliinstall.Answer, error) {
	if w.deps.DownloadWPCLI == nil {
		return wpcliinstall.Answer{}, ErrNoDownload
	}
	phar, err := w.deps.DownloadWPCLI(ctx, progress)
	if err != nil {
		return wpcliinstall.Answer{}, err
	}
	target := sshx.TargetFor(h)
	if err := wpcliinstall.Send(ctx, w.deps.Runner, target, phar, progress); err != nil {
		return wpcliinstall.Answer{}, err
	}
	return wpcliinstall.Test(ctx, w.deps.Runner, target, progress)
}
