package hostcheck_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/hostcheck"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
)

const (
	phpVersion = "'php' '-r' 'echo PHP_VERSION;'"
	infoWP     = "'wp' 'cli' 'info' '--format=json'"
	infoWPCLI  = "'wp-cli' 'cli' 'info' '--format=json'"
	infoBin    = "'bin/wp' 'cli' 'info' '--format=json'"
	infoPhar   = "'php' 'wp-cli.phar' 'cli' 'info' '--format=json'"
	infoCustom = "'/opt/wp/bin/wp' 'cli' 'info' '--format=json'"
)

// The keys Vigie reads from `wp cli info --format=json` (full recordings: internal/wpcli).
var cliInfo = []byte(`{"php_version":"8.4.27","wp_cli_version":"2.12.0"}`)

var notFound = sshx.Result{ExitCode: 127, Stderr: []byte("sh: 1: wp: not found\n")}

func TestFindsTheConfiguredCommandFirst(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(phpVersion, sshx.Result{Stdout: []byte("8.4.27")})
	fake.Set(infoCustom, sshx.Result{Stdout: cliInfo})
	report, err := hostcheck.Check(context.Background(), fake, sshx.Target{Alias: "a"}, "/opt/wp/bin/wp")
	if err != nil {
		t.Fatal(err)
	}
	want := hostcheck.Report{PHPVersion: "8.4.27", WPCLI: "/opt/wp/bin/wp", WPCLIVersion: "2.12.0"}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("Check() = %+v, want %+v", report, want)
	}
}

func TestTriesTheUsualPlacesInOrder(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(phpVersion, sshx.Result{Stdout: []byte("8.3.12")})
	fake.Set(infoWP, notFound)
	fake.Set(infoWPCLI, notFound)
	fake.Set(infoBin, notFound)
	fake.Set(infoPhar, sshx.Result{Stdout: cliInfo})
	report, err := hostcheck.Check(context.Background(), fake, sshx.Target{Alias: "a"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.WPCLI != "php ~/wp-cli.phar" {
		t.Fatalf("Check() found %q, want the phar in the home directory", report.WPCLI)
	}
	if calls := fake.Calls(); !slices.Equal(calls, []string{phpVersion, infoWP, infoWPCLI, infoBin, infoPhar}) {
		t.Fatalf("calls = %q", calls)
	}
}

func TestNoWPCLIListsWhatWasTried(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(phpVersion, sshx.Result{Stdout: []byte("8.4.27")})
	for _, c := range []string{infoWP, infoWPCLI, infoBin, infoPhar} {
		fake.Set(c, notFound)
	}
	report, err := hostcheck.Check(context.Background(), fake, sshx.Target{Alias: "a"}, "wp")
	if !errors.Is(err, hostcheck.ErrNoWPCLI) {
		t.Fatalf("Check() error = %v, want ErrNoWPCLI", err)
	}
	if report.PHPVersion != "8.4.27" || len(report.Tried) != 4 {
		t.Fatalf("Check() = %+v, want the PHP version and the four commands tried", report)
	}
}

func TestNoPHP(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set(phpVersion, sshx.Result{ExitCode: 127, Stderr: []byte("sh: 1: php: not found\n")})
	if _, err := hostcheck.Check(context.Background(), fake, sshx.Target{Alias: "a"}, ""); !errors.Is(err, hostcheck.ErrNoPHP) {
		t.Fatalf("Check() error = %v, want ErrNoPHP", err)
	}
}

func TestConnectionErrorsStopTheCheck(t *testing.T) {
	refused := &sshx.ConnectionError{Failure: sshx.FailureRefused, Message: "Connection refused"}
	_, err := hostcheck.Check(context.Background(), failing{refused}, sshx.Target{Alias: "a"}, "")
	var connErr *sshx.ConnectionError
	if !errors.As(err, &connErr) || connErr.Failure != sshx.FailureRefused {
		t.Fatalf("Check() error = %v, want the connection error", err)
	}
}

// failing answers every command with the same error.
type failing struct{ err error }

func (f failing) Run(context.Context, sshx.Target, string) (sshx.Result, error) {
	return sshx.Result{}, f.err
}

func (f failing) RunInput(ctx context.Context, target sshx.Target, remote string, _ []byte) (sshx.Result, error) {
	return f.Run(ctx, target, remote)
}
