package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

func setAside(site model.Site, kind model.ItemKind, slug, version string) model.SetAside {
	return model.SetAside{SiteID: site.ID, Kind: kind, Slug: slug, Version: version, OperationID: model.NewID(),
		Reason: "home 500 (critical error of WordPress)", Created: t0}
}

// A version rolled back on its own is set aside on its site, one per item (ADR-0042, point 5).
func TestSetAsidesSavedAndReadBack(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a, b := withSites(t, db)
	themeA := setAside(a, model.ItemTheme, "astra", "4.9.0")
	pluginA := setAside(a, model.ItemPlugin, "acf-pro", "6.9.0")
	pluginB := setAside(b, model.ItemPlugin, "acf-pro", "6.9.0")
	for _, s := range []model.SetAside{themeA, pluginA, pluginB} {
		must(t, db.SaveSetAside(ctx, s))
	}
	got, err := db.SetAsides(ctx, a.ID)
	must(t, err)
	if want := []model.SetAside{pluginA, themeA}; !reflect.DeepEqual(got, want) {
		t.Errorf("SetAsides(a) = %+v, want %+v", got, want)
	}

	// A newer version rolled back replaces the one set aside before.
	newer := setAside(a, model.ItemPlugin, "acf-pro", "6.9.1")
	newer.Created = t0.Add(24 * time.Hour)
	must(t, db.SaveSetAside(ctx, newer))
	got, err = db.SetAsides(ctx, a.ID)
	must(t, err)
	if want := []model.SetAside{newer, themeA}; !reflect.DeepEqual(got, want) {
		t.Errorf("after a newer one: %+v, want %+v", got, want)
	}

	must(t, db.DeleteSetAside(ctx, a.ID, model.ItemPlugin, "acf-pro"))
	got, err = db.SetAsides(ctx, a.ID)
	must(t, err)
	if want := []model.SetAside{themeA}; !reflect.DeepEqual(got, want) {
		t.Errorf("after the delete: %+v, want %+v", got, want)
	}
	if got, err := db.SetAsides(ctx, b.ID); err != nil || len(got) != 1 {
		t.Errorf("SetAsides(b) = %+v, %v; the other site keeps its own", got, err)
	}
}

func TestSetAsidesGoWithTheirSite(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a, _ := withSites(t, db)
	must(t, db.SaveSetAside(ctx, setAside(a, model.ItemPlugin, "acf-pro", "6.9.0")))
	must(t, db.DeleteSite(ctx, a.ID))
	if got, err := db.SetAsides(ctx, a.ID); err != nil || len(got) != 0 {
		t.Errorf("set-asides of a deleted site = %+v, %v", got, err)
	}
}

// Only a plugin or a theme rolls back on its own, and a set-aside names its version.
func TestSetAsideRefused(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a, _ := withSites(t, db)
	for name, s := range map[string]model.SetAside{
		"WordPress":    setAside(a, model.ItemCore, "wordpress", "7.1.4"),
		"translations": setAside(a, model.ItemLanguage, "fr_FR", "7.1.4"),
		"no slug":      setAside(a, model.ItemPlugin, "", "6.9.0"),
		"no version":   setAside(a, model.ItemPlugin, "acf-pro", ""),
	} {
		if err := db.SaveSetAside(ctx, s); !errors.Is(err, model.ErrInvalidSetAside) {
			t.Errorf("%s: SaveSetAside() = %v, want ErrInvalidSetAside", name, err)
		}
	}
	unknown := setAside(model.Site{ID: model.NewID()}, model.ItemPlugin, "acf-pro", "6.9.0")
	if err := db.SaveSetAside(ctx, unknown); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown site: SaveSetAside() = %v, want ErrNotFound", err)
	}
}
