package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func refreshHeadlessSession(ctx context.Context, expired types.NotebookRpcSession, httpClient *http.Client, path string) (*types.NotebookRpcSession, error) {
	refreshed, err := session.RefreshTokens(ctx, expired, httpClient, path)
	if err == nil {
		return refreshed, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// Another process may have exported a new session while this call ran.
	// Loading the same rejected credentials is not a successful recovery.
	fromDisk, loadErr := session.Load(path)
	if loadErr == nil && fromDisk != nil &&
		(fromDisk.AT != expired.AT || fromDisk.BL != expired.BL ||
			fromDisk.FSID != expired.FSID || fromDisk.Cookies != expired.Cookies ||
			fromDisk.BaseHost != expired.BaseHost) {
		return fromDisk, nil
	}
	return nil, types.NewSessionError(fmt.Sprintf("Session expired and auto-refresh failed: %v. Re-run `export-session`.", err), err)
}
