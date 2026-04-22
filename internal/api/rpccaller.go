package api

import "context"

type RpcCaller func(ctx context.Context, rpcID string, payload []any, sourcePath string) (string, error)

type ChatStreamCaller func(ctx context.Context, notebookID, message string, sourceIDs []string) (string, error)
