package app

import (
	"context"
	"errors"

	"github.com/MarJC5/vigie/addon/info"
	"github.com/MarJC5/vigie/internal/addons"
)

// errNoAddOn is returned when the add-on asked for is no longer installed.
var errNoAddOn = errors.New("no add-on of this name is installed")

// Addons serves the Add-ons page: the add-ons installed and the Library, and the run of a source,
// whose export opens in the import preview of the Data service.
type Addons struct {
	data *Data
	pick FilePicker
	// folders is where add-ons are looked for (addons.Folders).
	folders func() string
}

// NewAddons returns the service; folders gives where to look for add-ons.
func NewAddons(data *Data, pick FilePicker, folders func() string) *Addons {
	return &Addons{data: data, pick: pick, folders: folders}
}

// AddonView is an add-on as the page shows it: installed, known from the Library, or both. Its
// texts are by language; the page shows the user's, else English.
type AddonView struct {
	Name        string            `json:"name"`
	Title       map[string]string `json:"title"`
	Author      string            `json:"author"`
	Release     string            `json:"release"`
	Kind        string            `json:"kind"`
	Summary     map[string]string `json:"summary"`
	Description map[string]string `json:"description"`
	Details     []DetailView      `json:"details"`
	Image       string            `json:"image"` // a data URI; "" for Vigie's default image
	Settings    []SettingView     `json:"settings"`
	Installed   bool              `json:"installed"`
	Path        string            `json:"path"`    // where it is installed
	Install     string            `json:"install"` // where to get it, for one of the Library
	Problem     *Problem          `json:"problem"` // an installed one whose description Vigie cannot use
}

// DetailView is a fact of an add-on's page.
type DetailView struct {
	Label map[string]string `json:"label"`
	Value map[string]string `json:"value"`
}

// SettingView is something a run asks for.
type SettingView struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Label    map[string]string `json:"label"`
	Help     map[string]string `json:"help"`
	Default  string            `json:"default"`
	Required bool              `json:"required"`
	Choices  []ChoiceView      `json:"choices"`
}

// ChoiceView is one value of a choice.
type ChoiceView struct {
	Value string            `json:"value"`
	Label map[string]string `json:"label"`
}

// AddonsView is the content of the page.
type AddonsView struct {
	Installed []AddonView `json:"installed"`
	Library   []AddonView `json:"library"`
	Problem   *Problem    `json:"problem"`
}

// List finds the add-ons installed, asks each for its description, and reads the Library, where
// the ones installed are marked.
func (a *Addons) List(ctx context.Context) AddonsView {
	view := AddonsView{Installed: []AddonView{}, Library: []AddonView{}}
	installed := map[string]bool{}
	for _, found := range addons.Find(ctx, a.folders()) {
		v := viewOf(found.Info)
		v.Name, v.Installed, v.Path, v.Problem = found.Name, true, found.Path, ProblemOf(found.Err)
		view.Installed = append(view.Installed, v)
		installed[found.Name] = found.Err == nil
	}
	known, err := addons.Library()
	if err != nil {
		view.Problem = ProblemOf(err)
		return view
	}
	for _, k := range known {
		v := viewOf(k.Info)
		v.Installed, v.Install = installed[k.Info.Name], k.Install
		view.Library = append(view.Library, v)
	}
	return view
}

func viewOf(i info.Info) AddonView {
	v := AddonView{Name: i.Name, Title: i.Title, Author: i.Author, Release: i.Release, Kind: i.Kind, Summary: i.Summary,
		Description: i.Description, Image: i.Image, Details: []DetailView{}, Settings: []SettingView{}}
	for _, d := range i.Details {
		v.Details = append(v.Details, DetailView{Label: d.Label, Value: d.Value})
	}
	for _, s := range i.Settings {
		sv := SettingView{ID: s.ID, Kind: s.Kind, Label: s.Label, Help: s.Help, Default: s.Default, Required: s.Required,
			Choices: []ChoiceView{}}
		for _, c := range s.Choices {
			sv.Choices = append(sv.Choices, ChoiceView{Value: c.Value, Label: c.Label})
		}
		v.Settings = append(v.Settings, sv)
	}
	return v
}

// ChooseFile asks for a file a setting of an add-on reads; "" when the user cancels.
func (a *Addons) ChooseFile() FileChoice {
	file, err := a.pick.OpenFile(FileAny)
	return FileChoice{File: file, Problem: ProblemOf(err)}
}

// FileChoice is the answer of ChooseFile.
type FileChoice struct {
	File    string   `json:"file"`
	Problem *Problem `json:"problem"`
}

// Run runs a source with the values of its settings, then previews the import of what it wrote.
// A failure keeps what the add-on said, in Report. Cancelling the call stops the program.
func (a *Addons) Run(ctx context.Context, name string, values map[string]string) ImportPreview {
	var target *addons.Installed
	for _, found := range addons.Find(ctx, a.folders()) {
		if found.Name == name {
			target = &found
			break
		}
	}
	if target == nil {
		return ImportPreview{Problem: ProblemOf(errNoAddOn)}
	}
	f, report, err := addons.Run(ctx, *target, values)
	if err != nil {
		return ImportPreview{Source: SourceAddOn, AddOn: name, AddOnTitle: target.Info.Title.In("en"), Report: report,
			Problem: ProblemOf(err)}
	}
	return a.data.previewAddOn(ctx, f, name, target.Info.Title.In("en"), target.Info.Release, report)
}
