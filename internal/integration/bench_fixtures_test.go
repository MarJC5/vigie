//go:build integration

package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

const sites = "/home/vigie/sites"

// wp runs WP-CLI on one fixture site.
func wp(t *testing.T, root string, args ...string) output {
	t.Helper()
	res, err := runAs(context.Background(), append(append([]string{"wp"}, args...), "--path="+root)...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSSHServerAnswers(t *testing.T) {
	ctx := context.Background()
	host, err := account.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := account.MappedPort(ctx, "22/tcp")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port.Port()), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	banner, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(banner, "SSH-2.0-OpenSSH") {
		t.Fatalf("banner = %q, want an OpenSSH server", banner)
	}
}

func TestInstalledSitesAnswerIsInstalled(t *testing.T) {
	for _, root := range []string{sites + "/plain", sites + "/parent-config/public", sites + "/network", sites + "/outdated"} {
		if res := wp(t, root, "core", "is-installed"); res.exit != 0 {
			t.Errorf("%s: exit %d, stderr %q", root, res.exit, res.stderr)
		}
	}
}

// The two meanings of exit 1, told apart by stderr.
func TestIsInstalledTellsNotANetworkFromCannotLoad(t *testing.T) {
	single := wp(t, sites+"/plain", "core", "is-installed", "--network")
	if single.exit != 1 || single.stderr != "" {
		t.Errorf("single site --network: exit %d, stderr %q; want 1 and no error", single.exit, single.stderr)
	}
	broken := wp(t, sites+"/broken", "core", "is-installed")
	if broken.exit != 1 || !strings.Contains(broken.stderr, "Error") {
		t.Errorf("broken site: exit %d, stderr %q; want 1 and an error", broken.exit, broken.stderr)
	}
}

func TestNetworkHasThreeSubsitesOneArchived(t *testing.T) {
	if res := wp(t, sites+"/network", "core", "is-installed", "--network"); res.exit != 0 {
		t.Fatalf("--network: exit %d, stderr %q", res.exit, res.stderr)
	}
	res := wp(t, sites+"/network", "site", "list", "--format=json", "--fields=blog_id,archived")
	var list []struct {
		BlogID   json.Number `json:"blog_id"`
		Archived json.Number `json:"archived"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &list); err != nil {
		t.Fatalf("site list: %v\n%s%s", err, res.stdout, res.stderr)
	}
	archived := 0
	for _, s := range list {
		if s.Archived == "1" {
			archived++
		}
	}
	if len(list) != 4 || archived != 1 {
		t.Fatalf("sites = %+v, want the main site and three subsites, one archived", list)
	}
}

func TestOutdatedSiteHasPendingUpdates(t *testing.T) {
	for _, kind := range []string{"plugin", "theme"} {
		res := wp(t, sites+"/outdated", kind, "list", "--format=json", "--fields=name,update")
		var list []struct{ Name, Update string }
		if err := json.Unmarshal([]byte(res.stdout), &list); err != nil {
			t.Fatalf("%s list: %v\n%s%s", kind, err, res.stdout, res.stderr)
		}
		pending := false
		for _, item := range list {
			pending = pending || (item.Update == "available" && (item.Name == "akismet" || item.Name == "twentytwentyfour"))
		}
		if !pending {
			t.Errorf("%s list = %+v, want an update available for the older version", kind, list)
		}
	}
}

func TestStrayCopyUnderNodeModulesExists(t *testing.T) {
	res, err := runAs(context.Background(), "test", "-f", sites+"/plain/node_modules/stray/wp-includes/version.php")
	if err != nil || res.exit != 0 {
		t.Fatalf("stray copy missing: %v, exit %d", err, res.exit)
	}
}
