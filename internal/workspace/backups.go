package workspace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/update"
)

// ErrBackupDays refuses a time to keep the backups that the settings do not offer.
var ErrBackupDays = errors.New("backups are kept 7, 30 or 90 days, or with no limit")

// BackupDayChoices are the days the backups of an update can stay on the server; 0 is no limit,
// the last 3 updates of a site only (ADR-0033).
var BackupDayChoices = []int{7, 30, 90, 0}

const (
	backupDaysSetting = "backups.days"
	defaultBackupDays = 30
)

// BackupDays returns the days the backups of an update stay on the server, 0 for no limit.
func (w *Workspace) BackupDays(ctx context.Context) (int, error) {
	value, ok, err := w.db.Setting(ctx, backupDaysSetting)
	if err != nil || !ok {
		return defaultBackupDays, err
	}
	days, err := strconv.Atoi(value)
	if err != nil || !slices.Contains(BackupDayChoices, days) {
		return defaultBackupDays, nil
	}
	return days, nil
}

// SetBackupDays sets the days the backups of an update stay on the server, 0 for no limit.
func (w *Workspace) SetBackupDays(ctx context.Context, days int) error {
	if !slices.Contains(BackupDayChoices, days) {
		return ErrBackupDays
	}
	return w.db.SetSetting(ctx, backupDaysSetting, strconv.Itoa(days))
}

// backupLimits are what an update's job needs to keep the backups in check: how long they stay,
// and the other sites of this Vigie on each host.
func (w *Workspace) backupLimits(ctx context.Context, tree *model.Tree) (time.Duration, func(model.Site) []model.ID, error) {
	days, err := w.BackupDays(ctx)
	if err != nil {
		return 0, nil, err
	}
	siblings := func(site model.Site) []model.ID {
		var ids []model.ID
		for _, s := range tree.SitesUnder(model.Node{Kind: model.KindHost, ID: site.HostID}) {
			if s.ID != site.ID {
				ids = append(ids, s.ID)
			}
		}
		return ids
	}
	return time.Duration(days) * 24 * time.Hour, siblings, nil
}

// runOn runs a command line on a host, within its limit of commands at a time.
func (w *Workspace) runOn(ctx context.Context, host model.Host, line string) (sshx.Result, error) {
	var res sshx.Result
	err := w.limiter.Do(ctx, host.ID.String(), maxConcurrent(host), func() error {
		var err error
		res, err = w.deps.Runner.Run(ctx, sshx.TargetFor(host), line)
		return err
	})
	return res, err
}

// RemoveOldBackups removes, on its server, a site's backups past the limits (ADR-0033), then reads
// what is left into the site's latest scan.
func (w *Workspace) RemoveOldBackups(ctx context.Context, siteID model.ID) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	site, ok := tree.Site(siteID)
	if !ok {
		return fmt.Errorf("site %s: %w", siteID, model.ErrNotFound)
	}
	host, _ := tree.Host(site.HostID)
	keep, _, err := w.backupLimits(ctx, tree)
	if err != nil {
		return err
	}
	line, err := remote.ListBackups(site.ID)
	if err != nil {
		return err
	}
	res, err := w.runOn(ctx, host, line)
	if err != nil {
		return err
	}
	if res.ExitCode == 0 {
		runs := remote.ParseBackups(res.Stdout)
		times := make([]time.Time, len(runs))
		for i, run := range runs {
			times[i], _ = time.Parse(remote.BackupTimeLayout, run)
		}
		for i, past := range update.Expired(times, w.deps.Now(), keep) {
			if !past {
				continue
			}
			line, err := remote.RemoveBackup(".vigie/backups/" + site.ID.String() + "/" + runs[i])
			if err != nil {
				return err
			}
			if res, err := w.runOn(ctx, host, line); err != nil {
				return err
			} else if res.ExitCode != 0 {
				return fmt.Errorf("%w: %s", ErrRemoveRefused, strings.TrimSpace(string(res.Stderr)))
			}
		}
	}
	line, err = remote.BackupSizes(site.ID)
	if err != nil {
		return err
	}
	if res, err = w.runOn(ctx, host, line); err != nil {
		return err
	}
	backups := []model.Backup{}
	for _, b := range remote.ParseBackupSizes(res.Stdout) {
		if at, err := time.Parse(remote.BackupTimeLayout, b.Run); err == nil {
			backups = append(backups, model.Backup{At: at, Bytes: b.Bytes})
		}
	}
	return w.db.SetLatestBackups(ctx, site.ID, backups)
}

// ErrRemoveRefused is a removal of backups the server refused; its message follows.
var ErrRemoveRefused = errors.New("the server did not remove the backups")
