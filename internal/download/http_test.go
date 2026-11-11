package download

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestDownloadUsesHTTP1ForCDNCompatibility(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 1 {
			t.Errorf("CDN requires HTTP/1, got %s", r.Proto)
			w.WriteHeader(400)
			return
		}
		w.Write([]byte("%PDF-test"))
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	path, err := DownloadFileHTTP(context.Background(), Deps{HTTPClient: server.Client()}, server.URL, t.TempDir(), "slides.pdf")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "%PDF-test" {
		t.Fatalf("bad output %q, %v", body, err)
	}
}

type downloadRoundTripper func(*http.Request) (*http.Response, error)

func (f downloadRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadRedirectAuthentication(t *testing.T) {
	sess := types.NotebookRpcSession{BaseHost: "notebook.google.com", CookieJar: []types.SessionCookie{
		{Name: "SID", Value: "google-auth", Domain: ".google.com", Path: "/", Secure: true, Session: true},
		{Name: "OSID", Value: "not-for-cdn", Domain: ".google.com", Path: "/", Secure: true, Session: true},
		{Name: "__Secure-OSID", Value: "not-for-cdn", Domain: ".google.com", Path: "/", Secure: true, Session: true},
	}}
	calls := 0
	client := &http.Client{Transport: downloadRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Referer") != "https://notebook.google.com/" || r.Header.Get("Sec-Fetch-Site") != "cross-site" {
			t.Error("download navigation headers missing")
		}
		cookies := r.Header.Get("Cookie")
		if !strings.Contains(cookies, "SID=google-auth") || strings.Contains(cookies, "OSID=") {
			t.Errorf("incorrect download cookie selection")
		}
		response := &http.Response{StatusCode: 200, ContentLength: -1, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("%PDF-test")), Request: r}
		if calls == 1 {
			response.StatusCode = 302
			response.Header.Set("Location", "https://contribution.usercontent.google.com/download")
		}
		return response, nil
	})}
	_, err := DownloadFileHTTP(context.Background(), Deps{Session: sess, HTTPClient: client}, "https://lh3.googleusercontent.com/download", t.TempDir(), "slides.pdf")
	if err != nil || calls != 2 {
		t.Fatalf("redirect download failed: %v, calls %d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitDownloadRetry(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("retry ignores cancellation: %v", err)
	}
}
