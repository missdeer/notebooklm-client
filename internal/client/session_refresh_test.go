package client

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type refreshRoundTripper func(*http.Request) (*http.Response, error)

func (f refreshRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFailedRefreshDoesNotAcceptUnchangedDiskSession(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "another process exported"}[changed], func(t *testing.T) {
			expired := types.NotebookRpcSession{AT: "old-at", BL: "old-bl", Cookies: "SID=old", BaseHost: "notebooklm.google.com"}
			onDisk := expired
			if changed {
				onDisk.AT = "new-at"
				onDisk.Cookies = "SID=new"
			}
			path := filepath.Join(t.TempDir(), "session.json")
			if _, err := session.Save(onDisk, path); err != nil {
				t.Fatal(err)
			}
			hc := &http.Client{Transport: refreshRoundTripper(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 401, Request: r, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
			})}
			got, err := refreshHeadlessSession(context.Background(), expired, hc, path)
			if changed {
				if err != nil || got == nil || got.AT != "new-at" {
					t.Fatalf("did not load updated session: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "export-session") {
				t.Fatalf("refresh failure was hidden: %v", err)
			}
		})
	}
}
