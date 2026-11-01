package model_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		in, want string
		err      error
	}{
		{"Client Dupont", "Client Dupont", nil},
		{"  Contrat   de\tmaintenance ", "Contrat de maintenance", nil},
		{"Prioritaire", "Prioritaire", nil},
		{strings.Repeat("é", model.MaxTagLength), strings.Repeat("é", model.MaxTagLength), nil},
		{"", "", model.ErrEmptyTag},
		{"   ", "", model.ErrEmptyTag},
		{strings.Repeat("a", model.MaxTagLength+1), "", model.ErrTagTooLong},
		{"Client, Dupont", "", model.ErrTagCharacter},
		{"A|B", "", model.ErrTagCharacter},
		{"Line\nbreak", "Line break", nil},
		{"Bell\a", "", model.ErrTagCharacter},
	}
	for _, tt := range tests {
		got, err := model.NormalizeTag(tt.in)
		if !errors.Is(err, tt.err) || got != tt.want {
			t.Errorf("NormalizeTag(%q) = %q, %v; want %q, %v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

func TestTagKeyIgnoresCaseAndAccents(t *testing.T) {
	for _, pair := range [][2]string{{"Priorité", "PRIORITE"}, {"Église", "eglise"}, {"Zürich", "zurich"}} {
		if model.TagKey(pair[0]) != model.TagKey(pair[1]) {
			t.Errorf("TagKey(%q) = %q, TagKey(%q) = %q; want equal", pair[0], model.TagKey(pair[0]), pair[1], model.TagKey(pair[1]))
		}
	}
	if model.TagKey("Contrat") == model.TagKey("Contrats") {
		t.Error("TagKey: two different words compare equal")
	}
}

func TestSiteTagsKeepTheSpellingAlreadyUsed(t *testing.T) {
	used := []string{"Contrat de maintenance", "Prioritaire"}
	got, err := model.SiteTags([]string{"prioritaire", " Client Dupont ", "CONTRAT DE MAINTENANCE", "client dupont"}, used)
	want := []string{"Prioritaire", "Client Dupont", "Contrat de maintenance"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("SiteTags = %q, %v; want %q (the spelling used, each once, in the order given)", got, err, want)
	}
}

func TestSiteTagsRefuseABadTagOrTooMany(t *testing.T) {
	if _, err := model.SiteTags([]string{"Client", "a,b"}, nil); !errors.Is(err, model.ErrTagCharacter) {
		t.Errorf("SiteTags with a comma: %v, want ErrTagCharacter", err)
	}
	many := make([]string, model.MaxTags+1)
	for i := range many {
		many[i] = "tag " + strings.Repeat("x", i+1)
	}
	if _, err := model.SiteTags(many, nil); !errors.Is(err, model.ErrTooManyTags) {
		t.Errorf("SiteTags with %d tags: %v, want ErrTooManyTags", len(many), err)
	}
	if got, err := model.SiteTags(append(many[:model.MaxTags:model.MaxTags], "TAG X"), nil); err != nil || len(got) != model.MaxTags {
		t.Errorf("SiteTags with a repeat over the limit = %d tags, %v; repeats do not count", len(got), err)
	}
}

// taggedTree is two folders, a host in each, and tags across them.
func taggedTree(t *testing.T) *model.Tree {
	t.Helper()
	tr := model.NewTree()
	clients := model.Folder{ID: model.NewID(), Name: "Clients"}
	personal := model.Folder{ID: model.NewID(), Name: "Personal", Position: 1}
	shared := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Shared", Alias: "shared"}
	vps := model.Host{ID: model.NewID(), FolderID: personal.ID, Name: "VPS", Alias: "vps"}
	for _, err := range []error{
		tr.AddFolder(clients), tr.AddFolder(personal), tr.AddHost(shared), tr.AddHost(vps),
		tr.AddSite(model.Site{ID: model.NewID(), HostID: shared.ID, Name: "a.ch", Tags: []string{"Contrat", "Prioritaire"}}),
		tr.AddSite(model.Site{ID: model.NewID(), HostID: shared.ID, Name: "b.ch", Position: 1}),
		tr.AddSite(model.Site{ID: model.NewID(), HostID: vps.ID, Name: "c.ch", Tags: []string{"Contrat"}}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

func TestTheSitesOfATagAcrossFolders(t *testing.T) {
	tr := taggedTree(t)
	var names []string
	for _, s := range tr.SitesUnder(model.TagNode("CONTRAT")) {
		names = append(names, s.Name)
	}
	if strings.Join(names, " ") != "a.ch c.ch" {
		t.Fatalf("SitesUnder(tag) = %q, want the sites tagged, in tree order, case ignored", names)
	}
	if got := tr.PathOf(model.TagNode("Contrat")); got != "#Contrat" {
		t.Errorf("PathOf(tag) = %q, want #Contrat", got)
	}
	if len(tr.SitesUnder(model.TagNode("Nothing"))) != 0 {
		t.Error("SitesUnder(an unused tag) is not empty")
	}
}

func TestTheTagsInUseWithTheirSites(t *testing.T) {
	got := taggedTree(t).Tags()
	want := []model.TagCount{{Name: "Contrat", Sites: 2}, {Name: "Prioritaire", Sites: 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("Tags() = %+v, want %+v (by name, without case)", got, want)
	}
}
