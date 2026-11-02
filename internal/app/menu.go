package app

// MenuEvent carries a MenuCommand to the window when a menu item with an action is chosen.
const MenuEvent = "vigie:menu"

// The actions of the menu bar, carried out by the window.
const (
	ActionNewHost   = "new-host"   // opens the "add a host" wizard
	ActionNewFolder = "new-folder" // creates a folder next to the selection and names it in place
	ActionSearch    = "search"     // focuses the search of the site list
	ActionScan      = "scan"       // scans the selected folder, host or site
	ActionScanAll   = "scan-all"   // scans every site
	ActionSettings  = "settings"   // opens the settings
	ActionHelp      = "help"       // opens the help page
	// ActionOpen opens the page of what a notified run covered (ADR-0035): Nodes, one or several.
	ActionOpen = "open"
)

// MenuCommand is what the window receives with MenuEvent.
type MenuCommand struct {
	Action string   `json:"action"`
	Nodes  []string `json:"nodes,omitempty"` // ActionOpen: the nodes to open, "" for everything
}

// MenuEntry is one item of a menu: a separator, a standard item (Role, relabelled in the interface
// language) or an Action for the window.
type MenuEntry struct {
	Separator   bool
	Key         string // catalog key of the label; {{app}} is the application name
	Accelerator string // for example "CmdOrCtrl+N"
	Action      string
	Role        string // a standard item: undo, copy, quit...
}

// MenuSection is one menu of the bar; the application menu (macOS) has no key.
type MenuSection struct {
	Key   string
	Items []MenuEntry
}

var separator = MenuEntry{Separator: true}

// MenuBar is the menu bar, in order. The window builds it with the interface language.
var MenuBar = []MenuSection{
	{Key: "", Items: []MenuEntry{
		{Key: "menu.about", Role: "about"},
		separator,
		{Key: "menu.settings", Accelerator: "CmdOrCtrl+,", Action: ActionSettings},
		separator,
		{Key: "menu.hide", Role: "hide"},
		{Key: "menu.hideOthers", Role: "hideOthers"},
		{Key: "menu.showAll", Role: "showAll"},
		separator,
		{Key: "menu.quit", Role: "quit"},
	}},
	{Key: "menu.file", Items: []MenuEntry{
		{Key: "menu.newHost", Accelerator: "CmdOrCtrl+N", Action: ActionNewHost},
		{Key: "menu.newFolder", Accelerator: "CmdOrCtrl+Shift+N", Action: ActionNewFolder},
		separator,
		{Key: "menu.closeWindow", Role: "closeWindow"},
	}},
	{Key: "menu.edit", Items: []MenuEntry{
		{Key: "menu.undo", Role: "undo"},
		{Key: "menu.redo", Role: "redo"},
		separator,
		{Key: "menu.cut", Role: "cut"},
		{Key: "menu.copy", Role: "copy"},
		{Key: "menu.paste", Role: "paste"},
		{Key: "menu.selectAll", Role: "selectAll"},
	}},
	{Key: "menu.view", Items: []MenuEntry{
		{Key: "menu.search", Accelerator: "CmdOrCtrl+F", Action: ActionSearch},
		separator,
		{Key: "menu.fullScreen", Role: "toggleFullscreen"},
	}},
	{Key: "menu.sites", Items: []MenuEntry{
		{Key: "menu.scan", Accelerator: "CmdOrCtrl+R", Action: ActionScan},
		{Key: "menu.scanAll", Accelerator: "CmdOrCtrl+Shift+R", Action: ActionScanAll},
	}},
	{Key: "menu.window", Items: []MenuEntry{
		{Key: "menu.minimise", Role: "minimise"},
		{Key: "menu.zoom", Role: "zoom"},
		separator,
		{Key: "menu.front", Role: "bringAllToFront"},
	}},
	{Key: "menu.help", Items: []MenuEntry{
		{Key: "menu.vigieHelp", Action: ActionHelp},
	}},
}
