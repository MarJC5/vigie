package app

import (
	"os"
	"path/filepath"

	"github.com/MarJC5/vigie/internal/sshx"
)

// CommandView is the vigie command as the settings show it (ADR-0036): linked to this app, where,
// or the one command that links it.
type CommandView struct {
	Linked bool `json:"linked"`
	// Path is the link found, or the one the command makes.
	Path string `json:"path"`
	// Command links it, to run once in a terminal; empty when linked.
	Command string `json:"command"`
	// AppImage: the link goes in the user's ~/.local/bin, which must be on the PATH.
	AppImage bool `json:"appImage"`
}

// CommandEnv is where the command is looked for: this app's binary, the AppImage file when Vigie
// runs from one, and the folders of the PATH a link may be in, the first one being where the
// command links it.
type CommandEnv struct {
	Executable string
	AppImage   string
	Dirs       []string
}

// CommandState tells whether a folder holds a vigie that leads to this app, and otherwise the
// command that links it: with sudo into /usr/local/bin on macOS, into ~/.local/bin for an AppImage.
func CommandState(env CommandEnv) CommandView {
	target := env.Executable
	if env.AppImage != "" {
		target = env.AppImage
	}
	view := CommandView{AppImage: env.AppImage != ""}
	real := target
	if r, err := filepath.EvalSymlinks(target); err == nil {
		real = r
	}
	for _, dir := range env.Dirs {
		link := filepath.Join(dir, "vigie")
		if r, err := filepath.EvalSymlinks(link); err == nil && r == real {
			return CommandView{Linked: true, Path: link, AppImage: view.AppImage}
		}
	}
	if len(env.Dirs) == 0 {
		return view
	}
	view.Path = filepath.Join(env.Dirs[0], "vigie")
	paths, err := sshx.Quote(target, view.Path)
	if err != nil {
		return view
	}
	if view.AppImage {
		dir, _ := sshx.Quote(env.Dirs[0])
		view.Command = "mkdir -p " + dir + " && ln -sf " + paths
	} else {
		// /usr/local/bin belongs to root: the system asks for the password, not Vigie.
		view.Command = "sudo ln -sf " + paths
	}
	return view
}

// CommandLink serves the vigie command's state to the settings.
type CommandLink struct {
	env func() CommandEnv
}

// NewCommandLink returns the service for this process: its binary, its AppImage, the folders of the
// PATH where a link is looked for.
func NewCommandLink(goos, home string) *CommandLink {
	return &CommandLink{env: func() CommandEnv {
		exe, _ := os.Executable()
		env := CommandEnv{Executable: exe, AppImage: os.Getenv("APPIMAGE")}
		switch {
		case env.AppImage != "":
			env.Dirs = []string{filepath.Join(home, ".local", "bin"), "/usr/local/bin", "/usr/bin"}
		case goos == "darwin":
			env.Dirs = []string{"/usr/local/bin", "/opt/homebrew/bin"}
		default:
			env.Dirs = []string{"/usr/local/bin", "/usr/bin", filepath.Join(home, ".local", "bin")}
		}
		return env
	}}
}

// State reads, each time it is asked, whether vigie is linked to this app.
func (c *CommandLink) State() CommandView {
	return CommandState(c.env())
}
