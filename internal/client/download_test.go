package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type downloadTestTransport struct {
	execute func(transport.Request) (string, error)
}

func (d *downloadTestTransport) Execute(ctx context.Context, r transport.Request) (string, error) {
	return d.execute(r)
}
func (*downloadTestTransport) GetSession() types.NotebookRpcSession {
	return types.NotebookRpcSession{}
}
func (*downloadTestTransport) RefreshSession(context.Context) error { return nil }
func (*downloadTestTransport) Close() error                         { return nil }

func TestDownloadArtifactValidatesNotebookAndSavesSelectedFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pdf" {
			t.Errorf("wrong format requested: %s", r.URL.Path)
		}
		w.Write([]byte("%PDF-test"))
	}))
	defer server.Close()
	meta := make([]any, 17)
	meta[0], meta[1], meta[2], meta[4] = "slide-id", "Deck", 8, 3
	meta[16] = []any{nil, nil, nil, server.URL + "/pdf", server.URL + "/pptx"}
	for _, missing := range []bool{false, true} {
		calls := 0
		c := New()
		c.transport = &downloadTestTransport{execute: func(req transport.Request) (string, error) {
			calls++
			id := req.QueryParams["rpcids"]
			var value any = []any{meta}
			if id == rpc.GetArtifactsFiltered {
				if !strings.Contains(req.Body["f.req"], "notebook-id") {
					t.Error("notebook not included in list request")
				}
				value = []any{[]any{meta}}
				if missing {
					value = []any{[]any{}}
				}
			} else if id != rpc.GetInteractiveHTML {
				t.Fatalf("unexpected RPC %s", id)
			}
			inner, _ := json.Marshal(value)
			raw, _ := json.Marshal([]any{[]any{"wrb.fr", id, string(inner)}})
			return string(raw), nil
		}}
		path, err := c.DownloadArtifact(context.Background(), "notebook-id", "slide-id", t.TempDir(), "pdf")
		if missing {
			if err == nil || !strings.Contains(err.Error(), "not found") || calls != 1 {
				t.Fatalf("missing artifact was not rejected: %v (%d calls)", err, calls)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "%PDF-test" {
			t.Fatalf("bad downloaded file %q: %v", body, err)
		}
	}
}
