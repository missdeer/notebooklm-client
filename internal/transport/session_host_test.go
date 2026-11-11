package transport

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type hostRoundTripper func(*http.Request) (*http.Response, error)

func (f hostRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPRequestsUseExportedLoginHost(t *testing.T) {
	for _, endpoint := range []string{rpc.BatchExecuteURL, rpc.ChatStreamURL} {
		t.Run(endpoint, func(t *testing.T) {
			tr, err := NewUTLSTransport(UTLSTransportOptions{Session: types.NotebookRpcSession{BaseHost: "notebook.google.com", Cookies: "SID=test"}})
			if err != nil {
				t.Fatal(err)
			}
			tr.httpClient = &http.Client{Transport: hostRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "notebook.google.com" || r.Header.Get("Origin") != "https://notebook.google.com" || r.Header.Get("Referer") != "https://notebook.google.com/" || r.Header.Get("Cookie") != "SID=test" {
					t.Fatal("request did not preserve login host and credentials")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
			})}
			if _, err := tr.Execute(context.Background(), Request{URL: endpoint, Body: map[string]string{"at": "test"}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHTTPRetryRebindsRefreshedCredentials(t *testing.T) {
	for _, failure := range []struct {
		name   string
		status int
		body   string
	}{
		{"HTTP 401", 401, "unauthorized"},
		{"HTTP 400", 400, "expired token"},
		{"in-body unauthenticated", 200, `[["wrb.fr","test-rpc",null,null,null,[16],"generic"]]`},
	} {
		t.Run(failure.name, func(t *testing.T) {
			old := types.NotebookRpcSession{AT: "old-at", BL: "old-bl", FSID: "old-sid", Cookies: "SID=old", Language: "en"}
			fresh := types.NotebookRpcSession{AT: "new-at", BL: "new-bl", FSID: "new-sid", Cookies: "SID=new", Language: "zh", BaseHost: "notebook.google.com"}
			refreshes, calls := 0, 0
			tr, err := NewUTLSTransport(UTLSTransportOptions{Session: old, OnSessionExpired: func(context.Context) (*types.NotebookRpcSession, error) { refreshes++; return &fresh, nil }})
			if err != nil {
				t.Fatal(err)
			}
			tr.httpClient = &http.Client{Transport: hostRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				raw, _ := io.ReadAll(r.Body)
				body, _ := url.ParseQuery(string(raw))
				want := old
				if calls == 2 {
					want = fresh
				}
				if body.Get("at") != want.AT || r.URL.Query().Get("bl") != want.BL || r.URL.Query().Get("f.sid") != want.FSID || r.URL.Query().Get("hl") != want.Language || r.Header.Get("Cookie") != want.Cookies || r.URL.Host != strings.TrimPrefix(rpc.SessionBaseURL(want.BaseHost), "https://") {
					t.Fatal("request uses stale or inconsistent credentials")
				}
				if body.Get("f.req") != "payload" || r.URL.Query().Get("rpcids") != "test-rpc" {
					t.Fatal("RPC payload changed")
				}
				status, response := 200, "ok"
				if calls == 1 {
					status, response = failure.status, failure.body
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(response))}, nil
			})}
			req := Request{URL: rpc.BatchExecuteURL, QueryParams: map[string]string{"bl": old.BL, "f.sid": old.FSID, "hl": old.Language, "rpcids": "test-rpc"}, Body: map[string]string{"at": old.AT, "f.req": "payload"}}
			got, err := tr.Execute(context.Background(), req)
			if err != nil || got != "ok" || calls != 2 || refreshes != 1 {
				t.Fatalf("retry failed: %v, calls=%d, refreshes=%d", err, calls, refreshes)
			}
			if req.Body["at"] != old.AT || req.QueryParams["bl"] != old.BL {
				t.Fatal("modified caller's request")
			}
		})
	}
}

func TestRequestWithSessionRemovesStaleSessionID(t *testing.T) {
	req := requestWithSession(Request{Body: map[string]string{"at": "old"}, QueryParams: map[string]string{"f.sid": "old-sid"}}, types.NotebookRpcSession{AT: "new"})
	if _, exists := req.QueryParams["f.sid"]; exists {
		t.Fatal("stale session ID survived refresh")
	}
}
