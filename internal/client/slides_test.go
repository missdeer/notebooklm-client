package client

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestSlidesReuseNotebook(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected []string
		empty    bool
	}{
		{name: "all sources"},
		{name: "selected sources", selected: []string{"source-2"}},
		{name: "empty notebook", empty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stop := errors.New("stop after generation request")
			var calls []string
			c := New()
			c.transport = &downloadTestTransport{execute: func(req transport.Request) (string, error) {
				id := req.QueryParams["rpcids"]
				calls = append(calls, id)
				if !strings.Contains(req.Body["f.req"], "existing-notebook") {
					t.Fatal("request did not use existing notebook")
				}
				switch id {
				case rpc.GetNotebook:
					if len(tc.selected) > 0 {
						t.Fatal("explicit source selection should not fetch all sources")
					}
					sources := []any{}
					if !tc.empty {
						sources = []any{[]any{[]any{"source-1"}, "One"}, []any{[]any{"source-2"}, "Two"}}
					}
					inner, _ := json.Marshal([]any{[]any{"Notebook", sources}})
					raw, _ := json.Marshal([]any{[]any{"wrb.fr", id, string(inner)}})
					return string(raw), nil
				case rpc.GenerateArtifact:
					body := req.Body["f.req"]
					if !strings.Contains(body, "source-2") || (len(tc.selected) == 0 && !strings.Contains(body, "source-1")) {
						t.Fatalf("missing selected sources: %s", body)
					}
					if len(tc.selected) > 0 && strings.Contains(body, "source-1") {
						t.Fatal("unselected source included")
					}
					return "", stop
				default:
					t.Fatalf("unexpected RPC (must not create notebook or add sources): %s", id)
					return "", nil
				}
			}}
			_, err := c.RunSlideDeck(context.Background(), types.SlideDeckOptions{NotebookID: "existing-notebook", SourceIDs: tc.selected}, nil)
			if tc.empty {
				if err == nil || !strings.Contains(err.Error(), "no sources") || len(calls) != 1 {
					t.Fatalf("expected empty notebook rejection, got %v, calls %v", err, calls)
				}
			} else if !errors.Is(err, stop) {
				t.Fatalf("generation was not reached: %v", err)
			}
		})
	}
}

func TestSlidesRejectInvalidInputBeforeConnecting(t *testing.T) {
	for _, opts := range []types.SlideDeckOptions{
		{},
		{SourceIDs: []string{"source-1"}},
		{NotebookID: "existing", Source: types.SourceInput{Type: types.SourceText, Text: "new"}},
		{NotebookID: "existing", SourceIDs: []string{" "}},
	} {
		_, err := New().RunSlideDeck(context.Background(), opts, nil)
		if err == nil || strings.Contains(err.Error(), "not connected") {
			t.Fatalf("invalid input should fail before connecting: %v", err)
		}
	}
}
