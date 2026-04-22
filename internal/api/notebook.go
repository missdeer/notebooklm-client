package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func CreateNotebook(ctx context.Context, call RpcCaller) (string, error) {
	raw, err := call(ctx, rpc.CreateNotebook,
		[]any{"", nil, nil, copySlice(rpc.PlatformWeb), []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}},
		"/")
	if err != nil {
		return "", fmt.Errorf("create notebook: %w", err)
	}
	return parser.ParseCreateNotebook(raw)
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
