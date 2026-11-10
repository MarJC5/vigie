package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// An assistant's read of a site's errors is kept in the journal, with its client's name, and stays
// there once the site is removed, by its path, as runs do.
func TestAnAssistantsReadOfErrorsIsKept(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "shop.example.test")
	at := time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)
	reads := []model.ErrorRead{
		{ID: model.NewID(), SiteID: site.ID, Path: "Clients/Shop/shop.example.test", Client: "Claude Desktop", At: at},
		{ID: model.NewID(), SiteID: site.ID, Path: "Clients/Shop/shop.example.test", At: at.Add(time.Hour)}, // no name given
	}
	for _, r := range slices.Backward(reads) {
		must(t, db.SaveErrorRead(ctx, r))
	}
	must(t, db.DeleteSite(ctx, site.ID))

	got, err := db.ErrorReads(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ErrorReads() = %+v, want both reads", got)
	}
	for i, r := range got { // oldest first
		if r.ID != reads[i].ID || r.SiteID != site.ID || r.Path != reads[i].Path || r.Client != reads[i].Client || !r.At.Equal(reads[i].At) {
			t.Errorf("ErrorReads()[%d] = %+v, want %+v", i, r, reads[i])
		}
	}
	if later, err := db.ErrorReads(ctx, at.Add(time.Minute)); err != nil || len(later) != 1 || later[0].ID != reads[1].ID {
		t.Errorf("ErrorReads(since a minute later) = %+v, %v; want the second read", later, err)
	}
}

// The journal of the reads has no column that could hold a text from the log: who, which site, when.
func TestTheReadsOfErrorsHoldNoTextFromTheLog(t *testing.T) {
	db := migrated(t)
	rows, err := db.SQL().QueryContext(context.Background(), "SELECT name FROM pragma_table_info('error_reads') ORDER BY cid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var name string
		must(t, rows.Scan(&name))
		columns = append(columns, name)
	}
	if want := []string{"id", "site_id", "path", "by_client", "read_at"}; !slices.Equal(columns, want) {
		t.Errorf("columns of error_reads = %q, want %q", columns, want)
	}
}
