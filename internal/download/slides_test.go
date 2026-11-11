package download

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func metadataResponse(t *testing.T, value any) string {
	t.Helper()
	inner, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]any{[]any{"wrb.fr", rpc.GetInteractiveHTML, string(inner)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestGetArtifactMetadataUsesDetailRPC(t *testing.T) {
	meta := []any{"slide-id", "Slides", 8, nil, 3}
	for _, value := range []any{meta, []any{meta}} {
		call := func(ctx context.Context, id string, args []any, path string) (string, error) {
			if id != rpc.GetInteractiveHTML || len(args) != 1 || args[0] != "slide-id" {
				t.Fatalf("unexpected metadata request: %s %v", id, args)
			}
			return metadataResponse(t, value), nil
		}
		got, err := GetArtifactMetadata(context.Background(), call, "slide-id")
		if err != nil || len(got) != 5 || got[0] != "slide-id" {
			t.Fatalf("got %v, %v", got, err)
		}
	}
}

func TestSaveSlideDeckOpaqueURLsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		pptx bool
		fail bool
	}{
		{"both formats", true, false}, {"PDF only", false, false}, {"download fails", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := make([]any, 17)
			meta[0] = "slide-id"
			cfg := []any{[]any{"instructions"}, "en", nil, "https://example.com/opaque-a", nil}
			if tc.pptx {
				cfg[4] = "https://example.com/opaque-b"
			}
			meta[16] = cfg
			call := func(context.Context, string, []any, string) (string, error) {
				return metadataResponse(t, []any{meta}), nil
			}
			count := 0
			downloadErr := errors.New("CDN rejected request")
			dl := func(ctx context.Context, url, dir, filename string) (string, error) {
				count++
				if tc.fail {
					return "", downloadErr
				}
				if strings.HasSuffix(filename, ".pptx") && url != cfg[4] {
					t.Fatalf("wrong PPTX URL %s", url)
				}
				if strings.HasSuffix(filename, ".pdf") && url != cfg[3] {
					t.Fatalf("wrong PDF URL %s", url)
				}
				return filename, nil
			}
			primary, pdf, err := SaveSlideDeck(context.Background(), call, dl, "slide-id", t.TempDir())
			if tc.fail {
				if !errors.Is(err, downloadErr) {
					t.Fatalf("lost download error: %v", err)
				}
				return
			}
			if err != nil || primary == "" || pdf == "" {
				t.Fatalf("paths %q %q, err %v", primary, pdf, err)
			}
			want := 1
			if tc.pptx {
				want = 2
			}
			if count != want {
				t.Fatalf("downloads = %d, want %d", count, want)
			}
		})
	}
}

func TestSlideDeckURLsNotReadyForGenerationOptions(t *testing.T) {
	meta := make([]any, 17)
	meta[16] = []any{[]any{"prompt", "en", 1, 3}}
	if pdf, pptx := slideDeckURLs(meta); pdf != "" || pptx != "" {
		t.Fatal("generation options treated as download URLs")
	}
}
