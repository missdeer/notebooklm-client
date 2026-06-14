package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func CreateNotebook(ctx context.Context, call RpcCaller) (notebookID, threadID string, err error) {
	raw, err := call(ctx, rpc.CreateNotebook,
		[]any{"", nil, nil, copySlice(rpc.PlatformWeb), []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}},
		"/")
	if err != nil {
		return "", "", fmt.Errorf("create notebook: %w", err)
	}
	return parser.ParseCreateNotebook(raw)
}

// ListChatThreads returns chat thread IDs bound to a notebook.
// NotebookLM auto-allocates one default thread per notebook on creation; the
// web UI uses that thread for every chat so messages persist in the chat panel.
func ListChatThreads(ctx context.Context, call RpcCaller, notebookID string) ([]string, error) {
	raw, err := call(ctx, rpc.ListChatThreads,
		[]any{[]any{}, nil, notebookID, 20},
		"/notebook/"+notebookID)
	if err != nil {
		return nil, fmt.Errorf("list chat threads: %w", err)
	}
	return parser.ParseListChatThreads(raw), nil
}

func ListNotebooks(ctx context.Context, call RpcCaller) ([]types.NotebookInfo, error) {
	raw, err := call(ctx, rpc.ListNotebooks, []any{nil, 1, nil, copySlice(rpc.PlatformWeb)}, "/")
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	return parser.ParseListNotebooks(raw), nil
}

func GetNotebookDetail(ctx context.Context, call RpcCaller, notebookID string) (string, []types.SourceInfo, error) {
	raw, err := call(ctx, rpc.GetNotebook,
		[]any{notebookID, nil, copySlice(rpc.PlatformWeb), nil, 1},
		"/notebook/"+notebookID)
	if err != nil {
		return "", nil, fmt.Errorf("get notebook detail: %w", err)
	}
	title, sources := parser.ParseNotebookDetail(raw)
	return title, sources, nil
}

func DeleteNotebook(ctx context.Context, call RpcCaller, notebookID string) error {
	_, err := call(ctx, rpc.DeleteNotebook, []any{[]any{notebookID}, copySlice(rpc.PlatformWeb)}, "/")
	if err != nil {
		return fmt.Errorf("delete notebook: %w", err)
	}
	return nil
}

// NotebookSummary holds the AI-generated notebook overview plus suggested chat
// topic prompts returned by VfAZjd.
type NotebookSummary struct {
	Summary string
	Topics  []TopicSuggestion
}

type TopicSuggestion struct {
	Question string
	Prompt   string
}

func GetNotebookSummary(ctx context.Context, call RpcCaller, notebookID string) (NotebookSummary, error) {
	raw, err := call(ctx, rpc.GetNotebookSummary,
		[]any{notebookID, copySlice(rpc.PlatformWeb)},
		"/notebook/"+notebookID)
	if err != nil {
		return NotebookSummary{}, fmt.Errorf("get notebook summary: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return NotebookSummary{}, nil
	}
	first := envelopes[0]
	var out NotebookSummary
	if len(first) > 0 {
		if arr, ok := first[0].([]any); ok && len(arr) > 0 {
			out.Summary, _ = arr[0].(string)
		}
	}
	if len(first) > 1 {
		if outerTopics, ok := first[1].([]any); ok && len(outerTopics) > 0 {
			if topics, ok := outerTopics[0].([]any); ok {
				for _, t := range topics {
					arr, ok := t.([]any)
					if !ok || len(arr) < 2 {
						continue
					}
					q, _ := arr[0].(string)
					p, _ := arr[1].(string)
					out.Topics = append(out.Topics, TopicSuggestion{Question: q, Prompt: p})
				}
			}
		}
	}
	return out, nil
}

// ConfigureChat updates the notebook-scoped chat goal/style and response length.
// Shares the s0tc2d RPC with RenameNotebook but populates positional slot 7
// (chat settings) instead of slot 3 (title). goal: "default" | "custom" |
// "learning_guide"; responseLength: "default" | "longer" | "shorter".
func ConfigureChat(ctx context.Context, call RpcCaller, notebookID, goal, customPrompt, responseLength string) error {
	goalCode := 1
	switch goal {
	case "custom":
		goalCode = 2
	case "learning_guide":
		goalCode = 3
	}
	if goal == "custom" && customPrompt == "" {
		return fmt.Errorf("configure chat: customPrompt is required when goal=custom")
	}

	lengthCode := 1
	switch responseLength {
	case "longer":
		lengthCode = 4
	case "shorter":
		lengthCode = 5
	}

	var goalSetting []any
	if goal == "custom" {
		goalSetting = []any{goalCode, customPrompt}
	} else {
		goalSetting = []any{goalCode}
	}
	chatSettings := []any{goalSetting, []any{lengthCode}}

	_, err := call(ctx, rpc.RenameNotebook,
		[]any{notebookID, []any{[]any{nil, nil, nil, nil, nil, nil, nil, chatSettings}}},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("configure chat: %w", err)
	}
	return nil
}

func RenameNotebook(ctx context.Context, call RpcCaller, notebookID, newTitle string) error {
	_, err := call(ctx, rpc.RenameNotebook,
		[]any{notebookID, []any{[]any{nil, nil, nil, []any{nil, newTitle}}}},
		"/")
	if err != nil {
		return fmt.Errorf("rename notebook: %w", err)
	}
	return nil
}

func copySlice(src []any) []any {
	dst := make([]any, len(src))
	copy(dst, src)
	return dst
}
