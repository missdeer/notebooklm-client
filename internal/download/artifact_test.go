package download

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestSaveExistingArtifactFormats(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    int
		format  string
		wantURL string
		ext     string
	}{
		{"slides default", 8, "auto", "https://example.com/presentation", ".pptx"},
		{"slides PDF", 8, "pdf", "https://example.com/document", ".pdf"},
		{"audio", 1, "", "https://example.com/audio", ".m4a"},
		{"video", 3, "mp4", "https://example.com/video", ".mp4"},
		{"infographic", 7, "png", "https://example.com/image", ".png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := make([]any, 17)
			meta[0] = "artifact-id"
			meta[8] = []any{nil, nil, nil, "https://example.com/video"}
			meta[14] = []any{nil, nil, []any{[]any{nil, []any{"https://example.com/image"}}}}
			meta[16] = []any{nil, nil, nil, "https://example.com/document", "https://example.com/presentation"}
			call := func(context.Context, string, []any, string) (string, error) {
				return metadataResponse(t, []any{meta}), nil
			}
			dir := t.TempDir()
			dl := func(ctx context.Context, url, outputDir, name string) (string, error) {
				if url != tc.wantURL || outputDir != dir || !strings.HasSuffix(name, tc.ext) {
					t.Fatalf("unexpected download %s %s %s", url, outputDir, name)
				}
				return name, nil
			}
			path, err := SaveArtifact(context.Background(), call, nil, dl, types.ArtifactInfo{ID: "artifact-id", Type: tc.kind, DownloadURL: "https://example.com/audio"}, dir, tc.format)
			if err != nil || path == "" {
				t.Fatalf("%q, %v", path, err)
			}
		})
	}
}

func TestSaveArtifactRejectsUnavailableFormatAndNotReady(t *testing.T) {
	status := 1
	for _, tc := range []struct {
		kind   int
		format string
		status *int
		want   string
	}{
		{8, "mp3", nil, "pptx or pdf"}, {1, "mp3", nil, "conversion"},
		{8, "pdf", &status, "not ready"}, {5, "", nil, "not supported"},
	} {
		_, err := SaveArtifact(context.Background(), nil, nil, nil, types.ArtifactInfo{ID: "id", Type: tc.kind, StatusCode: tc.status}, "", tc.format)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("got %v, want %s", err, tc.want)
		}
	}
	meta := make([]any, 17)
	meta[0], meta[16] = "id", []any{nil, nil, nil, "https://example.com/pdf"}
	call := func(context.Context, string, []any, string) (string, error) {
		return metadataResponse(t, []any{meta}), nil
	}
	_, err := SaveArtifact(context.Background(), call, nil, nil, types.ArtifactInfo{ID: "id", Type: 8}, "", "pptx")
	if err == nil || !strings.Contains(err.Error(), "no pptx") {
		t.Fatalf("unexpected error: %v", err)
	}
	wantErr := errors.New("download failed")
	dl := func(context.Context, string, string, string) (string, error) { return "", wantErr }
	_, err = SaveArtifact(context.Background(), call, nil, dl, types.ArtifactInfo{ID: "id", Type: 8}, "", "auto")
	if !errors.Is(err, wantErr) {
		t.Fatalf("download error lost: %v", err)
	}
}

func TestSaveArtifactTextFormats(t *testing.T) {
	for _, kind := range []int{rpc.ArtifactReport, rpc.ArtifactQuiz, rpc.ArtifactDataTable} {
		meta := make([]any, 19)
		meta[7], meta[18] = []any{"# Report"}, []any{[]any{"Name", "Value"}, []any{"x", "a,b"}}
		call := func(context.Context, string, []any, string) (string, error) {
			return metadataResponse(t, []any{meta}), nil
		}
		html := func(context.Context, string) (string, error) { return "<html>Quiz</html>", nil }
		path, err := SaveArtifact(context.Background(), call, html, nil, types.ArtifactInfo{ID: "id", Type: kind}, t.TempDir(), "auto")
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil || len(body) == 0 {
			t.Fatalf("empty output %s, %v", path, err)
		}
	}
}
