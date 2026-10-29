//go:build integration

// Package integration_test runs against a throwaway shared hosting account: WordPress files,
// WP-CLI and an SSH server in one container, MariaDB in another, on a private Docker network.
// Run with make test-integration; Docker is the only requirement.
package integration_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/mariadb"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Fixed for the bench only: the database is reachable from the private network alone.
const dbPassword = "fixture-root"

// account is the container of the shared hosting account, ready with the fixtures of
// testdata/wordpress/fixtures.sh.
var account testcontainers.Container

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	stop, err := start(ctx)
	defer stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration bench:", err)
		return 1
	}
	code := m.Run()
	if accessVal.dir != "" {
		_ = os.RemoveAll(accessVal.dir)
	}
	return code
}

// start brings the bench up; stop removes whatever was created, even after an error.
func start(ctx context.Context) (stop func(), err error) {
	var cleanups []func()
	stop = func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	nw, err := network.New(ctx)
	if err != nil {
		return stop, err
	}
	cleanups = append(cleanups, func() { _ = nw.Remove(ctx) })

	db, err := mariadb.Run(ctx, "mariadb:11.8.9",
		mariadb.WithUsername("root"), mariadb.WithPassword(dbPassword),
		network.WithNetwork([]string{"db"}, nw))
	cleanups = append(cleanups, func() { _ = testcontainers.TerminateContainer(db) })
	if err != nil {
		return stop, err
	}

	wp, err := testcontainers.Run(ctx, "",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{
			Context: "testdata/wordpress",
			Repo:    "vigie-integration-account",
			Tag:     "latest",
			// Kept so the next run reuses the layers instead of downloading everything again.
			KeepImage: true,
		}),
		network.WithNetwork([]string{"account"}, nw),
		testcontainers.WithExposedPorts("22/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("22/tcp")))
	cleanups = append(cleanups, func() { _ = testcontainers.TerminateContainer(wp) })
	if err != nil {
		return stop, err
	}
	account = wp

	res, err := runAs(ctx, "vigie-fixtures", "db", "root", dbPassword)
	if err != nil {
		return stop, err
	}
	if res.exit != 0 {
		return stop, fmt.Errorf("fixtures: exit %d\n%s%s", res.exit, res.stdout, res.stderr)
	}
	return stop, nil
}

// output is what a command printed and returned in the account.
type output struct {
	stdout, stderr string
	exit           int
}

// runAs runs a command in the account as the user "vigie", keeping stdout and stderr apart.
func runAs(ctx context.Context, cmd ...string) (output, error) {
	exit, reader, err := account.Exec(ctx, cmd, tcexec.WithUser("vigie"), tcexec.WithWorkingDir("/home/vigie"))
	if err != nil {
		return output{}, err
	}
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return output{}, err
	}
	return output{stdout: stdout.String(), stderr: stderr.String(), exit: exit}, nil
}

// timeNow is the clock of the scans in these tests.
func timeNow() time.Time { return time.Now() }
