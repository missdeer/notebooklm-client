package transport

import (
	"context"
	"maps"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type Request struct {
	URL         string
	QueryParams map[string]string
	Body        map[string]string
}

// Bind the credentials at execution time, including on a retry after refresh.
// The caller's maps may be reused, so never change them in place.
func requestWithSession(req Request, sess types.NotebookRpcSession) Request {
	if _, authenticated := req.Body["at"]; !authenticated {
		return req
	}
	req.Body = maps.Clone(req.Body)
	req.QueryParams = maps.Clone(req.QueryParams)
	if req.QueryParams == nil {
		req.QueryParams = make(map[string]string)
	}
	req.Body["at"] = sess.AT
	req.QueryParams["bl"] = sess.BL
	delete(req.QueryParams, "f.sid")
	if sess.FSID != "" {
		req.QueryParams["f.sid"] = sess.FSID
	}
	if sess.Language != "" {
		req.QueryParams["hl"] = sess.Language
	}
	return req
}

type Transport interface {
	Execute(ctx context.Context, req Request) (string, error)
	GetSession() types.NotebookRpcSession
	RefreshSession(ctx context.Context) error
	Close() error
}
