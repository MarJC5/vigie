package mcp

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/locales"
)

// probe is a tool of a level that answers its own name, for the tests of the levels.
func probe(name string) (*sdk.Tool, sdk.ToolHandler) {
	tool := &sdk.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}
	return tool, func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: name + " ran"}}}, nil
	}
}

// connected starts a server whose level is *level, read at every call, with a read and an act probe,
// and a client connected to it in memory at the given revision ("" for the newest).
func connected(t *testing.T, level *model.AssistantAccess, revision string) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{
		Access:  func(context.Context) (model.AssistantAccess, error) { return *level, nil },
		Texts:   texts,
		Lang:    "en",
		Version: "0.3.0-test",
	})
	readTool, readHandler := probe("probe_read")
	s.add(model.AssistantRead, readTool, readHandler)
	actTool, actHandler := probe("probe_act")
	s.add(model.AssistantAct, actTool, actHandler)
	serverEnd, clientEnd := sdk.NewInMemoryTransports()
	if _, err := s.server.Connect(ctx, serverEnd, nil); err != nil {
		t.Fatal(err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, clientEnd, &sdk.ClientSessionOptions{ProtocolVersion: revision})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// call calls a tool and returns whether it was refused, and its text.
func call(t *testing.T, cs *sdk.ClientSession, name string) (bool, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return res.IsError, text.String()
}

// Off: no tool listed, and any call says where to turn the assistants on (ADR-0043, point 3).
func TestOffListsNoToolAndRefusesEveryCall(t *testing.T) {
	level := model.AssistantOff
	cs := connected(t, &level, "")
	if names := toolNames(t, cs); len(names) != 0 {
		t.Errorf("tools = %v, want none", names)
	}
	for _, name := range []string{"probe_read", "probe_act", "delete_site"} {
		refused, text := call(t, cs, name)
		if !refused || !strings.Contains(text, "Settings, Assistants") {
			t.Errorf("%s: refused %v, %q; want refused, saying where to turn them on", name, refused, text)
		}
	}
}

// Read: the read tools only; an action says it needs the level Actions.
func TestReadListsTheReadToolsOnly(t *testing.T) {
	level := model.AssistantRead
	cs := connected(t, &level, "")
	if names := toolNames(t, cs); !slices.Equal(names, []string{"probe_read"}) {
		t.Errorf("tools = %v, want [probe_read]", names)
	}
	if refused, text := call(t, cs, "probe_read"); refused || text != "probe_read ran" {
		t.Errorf("probe_read: refused %v, %q", refused, text)
	}
	if refused, text := call(t, cs, "probe_act"); !refused || !strings.Contains(text, "Actions") {
		t.Errorf("probe_act: refused %v, %q; want refused, naming the level Actions", refused, text)
	}
}

// Act: every tool.
func TestActListsEveryTool(t *testing.T) {
	level := model.AssistantAct
	cs := connected(t, &level, "")
	if names := toolNames(t, cs); !slices.Equal(names, []string{"probe_act", "probe_read"}) {
		t.Errorf("tools = %v, want both", names)
	}
	if refused, _ := call(t, cs, "probe_act"); refused {
		t.Error("probe_act refused at the level act")
	}
}

// The level is read at every call: a change in Settings applies to a client already started.
func TestTheLevelIsReadAtEveryCall(t *testing.T) {
	level := model.AssistantOff
	cs := connected(t, &level, "")
	if names := toolNames(t, cs); len(names) != 0 {
		t.Fatalf("tools while off = %v", names)
	}
	level = model.AssistantRead
	if names := toolNames(t, cs); !slices.Equal(names, []string{"probe_read"}) {
		t.Errorf("tools once read = %v, want [probe_read]", names)
	}
	if refused, _ := call(t, cs, "probe_read"); refused {
		t.Error("probe_read refused once read")
	}
	level = model.AssistantOff
	if refused, _ := call(t, cs, "probe_read"); !refused {
		t.Error("probe_read ran once off again")
	}
}

// A client of the revision 2025-11-25, which opens with initialize, is served the same way.
func TestAClientOfTheRevisionBeforeIsServed(t *testing.T) {
	level := model.AssistantRead
	cs := connected(t, &level, "2025-11-25")
	if got := cs.InitializeResult(); got == nil || got.ProtocolVersion != "2025-11-25" {
		t.Errorf("negotiated %+v, want 2025-11-25", got)
	}
	if names := toolNames(t, cs); !slices.Equal(names, []string{"probe_read"}) {
		t.Errorf("tools = %v, want [probe_read]", names)
	}
}
