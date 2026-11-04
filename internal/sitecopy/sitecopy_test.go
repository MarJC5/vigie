package sitecopy_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/wpcli"
)

var paths = wpcli.SitePaths{
	Root: "/home/u/www/site", Content: "/home/u/www/site/wp-content",
	Plugins: "/home/u/www/site/wp-content/plugins", MuPlugins: "/home/u/www/site/wp-content/mu-plugins",
	Themes: "/home/u/www/site/wp-content/themes", Uploads: "/home/u/www/site/wp-content/uploads", Prefix: "wp_",
}

func partsOf(ps []sitecopy.PartFolder) []sitecopy.Part {
	var out []sitecopy.Part
	for _, p := range ps {
		out = append(out, p.Part)
	}
	return out
}

// The parts of each content, and the folders each is read from (ADR-0041, point 2).
func TestThePartsOfEachContent(t *testing.T) {
	if got := sitecopy.Folders(paths, sitecopy.DatabaseOnly, nil); len(got) != 0 {
		t.Errorf("database only: folders %+v, want none", got)
	}
	content := sitecopy.Folders(paths, sitecopy.WithContent, []string{"cache", "backups-dup-lite"})
	if want := []sitecopy.Part{sitecopy.Plugins, sitecopy.Themes, sitecopy.MuPlugins, sitecopy.Uploads, sitecopy.Others}; !slices.Equal(partsOf(content), want) {
		t.Fatalf("database and wp-content: parts %v, want %v", partsOf(content), want)
	}
	others := content[4]
	if others.Dir != paths.Content {
		t.Errorf("others read from %s", others.Dir)
	}
	wantSkip := []string{paths.Plugins, paths.Themes, paths.MuPlugins, paths.Uploads, paths.Content + "/cache", paths.Content + "/backups-dup-lite"}
	if !slices.Equal(others.Skip, wantSkip) {
		t.Errorf("others skip %v, want %v", others.Skip, wantSkip)
	}
	site := sitecopy.Folders(paths, sitecopy.WholeSite, nil)
	last := site[len(site)-1]
	if last.Part != sitecopy.WordPress || last.Dir != paths.Root || !slices.Equal(last.Skip, []string{paths.Content}) {
		t.Errorf("whole site: last part %+v, want WordPress from the root without wp-content", last)
	}
}

// A folder of wp-content left out that is a part's leaves the part out.
func TestAPartLeftOut(t *testing.T) {
	got := partsOf(sitecopy.Folders(paths, sitecopy.WithContent, []string{"uploads"}))
	if slices.Contains(got, sitecopy.Uploads) {
		t.Errorf("uploads left out, parts %v", got)
	}
}

// A moved uploads folder, outside wp-content, is its own part and not skipped from others.
func TestAMovedUploadsFolder(t *testing.T) {
	moved := paths
	moved.Uploads = "/home/u/media"
	got := sitecopy.Folders(moved, sitecopy.WithContent, nil)
	if got[3].Part != sitecopy.Uploads || got[3].Dir != "/home/u/media" {
		t.Errorf("uploads part %+v", got[3])
	}
	if slices.Contains(got[4].Skip, "/home/u/media") {
		t.Errorf("others skips %v, a folder outside wp-content", got[4].Skip)
	}
}

func files(sizes ...int64) []remote.File {
	var fs []remote.File
	for i, n := range sizes {
		fs = append(fs, remote.File{Name: string(rune('a' + i)), Bytes: n})
	}
	return fs
}

func names(fs []remote.File) string {
	s := ""
	for _, f := range fs {
		s += f.Name
	}
	return s
}

// Files grouped in their order up to the limit; a file larger than the limit alone (point 3).
func TestFilesGroupedUpToTheLimit(t *testing.T) {
	groups := sitecopy.Group(files(200, 200, 150, 900, 50, 300), 500)
	var got []string
	for _, g := range groups {
		got = append(got, names(g))
	}
	if want := []string{"ab", "c", "d", "ef"}; !slices.Equal(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	if got := sitecopy.Group(nil, 500); len(got) != 0 {
		t.Errorf("no files: %d groups", len(got))
	}
}

func TestArchiveNames(t *testing.T) {
	for _, tt := range []struct {
		part sitecopy.Part
		i    int
		want string
	}{
		{sitecopy.Uploads, 0, "uploads.tar.gz"}, {sitecopy.Uploads, 1, "uploads-2.tar.gz"},
		{sitecopy.Database, 0, "database.sql.gz"}, {sitecopy.Database, 2, "database-3.sql.gz"},
		{sitecopy.MuPlugins, 0, "mu-plugins.tar.gz"}, {sitecopy.WordPress, 0, "wordpress.tar.gz"},
	} {
		if got := sitecopy.ArchiveName(tt.part, tt.i); got != tt.want {
			t.Errorf("ArchiveName(%s, %d) = %s, want %s", tt.part, tt.i, got, tt.want)
		}
	}
}

// The database is one dump, split by tables when larger than the limit.
func TestTheDatabaseSplitByTables(t *testing.T) {
	tables := []wpcli.Table{{Name: "wp_posts", Bytes: 700}, {Name: "wp_options", Bytes: 100}, {Name: "wp_users", Bytes: 50}, {Name: "wp_postmeta", Bytes: 400}}
	if got := sitecopy.DatabaseArchives(tables, 2000); len(got) != 1 || got[0].Tables != nil || got[0].Name != "database.sql.gz" {
		t.Fatalf("under the limit: %+v, want one whole dump", got)
	}
	got := sitecopy.DatabaseArchives(tables, 500)
	var sets [][]string
	for _, a := range got {
		sets = append(sets, a.Tables)
	}
	want := [][]string{{"wp_posts"}, {"wp_options", "wp_users"}, {"wp_postmeta"}}
	if !reflect.DeepEqual(sets, want) || got[1].Name != "database-2.sql.gz" {
		t.Fatalf("split: %v (%s), want %v", sets, got[1].Name, want)
	}
	// A table whose name a dump would refuse: the database stays whole.
	odd := append(slices.Clone(tables), wpcli.Table{Name: "wp posts", Bytes: 10})
	if got := sitecopy.DatabaseArchives(odd, 500); len(got) != 1 || got[0].Tables != nil {
		t.Errorf("a table with a space: %+v, want one whole dump", got)
	}
}

// The plan of a copy: every archive of every part, in order; without GNU find or tar, one archive
// per part, its files not listed.
func TestThePlanOfACopy(t *testing.T) {
	listed := map[sitecopy.Part][]remote.File{
		sitecopy.Plugins: files(100, 100), sitecopy.Themes: files(40), sitecopy.MuPlugins: nil,
		sitecopy.Uploads: files(300, 200, 300), sitecopy.Others: files(10),
	}
	in := sitecopy.Inputs{
		Folders: sitecopy.Folders(paths, sitecopy.WithContent, nil), Listed: listed,
		Tables: []wpcli.Table{{Name: "wp_posts", Bytes: 50}}, Limit: 500, Split: true,
	}
	var got []string
	for _, a := range sitecopy.Plan(in) {
		got = append(got, a.Name)
	}
	want := []string{"database.sql.gz", "plugins.tar.gz", "themes.tar.gz", "uploads.tar.gz", "uploads-2.tar.gz", "others.tar.gz"}
	if !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v (an empty part has no archive)", got, want)
	}
	in.Split = false
	got = nil
	for _, a := range sitecopy.Plan(in) {
		if a.Files != nil {
			t.Errorf("%s: files listed without GNU find and tar", a.Name)
		}
		got = append(got, a.Name)
	}
	want = []string{"database.sql.gz", "plugins.tar.gz", "themes.tar.gz", "mu-plugins.tar.gz", "uploads.tar.gz", "others.tar.gz"}
	if !slices.Equal(got, want) {
		t.Fatalf("unsplit plan %v, want %v", got, want)
	}
}

// copy.json says what a copy holds, and is read back (point 5).
func TestTheManifestReadBack(t *testing.T) {
	at := time.Date(2026, 11, 3, 20, 41, 0, 0, time.UTC)
	m := sitecopy.Manifest{
		Vigie: "0.2.0", Site: "https://boulangerie-exemple.ch", Host: "Shared hosting", WordPress: "7.1.3", Prefix: "wp_",
		Content: sitecopy.WithContent, LeftOut: []string{"cache"}, Started: at,
		Archives: []sitecopy.Record{{Name: "database.sql.gz", Part: sitecopy.Database, Bytes: 3100000, SHA256: "ab12", Read: at}},
	}
	dir := t.TempDir()
	if err := sitecopy.WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	got, err := sitecopy.ReadManifest(dir)
	if err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("ReadManifest() = %+v, %v\nwant %+v", got, err, m)
	}
}

// A folder left out is a plain name of wp-content's folders: a path is ignored.
func TestOnlyPlainNamesAreLeftOut(t *testing.T) {
	others := sitecopy.Folders(paths, sitecopy.WithContent, []string{"..", "../..", "a/b", "", "cache"})[4]
	want := []string{paths.Plugins, paths.Themes, paths.MuPlugins, paths.Uploads, paths.Content + "/cache"}
	if !slices.Equal(others.Skip, want) {
		t.Errorf("skip %v, want %v", others.Skip, want)
	}
}
