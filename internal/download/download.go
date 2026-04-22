package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/parser"
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
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outputDir, filename)

	maxRetries := 10
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return "", err
		}
		ua := deps.Session.UserAgent
		if ua == "" {
			ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")
		if deps.Session.Cookies != "" {
			req.Header.Set("Cookie", deps.Session.Cookies)
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*10) * time.Second)
				continue
			}
			return "", fmt.Errorf("download: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("download read: %w", err)
		}

		bodyStr := string(body)
		if resp.StatusCode == 404 || isHTMLResponse(bodyStr) {
			if isAuthFailure(bodyStr) {
				return "", fmt.Errorf("download auth failure: login page returned (cookies may be expired)")
			}
			if attempt < maxRetries {
				log.Printf("NotebookLM: CDN not ready (attempt %d/%d), retrying...", attempt, maxRetries)
				time.Sleep(time.Duration(attempt*10) * time.Second)
				continue
			}
			return "", fmt.Errorf("download failed after %d retries (CDN propagation)", maxRetries)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
		}

		if err := os.WriteFile(outPath, body, 0o644); err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}
		return outPath, nil
	}
	return "", fmt.Errorf("download failed: exhausted retries")
}

func isHTMLResponse(body string) bool {
	lower := strings.ToLower(body[:min(500, len(body))])
	return strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<html")
}

func isAuthFailure(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "accounts.google") || strings.Contains(lower, "servicelogin")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetArtifactMetadata fetches raw artifact metadata via GetArtifactsFiltered.
func GetArtifactMetadata(ctx context.Context, call api.RpcCaller, artifactID string) ([]any, error) {
	// Re-use the parser.ParseEnvelopes approach
	raw, err := call(ctx, "gArtLc", []any{
		[]any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}, []any{[]any{2, 1, 3}}},
		nil,
		fmt.Sprintf(`artifact.id = "%s"`, artifactID),
	}, "")
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
			if len(arr) > 0 {
				if inner, ok := arr[0].([]any); ok {
					return inner, nil
				}
			}
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
