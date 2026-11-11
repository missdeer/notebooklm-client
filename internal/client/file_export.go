package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/transport"
)

func trustedExportURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range []string{"google.com", "googleusercontent.com", "usercontent.google"} {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// ResolveFileExport handles both direct exports and Google's JSON viewer envelope.
func (c *NotebookClient) ResolveFileExport(ctx context.Context, direct, viewer string) (string, string, error) {
	if direct != "" {
		if !trustedExportURL(direct) {
			return "", "", fmt.Errorf("file download URL must be an HTTPS Google endpoint")
		}
		return direct, "", nil
	}
	if !trustedExportURL(viewer) {
		return "", "", fmt.Errorf("file viewer URL must be an HTTPS Google endpoint")
	}
	sess := c.GetRpcSession()
	if sess == nil {
		return "", "", fmt.Errorf("not connected")
	}
	httpClient := transport.NewUTLSHTTPClient(c.proxy)
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || !trustedExportURL(req.URL.String()) {
			return fmt.Errorf("untrusted file viewer redirect")
		}
		req.Header.Set("Cookie", session.CookieHeaderForURL(*sess, req.URL.String()))
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", viewer, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Cookie", session.CookieHeaderForURL(*sess, viewer))
	req.Header.Set("User-Agent", sess.UserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("file viewer HTTP %d", resp.StatusCode)
	}
	var data map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return "", "", fmt.Errorf("file viewer JSON: %w", err)
	}
	for _, key := range []string{"pdf", "url", "downloadUrl", "download_url"} {
		if target, ok := data[key].(string); ok && target != "" {
			if !trustedExportURL(target) {
				return "", "", fmt.Errorf("untrusted viewer target")
			}
			format := ""
			if key == "pdf" {
				format = "pdf"
			}
			return target, format, nil
		}
	}
	return "", "", fmt.Errorf("viewer contains no download URL")
}
