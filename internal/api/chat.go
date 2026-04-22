package api

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
)

func SendChat(ctx context.Context, callChat ChatStreamCaller, notebookID, message string, sourceIDs []string) (text, threadID string, err error) {
	raw, err := callChat(ctx, notebookID, message, sourceIDs)
	if err != nil {
		return "", "", fmt.Errorf("send chat: %w", err)
	}
	text, threadID, _ = parser.ParseChatStream(raw)
	return text, threadID, nil
}

func DeleteChatThread(ctx context.Context, call RpcCaller, threadID string) error {
	_, err := call(ctx, rpc.DeleteChatThread, []any{[]any{}, threadID, nil, 1}, "")
	if err != nil {
		return fmt.Errorf("delete chat thread: %w", err)
	}
	return nil
}
