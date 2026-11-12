package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type Deps struct {
	Session    types.NotebookRpcSession
	Proxy      string
	HTTPClient *http.Client
}

type DownloadFn func(ctx context.Context, downloadURL, outputDir, filename string) (string, error)

func DownloadFileHTTP(ctx context.Context, deps Deps, downloadURL, outputDir, filename string) (string, error) {
	client := deps.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	// A jar applies domain-scoped cookies on every redirect, including redirects
	// between googleusercontent.com and usercontent.google.com.
	copyClient := *client
	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	if tr, ok := baseTransport.(*http.Transport); ok {
		tr = tr.Clone()
		// contribution.usercontent.google.com rejects these downloads over Go
		// HTTP/2 with HTTP 400; HTTP/1.1 succeeds (also used by the Python client).
		tr.Protocols = new(http.Protocols)
		tr.Protocols.SetHTTP1(true)
		if tr.TLSClientConfig != nil {
			tr.TLSClientConfig = tr.TLSClientConfig.Clone()
			tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
		}
		copyClient.Transport = tr
		defer tr.CloseIdleConnections()
	}
	copyClient.Jar = downloadCookieJar(deps.Session)
	originalRedirect := client.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if originalRedirect != nil {
			if err := originalRedirect(req, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		// Keep the originating Gemini Notebook page, not an intermediate signed CDN URL.
		req.Header.Set("Referer", rpc.SessionBaseURL(deps.Session.BaseHost)+"/")
		return nil
	}
	client = &copyClient
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outputDir, filename)
	if err := downloadContent(ctx, client, deps.Session, downloadURL, outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

func downloadCookieJar(sess types.NotebookRpcSession) http.CookieJar {
	jar, _ := cookiejar.New(nil)
	cookies := sess.CookieJar
	if len(cookies) == 0 {
		cookies = session.InferCookieJar(sess.Cookies)
	}
	for _, c := range cookies {
		// Gemini Notebook's OSID cookies can cause ServiceLogin redirects on CDN hosts.
		if c.Name == "OSID" || c.Name == "__Secure-OSID" {
			continue
		}
		host := strings.TrimPrefix(c.Domain, ".")
		if host == "" {
			continue
		}
		cookie := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HttpOnly}
		if strings.HasPrefix(c.Domain, ".") {
			cookie.Domain = c.Domain
		}
		if !c.Session && c.Expires != nil {
			cookie.Expires = *c.Expires
		}
		jar.SetCookies(&url.URL{Scheme: "https", Host: host, Path: "/"}, []*http.Cookie{cookie})
		// Gemini Notebook artifact URLs require Google auth on this separate CDN domain.
		// Mirror the Python client's explicit domain mapping, excluding OSID above.
		if c.Domain == ".google.com" {
			cdnCookie := *cookie
			cdnCookie.Domain = ".googleusercontent.com"
			jar.SetCookies(&url.URL{Scheme: "https", Host: "googleusercontent.com", Path: "/"}, []*http.Cookie{&cdnCookie})
		}
	}
	return jar
}

func waitDownloadRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt*10) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isHTMLResponse(body string) bool {
	lower := strings.ToLower(body[:min(500, len(body))])
	return strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<html")
}

func isAuthFailure(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "accounts.google") || strings.Contains(lower, "servicelogin")
}

// GetArtifactMetadata fetches one artifact via the same detail RPC used by TS.
func GetArtifactMetadata(ctx context.Context, call api.RpcCaller, artifactID string) ([]any, error) {
	raw, err := call(ctx, rpc.GetInteractiveHTML, []any{artifactID}, "")
	if err != nil {
		return nil, err
	}
	envelopes := parser.ParseEnvelopes(raw)
	if len(envelopes) == 0 {
		return nil, nil
	}
	first := envelopes[0]
	if len(first) > 0 {
		if arr, ok := first[0].([]any); ok {
			return arr, nil
		}
	}
	return first, nil
}

func PollArtifactMetadata(ctx context.Context, call api.RpcCaller, artifactID string, isReady func([]any) bool, maxAttempts int) ([]any, error) {
	if maxAttempts == 0 {
		maxAttempts = 30
	}
	for i := 0; i < maxAttempts; i++ {
		meta, err := GetArtifactMetadata(ctx, call, artifactID)
		if err != nil {
			return nil, err
		}
		if meta != nil && isReady(meta) {
			return meta, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return nil, fmt.Errorf("artifact metadata poll timed out after %d attempts", maxAttempts)
}
