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
