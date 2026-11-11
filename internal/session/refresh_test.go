package session

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type refreshRoundTripper func(*http.Request) (*http.Response, error)

func (f refreshRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRefreshPreservesLoginHostAndRotatedCookies(t *testing.T) {
	for _, host := range []string{"notebooklm.google.com", "notebook.google.com"} {
		t.Run(host, func(t *testing.T) {
			sess := types.NotebookRpcSession{BaseHost: host, Cookies: "SID=old", AT: "old"}
			client := &http.Client{Transport: refreshRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != host {
					t.Fatalf("refresh used %s", r.URL.Host)
				}
				return &http.Response{StatusCode: 200, Request: r, Header: http.Header{"Set-Cookie": {"SID=new; Domain=.google.com; Path=/; Secure; HttpOnly"}}, Body: io.NopCloser(strings.NewReader(`<html lang="zh-CN"><script>{"SNlM0e":"fresh","cfb2h":"gemini-notebook-new","FdrFJe":"123"}</script>`))}, nil
			})}
			path := filepath.Join(t.TempDir(), "session.json")
			got, err := RefreshTokens(context.Background(), sess, client, path)
			if err != nil {
				t.Fatal(err)
			}
			if got.BaseHost != host || got.AT != "fresh" || got.BL != "gemini-notebook-new" || got.Cookies != "SID=new" || got.Language != "zh" {
				t.Fatalf("unexpected refresh fields")
			}
			loaded, err := Load(path)
			if err != nil || loaded == nil || loaded.BaseHost != host {
				t.Fatalf("login host not persisted: %v", err)
			}
		})
	}
}

func TestRefreshRejectsLoginPageTokens(t *testing.T) {
	client := &http.Client{Transport: refreshRoundTripper(func(r *http.Request) (*http.Response, error) {
		u, _ := url.Parse("https://accounts.google.com/")
		return &http.Response{StatusCode: 200, Request: &http.Request{URL: u}, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"SNlM0e":"login-token"}`))}, nil
	})}
	if _, err := RefreshTokens(context.Background(), types.NotebookRpcSession{}, client, filepath.Join(t.TempDir(), "session.json")); err == nil {
		t.Fatal("accepted Google login token")
	}
}

func TestRefreshLegacySessionProbesRebrandHost(t *testing.T) {
	var hosts []string
	client := &http.Client{Transport: refreshRoundTripper(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		body := `<html>Sign in</html>`
		if r.URL.Host == "notebook.google.com" {
			body = `{"SNlM0e":"fresh","cfb2h":"new-build"}`
		}
		return &http.Response{StatusCode: 200, Request: r, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := RefreshTokens(context.Background(), types.NotebookRpcSession{Cookies: "SID=old"}, client, filepath.Join(t.TempDir(), "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(hosts, ",") != "notebooklm.google.com,notebook.google.com" || got.BaseHost != "notebook.google.com" {
		t.Fatal("legacy host fallback failed")
	}
}
