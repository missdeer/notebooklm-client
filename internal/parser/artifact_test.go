package parser

import (
	"encoding/json"
	"testing"
)

func TestParseArtifactsResponseShapes(t *testing.T) {
	a := make([]any, 17)
	a[0], a[1], a[2], a[3], a[4] = "slide-id", "Slides", 8, []any{[]any{}}, 3
	a[16] = []any{nil, "en", nil, "https://example.com/a", "https://example.com/b"}
	for _, tc := range []struct {
		value any
		count int
	}{
		{[]any{a}, 1}, {[]any{[]any{a}}, 1}, {[]any{[]any{a, a}}, 2},
		{[]any{[]any{a}, nil}, 1}, {[]any{[]any{a}, []any{a}}, 2},
	} {
		value := tc.value
		inner, _ := json.Marshal(value)
		raw, _ := json.Marshal([]any{[]any{"wrb.fr", "gArtLc", string(inner)}})
		got := ParseArtifacts(string(raw))
		if len(got) != tc.count {
			t.Fatalf("got %d artifacts, want %d for %s", len(got), tc.count, inner)
		}
		for _, item := range got {
			if item.ID != "slide-id" || item.Type != 8 || item.StatusCode == nil || *item.StatusCode != 3 || item.DownloadURL != "https://example.com/b" {
				t.Fatalf("incorrect artifact: %+v", item)
			}
		}
	}
}
