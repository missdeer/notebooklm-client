package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func CreateWebSearch(ctx context.Context, call RpcCaller, notebookID, query string, mode types.ResearchMode) (researchID, artifactID string, err error) {
	if mode == types.ResearchDeep {
		return createDeepResearch(ctx, call, notebookID, query)
	}
	raw, err := call(ctx, rpc.CreateWebSearch,
		[]any{[]any{query, 1}, nil, 1, notebookID},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("create web search: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) > 0 {
		if arr := envelopes[0]; len(arr) > 0 {
			researchID, _ = arr[0].(string)
		}
	}
	return researchID, "", nil
}

func createDeepResearch(ctx context.Context, call RpcCaller, notebookID, query string) (string, string, error) {
	raw, err := call(ctx, rpc.CreateDeepResearch,
		[]any{nil, []any{1}, []any{query, 1}, 5, notebookID},
		"/notebook/"+notebookID)
	if err != nil {
		return "", "", fmt.Errorf("create deep research: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	var researchID, artifactID string
	if len(envelopes) > 0 {
		first := envelopes[0]
		if len(first) > 0 {
			researchID, _ = first[0].(string)
		}
		if len(first) > 1 {
			artifactID, _ = first[1].(string)
		}
	}
	return researchID, artifactID, nil
}

func PollResearchResults(ctx context.Context, call RpcCaller, notebookID string, timeout time.Duration) ([]types.ResearchResult, string, error) {
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := call(ctx, rpc.PollResearch,
			[]any{nil, nil, notebookID},
			"/notebook/"+notebookID)
		if err != nil {
			return nil, "", fmt.Errorf("poll research: %w", err)
		}
		parsed := parser.ParseResearchResults(raw)
		if parsed.Status >= 2 {
			log.Printf("NotebookLM: Research completed — %d sources", len(parsed.Results))
			return parsed.Results, parsed.Report, nil
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	log.Println("NotebookLM: Research poll timed out")
	return nil, "", nil
}

func ImportResearch(ctx context.Context, call RpcCaller, notebookID, researchID string, results []types.ResearchResult, report string) error {
	var sources []any
	if report != "" {
		sources = append(sources, []any{nil, []any{"Deep Research Report", report}, nil, 3, nil, nil, nil, nil, nil, nil, 3})
	}
	for _, r := range results {
		sources = append(sources, []any{nil, nil, []any{r.URL, r.Title}, nil, nil, nil, nil, nil, nil, nil, 2})
	}
	if len(sources) == 0 {
		return nil
	}
	_, err := call(ctx, rpc.ImportResearch,
		[]any{nil, []any{1}, researchID, notebookID, sources},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("import research: %w", err)
	}
	log.Printf("NotebookLM: Imported %d research sources", len(sources))
	return nil
}
