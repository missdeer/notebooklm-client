package transport

import (
	"context"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type Request struct {
	URL         string
	QueryParams map[string]string
	Body        map[string]string
}

type Transport interface {
	Execute(ctx context.Context, req Request) (string, error)
	GetSession() types.NotebookRpcSession
	RefreshSession(ctx context.Context) error
	Close() error
}
