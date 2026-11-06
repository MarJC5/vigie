package cli

import (
	"context"
	"errors"
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/mcp"
)

// runMCP serves Vigie to an assistant over the standard input and output, until the client closes
// the input (ADR-0043, point 1). The output carries the protocol only: Vigie's own messages go to
// the error output.
func runMCP(ctx context.Context, env Env, args []string) int {
	if len(args) > 0 {
		return env.fail(env.t("cli.mcpArguments", nil))
	}
	ws, err := env.Open(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	defer ws.Close()
	server := mcp.New(mcp.Config{Access: ws.AssistantAccess, Texts: env.Texts, Lang: env.Lang, Version: buildinfo.Version()})
	err = server.Run(ctx, &sdk.IOTransport{Reader: io.NopCloser(env.In), Writer: nopWriteCloser{env.Out}})
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return env.fail(err.Error())
	}
	return exitOK
}

// nopWriteCloser leaves the standard output open: it is the process's, not the transport's.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
