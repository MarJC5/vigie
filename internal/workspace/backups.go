package workspace

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/MarJC5/vigie/internal/model"
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
