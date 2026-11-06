package cli_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// vigie mcp speaks only the protocol on its standard output, says nothing on its error output, and
// ends with success when its client closes its input (ADR-0043, point 1).
func TestMcpSpeaksOnlyTheProtocolAndEndsWithItsInput(t *testing.T) {
	ctx := context.Background()
	ws := benchWorkspace(t)
	if err := ws.SetAssistantAccess(ctx, model.AssistantRead); err != nil {
		t.Fatal(err)
	}
	toServer, clientWrites := io.Pipe()
	clientReads, fromServer := io.Pipe()
	var errOut strings.Builder
	done := make(chan int, 1)
	go func() {
		code := cli.Run(ctx, cli.Options{Command: []string{"mcp"}}, cli.Env{
			In: toServer, Out: fromServer, Err: &errOut, Lang: "en", Texts: texts(t),
			Open: func(context.Context) (*workspace.Workspace, error) { return ws, nil },
		})
		fromServer.Close()
		done <- code
	}()

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, &sdk.IOTransport{Reader: clientReads, Writer: clientWrites}, nil)
	if err != nil {
		t.Fatalf("connect: %v (stderr %q)", err, errOut.String())
	}
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	cs.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit status %d, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("vigie mcp did not end once its input closed")
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", errOut.String())
	}
}

// vigie mcp takes no argument: the client starts it as Settings gives it.
func TestMcpRefusesArguments(t *testing.T) {
	code, out := runIn(t, benchWorkspace(t), "en", "mcp", "--port", "9099")
	if code == 0 || !strings.Contains(out, "vigie mcp takes no argument") {
		t.Errorf("exit %d, %q; want refused", code, out)
	}
}

// vigie help names vigie mcp.
func TestHelpNamesMcp(t *testing.T) {
	for _, lang := range []string{"en", "fr"} {
		code, out, _ := run(t, lang, "help")
		if code != 0 || !strings.Contains(out, "\n  mcp ") {
			t.Errorf("help in %s does not name mcp:\n%s", lang, out)
		}
	}
}
