package sshx_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

func TestShellCommand(t *testing.T) {
	cases := []struct {
		name   string
		target sshx.Target
		dir    string
		want   string
	}{
		{"host, user and the usual port",
			sshx.TargetFor(model.Host{Hostname: "ssh.example-host.ch", User: "u12ab"}), "sites/boulangerie",
			`ssh -t u12ab@ssh.example-host.ch "cd 'sites/boulangerie' && exec \$SHELL -l"`},
		{"another port and a key",
			sshx.TargetFor(model.Host{Hostname: "vps.example.test", User: "deploy", Port: 2222, KeyPath: "/Users/me/.ssh/id_vps"}), "/var/www/site",
			`ssh -t -p 2222 -o IdentitiesOnly=yes -i "/Users/me/.ssh/id_vps" deploy@vps.example.test "cd '/var/www/site' && exec \$SHELL -l"`},
		{"an alias of ~/.ssh/config",
			sshx.TargetFor(model.Host{Alias: "infomaniak"}), "web",
			`ssh -t infomaniak "cd 'web' && exec \$SHELL -l"`},
		{"no folder: the home directory",
			sshx.TargetFor(model.Host{Alias: "infomaniak"}), "",
			`ssh -t infomaniak`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.target.ShellCommand(c.dir)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("ShellCommand(%q) =\n%s\nwant\n%s", c.dir, got, c.want)
			}
		})
	}
}

// The command survives the user's shell, then the server's: a folder with spaces, quotes, $ and
// backquotes arrives as it is, and runs nothing.
func TestShellCommandQuotesOddFolders(t *testing.T) {
	target := sshx.TargetFor(model.Host{Hostname: "h.example.test", User: "u", KeyPath: `/Users/me/my "keys"/id`})
	for _, dir := range []string{"sites/l'atelier", `sites/a "b" $HOME`, "sites/`id` & ;x", `sites/back\slash`} {
		command, err := target.ShellCommand(dir)
		if err != nil {
			t.Fatal(err)
		}
		// The user's shell: print each argument ssh would receive, one per line.
		local, err := exec.Command("sh", "-c", "printf '%s\\n' "+strings.TrimPrefix(command, "ssh ")).Output()
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSuffix(string(local), "\n"), "\n")
		if got := args[len(args)-3]; got != `/Users/me/my "keys"/id` {
			t.Errorf("%q: the key arrives as %q", dir, got)
		}
		remote := args[len(args)-1]
		// The server's shell: the folder cd receives.
		folder, err := exec.Command("sh", "-c", strings.Replace(strings.Replace(remote, "cd ", "printf '%s' ", 1), " && exec $SHELL -l", "", 1)).Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(folder) != dir {
			t.Errorf("%q: the server's shell changes to %q", dir, folder)
		}
	}
}

func TestShellCommandRefusesControlCharacters(t *testing.T) {
	target := sshx.TargetFor(model.Host{Alias: "infomaniak"})
	for _, dir := range []string{"sites/a\nrm", "sites/\x00", "sites/\tx"} {
		if _, err := target.ShellCommand(dir); !errors.Is(err, sshx.ErrControlCharacter) {
			t.Errorf("ShellCommand(%q): %v, want ErrControlCharacter", dir, err)
		}
	}
}

func TestURL(t *testing.T) {
	cases := []struct {
		host model.Host
		want string
	}{
		{model.Host{Hostname: "ssh.example-host.ch", User: "u12ab"}, "ssh://u12ab@ssh.example-host.ch:22"},
		{model.Host{Hostname: "vps.example.test", User: "deploy", Port: 2222}, "ssh://deploy@vps.example.test:2222"},
		{model.Host{Hostname: "2001:db8::1", User: "deploy"}, "ssh://deploy@[2001:db8::1]:22"},
		{model.Host{Alias: "infomaniak"}, "ssh://infomaniak"},
	}
	for _, c := range cases {
		if got := sshx.TargetFor(c.host).URL(); got != c.want {
			t.Errorf("URL() of %+v = %q, want %q", c.host, got, c.want)
		}
	}
}
