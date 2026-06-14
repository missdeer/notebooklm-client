package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/payload"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func GenerateArtifact(ctx context.Context, call RpcCaller, notebookID string, sourceIDs []string, sessionLang string, opts types.ArtifactOption) (artifactID, title string, err error) {
	sidsTriple := make([]any, len(sourceIDs))
	sidsDouble := make([]any, len(sourceIDs))
	for i, id := range sourceIDs {
		sidsTriple[i] = []any{[]any{id}}
		sidsDouble[i] = []any{id}
	}

	if opts == nil {
		opts = types.AudioArtifactOptions{Language: sessionLang}
	}

	// Ensure language is set
	switch o := opts.(type) {
	case types.AudioArtifactOptions:
		if o.Language == "" {
			o.Language = sessionLang
		}
		opts = o
	case types.ReportArtifactOptions:
		if o.Language == "" {
			o.Language = sessionLang
		}
		opts = o
	case types.VideoArtifactOptions:
		if o.Language == "" {
			o.Language = sessionLang
		}
		opts = o
	}

	innerPayload := payload.BuildArtifactPayload(sidsTriple, sidsDouble, opts)
	raw, err := call(ctx, rpc.GenerateArtifact,
		[]any{copySlice(rpc.DefaultUserConfig), notebookID, innerPayload},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("generate artifact: %w", err)
	}
	artifactID, title = parser.ParseGenerateArtifact(raw)
	return artifactID, title, nil
}

func GetArtifacts(ctx context.Context, call RpcCaller, notebookID string) ([]types.ArtifactInfo, error) {
	raw, err := call(ctx, rpc.GetArtifactsFiltered,
		[]any{
			copySlice(rpc.DefaultUserConfig),
			notebookID,
			`NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`,
		},
		"/notebook/"+notebookID)
	if err != nil {
		return nil, fmt.Errorf("get artifacts: %w", err)
	}
	return parser.ParseArtifacts(raw), nil
}

func DeleteArtifact(ctx context.Context, call RpcCaller, artifactID string) error {
	_, err := call(ctx, rpc.DeleteArtifact, []any{copySlice(rpc.DefaultUserConfig), artifactID}, "")
	if err != nil {
		return fmt.Errorf("delete artifact: %w", err)
	}
	return nil
}

func RenameArtifact(ctx context.Context, call RpcCaller, artifactID, newTitle string) error {
	_, err := call(ctx, rpc.RenameArtifact, []any{artifactID, newTitle}, "")
	if err != nil {
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

// SlideRevision pairs a 0-based slide index with a free-form instruction
// describing the change to apply.
type SlideRevision struct {
	Index       int
	Instruction string
}

// ReviseSlideDeck submits per-slide revision instructions for an existing slide
// deck artifact. The server creates a NEW slide-deck artifact with the changes
// applied; the original artifact is left untouched.
func ReviseSlideDeck(ctx context.Context, call RpcCaller, artifactID string, revisions []SlideRevision) (newArtifactID, newTitle string, err error) {
	pairs := make([]any, len(revisions))
	for i, r := range revisions {
		pairs[i] = []any{r.Index, r.Instruction}
	}
	raw, err := call(ctx, rpc.ReviseSlide,
		[]any{copySlice(rpc.PlatformWeb), artifactID, []any{pairs}},
		"")
	if err != nil {
		return "", "", fmt.Errorf("revise slide deck: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return "", "", nil
	}
	first := envelopes[0]
	if len(first) == 0 {
		return "", "", nil
	}
	entry, ok := first[0].([]any)
	if !ok || len(entry) == 0 {
		return "", "", nil
	}
	newArtifactID, _ = entry[0].(string)
	if len(entry) > 1 {
		newTitle, _ = entry[1].(string)
	}
	return newArtifactID, newTitle, nil
}

// ExportArtifact exports an artifact to Google Docs (exportType="docs") or
// Sheets (exportType="sheets") and returns the created document URL.
func ExportArtifact(ctx context.Context, call RpcCaller, notebookID, artifactID, title, exportType string) (string, error) {
	code := 1 // docs
	if exportType == "sheets" {
		code = 2
	}
	if title == "" {
		title = "NotebookLM Export"
	}
	raw, err := call(ctx, rpc.ExportArtifact,
		[]any{nil, artifactID, nil, title, code},
		"/notebook/"+notebookID)
	if err != nil {
		return "", fmt.Errorf("export artifact: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return "", nil
	}
	first := envelopes[0]
	if len(first) == 0 {
		return "", nil
	}
	// Response shapes observed: [[[url]]], [[url]], or [url]
	switch v := first[0].(type) {
	case string:
		return v, nil
	case []any:
		if len(v) == 0 {
			return "", nil
		}
		if s, ok := v[0].(string); ok {
			return s, nil
		}
		if sub, ok := v[0].([]any); ok && len(sub) > 0 {
			if s, ok := sub[0].(string); ok {
				return s, nil
			}
		}
	}
	return "", nil
}

func GetInteractiveHTML(ctx context.Context, call RpcCaller, artifactID string) (string, error) {
	raw, err := call(ctx, rpc.GetInteractiveHTML, []any{artifactID}, "")
	if err != nil {
		return "", fmt.Errorf("get interactive html: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return "", nil
	}
	first := envelopes[0]
	if len(first) > 0 {
		if s, ok := first[0].(string); ok {
			return s, nil
		}
		if arr, ok := first[0].([]any); ok {
			for _, el := range arr {
				if s, ok := el.(string); ok && len(s) > 200 {
					return s, nil
				}
				if sub, ok := el.([]any); ok && len(sub) > 0 {
					if s, ok := sub[0].(string); ok && len(s) > 200 {
						return s, nil
					}
				}
			}
		}
	}
	return "", nil
}
