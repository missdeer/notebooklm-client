package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

type Note struct {
	ID      string
	Title   string
	Content string
}

func ListNotes(ctx context.Context, call RpcCaller, notebookID string) ([]Note, error) {
	raw, err := call(ctx, rpc.GetNotes, []any{notebookID}, "/notebook/"+notebookID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return nil, nil
	}
	first := envelopes[0]
	if len(first) == 0 {
		return nil, nil
	}
	items, ok := first[0].([]any)
	if !ok {
		return nil, nil
	}

	var notes []Note
	for _, item := range items {
		arr, ok := item.([]any)
		if !ok || len(arr) < 1 {
			continue
		}
		id, _ := arr[0].(string)
		if id == "" {
			continue
		}
		// Skip mind map entries
		if len(arr) > 2 {
			if code, ok := arr[2].(float64); ok && code == 2 && arr[1] == nil {
				continue
			}
		}
		content := ""
		if len(arr) > 1 {
			if s, ok := arr[1].(string); ok {
				content = s
			} else if sub, ok := arr[1].([]any); ok && len(sub) > 1 {
				content, _ = sub[1].(string)
			}
		}
		// Skip structured content (mind maps)
		if len(content) > 0 && (contains(content, `"children":`) || contains(content, `"nodes":`)) {
			continue
		}
		title := ""
		if len(arr) > 1 {
			if sub, ok := arr[1].([]any); ok && len(sub) > 4 {
				title, _ = sub[4].(string)
			}
		}
		notes = append(notes, Note{ID: id, Title: title, Content: content})
	}
	return notes, nil
}

func CreateNote(ctx context.Context, call RpcCaller, notebookID, title, content string) (string, error) {
	if title == "" {
		title = "New Note"
	}
	raw, err := call(ctx, rpc.CreateNote,
		[]any{notebookID, "", []any{1}, nil, "New Note"},
		"/notebook/"+notebookID)
	if err != nil {
		return "", fmt.Errorf("create note: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	var noteID string
	if len(envelopes) > 0 {
		first := envelopes[0]
		if len(first) > 0 {
			if sub, ok := first[0].([]any); ok && len(sub) > 0 {
				noteID, _ = sub[0].(string)
			} else {
				noteID, _ = first[0].(string)
			}
		}
	}
	if noteID != "" && (title != "New Note" || content != "") {
		if err := UpdateNote(ctx, call, notebookID, noteID, content, title); err != nil {
			return noteID, err
		}
	}
	return noteID, nil
}

func UpdateNote(ctx context.Context, call RpcCaller, notebookID, noteID, content, title string) error {
	_, err := call(ctx, rpc.UpdateNote,
		[]any{notebookID, noteID, []any{[]any{[]any{content, title, []any{}, 0}}}},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	return nil
}

func DeleteNote(ctx context.Context, call RpcCaller, notebookID, noteID string) error {
	_, err := call(ctx, rpc.DeleteNote,
		[]any{notebookID, nil, []any{noteID}},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
