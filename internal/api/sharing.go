package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func GetShareStatus(ctx context.Context, call RpcCaller, notebookID string) (any, error) {
	raw, err := call(ctx, rpc.GetShareStatus,
		[]any{notebookID, copySlice(rpc.PlatformWeb)},
		"/notebook/"+notebookID)
	if err != nil {
		return nil, fmt.Errorf("get share status: %w", err)
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) > 0 {
		return envelopes[0], nil
	}
	return nil, nil
}

func ShareNotebook(ctx context.Context, call RpcCaller, notebookID string, isPublic bool) error {
	access := 0
	if isPublic {
		access = 1
	}
	_, err := call(ctx, rpc.ShareNotebook,
		[]any{[]any{[]any{notebookID, nil, []any{access}, []any{access, ""}}}, 1, nil, copySlice(rpc.PlatformWeb)},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("share notebook: %w", err)
	}
	return nil
}

func ShareNotebookWithUser(ctx context.Context, call RpcCaller, notebookID, email, permission string, notify bool, message string) error {
	if permission == "" {
		permission = "viewer"
	}
	permCode := 3
	if permission == "editor" {
		permCode = 2
	}
	notifyCode := 1
	if !notify {
		notifyCode = 0
	}
	msgFlag := 1
	if message != "" {
		msgFlag = 0
	}
	_, err := call(ctx, rpc.ShareNotebook,
		[]any{[]any{[]any{notebookID, []any{[]any{email, nil, permCode}}, nil, []any{msgFlag, message}}}, notifyCode, nil, copySlice(rpc.PlatformWeb)},
		"/notebook/"+notebookID)
	if err != nil {
		return fmt.Errorf("share notebook with user: %w", err)
	}
	return nil
}
