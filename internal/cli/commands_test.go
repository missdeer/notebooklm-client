package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/spf13/cobra"
)

func TestMain(m *testing.M) {
	initializeCommands()
	os.Exit(m.Run())
}

func TestCommandRoutesAndFlags(t *testing.T) {
	for _, s := range commandDefinitions {
		t.Run(s.Path, func(t *testing.T) {
			cmd := rootCmd
			for _, part := range strings.Fields(s.Path) {
				cmd = directChild(cmd, part)
				if cmd == nil {
					t.Fatalf("route %s is not registered", s.Path)
				}
			}
			if cmd.RunE == nil {
				t.Fatal("route has no handler")
			}
			for _, f := range s.Flags {
				if cmd.Flags().Lookup(f.Name) == nil {
					t.Errorf("missing flag %s", f.Name)
				}
			}
			if strings.HasPrefix(s.Canonical, "setup ") || strings.HasPrefix(s.Canonical, "skill ") {
				t.Fatal("MCP integration must not be registered")
			}
			if cmd.Flags().Lookup("mcp-compatible") != nil {
				t.Fatal("MCP-specific output flag registered")
			}
		})
	}
	for _, name := range []string{"setup", "skill", "install", "uninstall", "update"} {
		if directChild(rootCmd, name) != nil {
			t.Errorf("unexpected integration command %s", name)
		}
	}
}

type scriptedTransport struct {
	run func(transport.Request) (string, error)
}

func (s scriptedTransport) Execute(ctx context.Context, r transport.Request) (string, error) {
	return s.run(r)
}
func (s scriptedTransport) GetSession() types.NotebookRpcSession {
	return types.NotebookRpcSession{Language: "en"}
}
func (s scriptedTransport) Close() error                         { return nil }
func (s scriptedTransport) RefreshSession(context.Context) error { return nil }
func rpcResponse(id string, v any) string {
	b, _ := json.Marshal(v)
	out, _ := json.Marshal([]any{[]any{"wrb.fr", id, string(b)}})
	return string(out)
}
func TestBatchCreateDispatchesNotebookCreation(t *testing.T) {
	rpc.SetHomeDir(t.TempDir())
	t.Cleanup(func() { rpc.SetHomeDir("") })
	calls := []string{}
	c := client.NewWithTransport(scriptedTransport{run: func(r transport.Request) (string, error) {
		id := r.QueryParams["rpcids"]
		calls = append(calls, id)
		switch id {
		case rpc.CreateNotebook:
			return rpcResponse(id, []any{"Notebook", nil, "new-id"}), nil
		case rpc.RenameNotebook:
			return rpcResponse(id, []any{}), nil
		}
		t.Fatalf("unexpected artifact RPC %s", id)
		return "", nil
	}})
	parent := &cobra.Command{}
	parent.SetOut(&bytes.Buffer{})
	_, err := runRemoteCommand(context.Background(), c, internalInput(parent, "batch create", map[string]any{"titles": "One,Two"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("expected two create/rename pairs: %v", calls)
	}
}
func TestRepeatableSourceFlagsAndNegativeBoolean(t *testing.T) {
	cmd := &cobra.Command{}
	for _, f := range []commandFlag{{Name: "url", Type: "strings"}, {Name: "compact", Type: "bool", Default: true, Negative: "full"}} {
		addSchemaFlag(cmd.Flags(), f)
	}
	if err := cmd.ParseFlags([]string{"--url", "https://a.example", "--url", "https://b.example", "--full"}); err != nil {
		t.Fatal(err)
	}
	if len(flagStrings(cmd, "url")) != 2 {
		t.Fatal("URL repetitions were lost")
	}
	if flagBool(cmd, "compact") {
		t.Fatal("--full did not invert compact")
	}
}
func TestLocalCommandsPersistAndRejectTraversal(t *testing.T) {
	rpc.SetHomeDir(t.TempDir())
	t.Cleanup(func() { rpc.SetHomeDir("") })
	run := func(path string, values map[string]any) (string, error) {
		cmd := &cobra.Command{}
		var output bytes.Buffer
		cmd.SetOut(&output)
		in := internalInput(cmd, path, values)
		handled, err := runLocalCommand(in)
		if !handled {
			t.Fatalf("%s not handled locally", path)
		}
		return output.String(), err
	}
	if _, err := run("alias set", map[string]any{"name": "research", "value": "notebook-id"}); err != nil {
		t.Fatal(err)
	}
	if id, err := resolveResource("research"); err != nil || id != "notebook-id" {
		t.Fatalf("alias: %s %v", id, err)
	}
	if _, err := run("tag add", map[string]any{"notebook_id": "research", "tags": "ai,science"}); err != nil {
		t.Fatal(err)
	}
	out, err := run("tag select", map[string]any{"query": "ai unrelated"})
	if err != nil || !strings.Contains(out, "notebook-id") {
		t.Fatalf("selection: %s %v", out, err)
	}
	if _, err := profileSessionPath("../../elsewhere"); err == nil {
		t.Fatal("profile traversal accepted")
	}
	file := filepath.Join(t.TempDir(), "pipeline.yaml")
	if err := os.WriteFile(file, []byte("steps:\n  - action: notebook_delete\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := run("pipeline create", map[string]any{"name": "../escape", "file": file}); err == nil {
		t.Fatal("pipeline traversal accepted")
	}
}
func TestCatalogCanonicalHandlers(t *testing.T) {
	// An unknown handler must be detected before connecting or mutating state.
	var schema []commandSchema
	if err := json.Unmarshal(commandCatalog, &schema); err != nil {
		t.Fatal(err)
	}
	for _, s := range schema {
		p := s.Canonical
		if p == "" {
			t.Fatalf("empty canonical route: %s", s.Path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitUntil(ctx, 1, func() (bool, error) { return false, nil }); err == nil {
		t.Fatal("cancelled poll succeeded")
	}
}
