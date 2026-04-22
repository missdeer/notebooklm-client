package session

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

var (
	reAT   = regexp.MustCompile(`"SNlM0e":"([^"]+)"`)
	reBL   = regexp.MustCompile(`"cfb2h":"([^"]+)"`)
	reFSID = regexp.MustCompile(`"FdrFJe":"([^"]+)"`)
	reLang = regexp.MustCompile(`<html[^>]*\slang="([^"]+)"`)
)

// RefreshTokens refreshes short-lived tokens (at, bl, fsid) using long-lived cookies.
// Makes a GET request to the dashboard and extracts WIZ_global_data values from HTML.
func RefreshTokens(ctx context.Context, session types.NotebookRpcSession, httpClient *http.Client, savePath string) (*types.NotebookRpcSession, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	ua := session.UserAgent
	if ua == "" {
		ua = defaultUA
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rpc.DashboardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("refresh tokens: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Cookie", session.Cookies)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh tokens: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("refresh tokens: read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token refresh failed: HTTP %d", resp.StatusCode)
	}

	html := string(body)
	atMatch := reAT.FindStringSubmatch(html)
	if len(atMatch) < 2 {
		return nil, fmt.Errorf("token refresh failed: SNlM0e not found in page (cookies may be expired)")
	}

	blMatch := reBL.FindStringSubmatch(html)
	fsidMatch := reFSID.FindStringSubmatch(html)
	langMatch := reLang.FindStringSubmatch(html)

	updatedCookies := MergeCookies(session.Cookies, resp.Header["Set-Cookie"])

	refreshed := types.NotebookRpcSession{
		AT:        atMatch[1],
		BL:        orDefault(blMatch, session.BL),
		FSID:      orDefault(fsidMatch, session.FSID),
		Cookies:   updatedCookies,
		CookieJar: InferCookieJar(updatedCookies),
		UserAgent: session.UserAgent,
		Language:  extractLang(langMatch, session.Language),
	}

	filePath := savePath
	if filePath == "" {
		filePath = rpc.SessionPath()
	}
	if savedPath, err := Save(refreshed, filePath); err == nil {
		log.Printf("NotebookLM: Tokens refreshed and saved to %s", savedPath)
	}

	return &refreshed, nil
}

// MergeCookies merges existing cookies with new Set-Cookie headers.
func MergeCookies(existing string, setCookieHeaders []string) string {
	cookieMap := make(map[string]string)
	for _, pair := range strings.Split(existing, "; ") {
		eq := strings.Index(pair, "=")
		if eq > 0 {
			cookieMap[pair[:eq]] = pair[eq+1:]
		}
	}
	for _, h := range setCookieHeaders {
		nameValue, _, _ := strings.Cut(h, ";")
		nameValue = strings.TrimSpace(nameValue)
		eq := strings.Index(nameValue, "=")
		if eq > 0 {
			cookieMap[strings.TrimSpace(nameValue[:eq])] = strings.TrimSpace(nameValue[eq+1:])
		}
	}
	parts := make([]string, 0, len(cookieMap))
	for k, v := range cookieMap {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func orDefault(match []string, fallback string) string {
	if len(match) >= 2 {
		return match[1]
	}
	return fallback
}

func extractLang(match []string, fallback string) string {
	if len(match) >= 2 {
		lang := match[1]
		if idx := strings.Index(lang, "-"); idx > 0 {
			return lang[:idx]
		}
		return lang
	}
	return fallback
}
