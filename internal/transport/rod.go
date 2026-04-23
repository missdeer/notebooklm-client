package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type RodTransport struct {
	mu      sync.RWMutex
	browser *rod.Browser
	page    *rod.Page
	session types.NotebookRpcSession
	opts    RodTransportOptions
}

type RodTransportOptions struct {
	ProfileDir string
	ChromePath string
	Headless   bool
	Proxy      string
	Timeout    time.Duration
}

func NewRodTransport(opts RodTransportOptions) *RodTransport {
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}
	return &RodTransport{opts: opts}
}

// detectBrowserPath finds a locally installed Chromium-based browser.
// Search order matches the TypeScript version: Chrome → Edge → Brave → Chromium.
// Returns empty string if nothing found.
func detectBrowserPath() string {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("PROGRAMFILES")
		programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
		if programFilesX86 == "" {
			programFilesX86 = `C:\Program Files (x86)`
		}
		candidates := []string{
			filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(programFilesX86, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(programFilesX86, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(programFiles, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(localAppData, `BraveSoftware\Brave-Browser\Application\brave.exe`),
			filepath.Join(programFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "darwin":
		home := os.Getenv("HOME")
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			home + "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	default: // linux, freebsd, etc.
		candidates := []string{
			"google-chrome",
			"google-chrome-stable",
			"microsoft-edge",
			"microsoft-edge-stable",
			"brave-browser",
			"brave-browser-stable",
			"chromium",
			"chromium-browser",
		}
		for _, name := range candidates {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
		// Also check absolute paths
		abs := []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/usr/bin/brave-browser",
			"/usr/bin/brave-browser-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
		for _, p := range abs {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func (t *RodTransport) Init(ctx context.Context) error {
	l := launcher.New()

	bin := t.opts.ChromePath
	if bin == "" {
		bin = detectBrowserPath()
	}
	if bin != "" {
		l = l.Bin(bin)
		log.Printf("NotebookLM: Using browser: %s", bin)
	}

	profileDir := t.opts.ProfileDir
	if profileDir == "" {
		profileDir = rpc.ProfileDir()
	}
	isFirstRun := !dirExists(filepath.Join(profileDir, "Default"))
	l = l.UserDataDir(profileDir)

	if t.opts.Headless {
		l = l.Headless(true)
	} else {
		l = l.Headless(false)
	}
	if t.opts.Proxy != "" {
		l = l.Proxy(t.opts.Proxy)
	}

	u, err := l.Launch()
	if err != nil {
		return &types.BrowserError{Msg: fmt.Sprintf("launch chrome: %v", err), Cause: err}
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return &types.BrowserError{Msg: fmt.Sprintf("connect to chrome: %v", err), Cause: err}
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: rpc.DashboardURL})
	if err != nil {
		return &types.BrowserError{Msg: fmt.Sprintf("navigate to dashboard: %v", err), Cause: err}
	}

	if err := page.WaitLoad(); err != nil {
		return &types.BrowserError{Msg: fmt.Sprintf("wait for page load: %v", err), Cause: err}
	}

	if isFirstRun {
		log.Println("NotebookLM: First run — please log in to your Google account.")
	}

	// Wait for user to land on notebooklm.google.com with valid tokens.
	// May go through Google login first — poll up to 3 minutes.
	at, bl, fsid, language := t.waitForTokens(page, 180*time.Second)

	if at == "" {
		// Tokens not found — likely the page came from a login redirect
		// and WIZ_global_data wasn't injected. Reload to get a clean page load.
		currentURL := page.MustInfo().URL
		log.Printf("NotebookLM: Tokens not found at %s, reloading...", currentURL)

		if !strings.Contains(currentURL, "notebooklm.google.com") {
			if err := page.Navigate(rpc.DashboardURL); err != nil {
				return fmt.Errorf("navigate after login: %w", err)
			}
		} else {
			if err := page.Reload(); err != nil {
				return fmt.Errorf("reload after login: %w", err)
			}
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("wait for reload: %w", err)
		}

		at, bl, fsid, language = t.waitForTokens(page, 60*time.Second)
	}

	if at == "" {
		return types.NewSessionError("failed to extract tokens from page (not logged in?)", nil)
	}

	// Use CDP Network.getAllCookies to get ALL browser cookies including HttpOnly ones
	// (SID, HSID, SSID, etc.) across all domains — not just the current page URL.
	// This matches the TS implementation which uses cdp.send('Network.getAllCookies').
	allCookiesResult, err := proto.NetworkGetAllCookies{}.Call(page)
	if err != nil {
		return fmt.Errorf("get all cookies: %w", err)
	}

	var jar []types.SessionCookie
	var cookieParts []string
	seen := make(map[string]bool)
	for _, c := range allCookiesResult.Cookies {
		// Only keep Google domain cookies (matches TS filter)
		if !isGoogleDomain(c.Domain) {
			continue
		}
		var expires *time.Time
		if !c.Session && c.Expires > 0 {
			t := c.Expires.Time().UTC()
			expires = &t
		}
		jar = append(jar, types.SessionCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
			Expires:  expires,
			Session:  c.Session,
		})
		// Deduplicate for flat cookie string
		key := c.Name + "=" + c.Value
		if !seen[key] {
			seen[key] = true
			cookieParts = append(cookieParts, key)
		}
	}

	ua, _ := page.Eval(`() => navigator.userAgent`)
	userAgent := DefaultUserAgent
	if ua != nil && ua.Value.Str() != "" {
		userAgent = ua.Value.Str()
	}

	t.browser = browser
	t.page = page
	t.session = types.NotebookRpcSession{
		AT:        at,
		BL:        bl,
		FSID:      fsid,
		Cookies:   joinStrings(cookieParts, "; "),
		CookieJar: jar,
		UserAgent: userAgent,
		Language:  language,
	}

	log.Printf("NotebookLM: Connected via browser (rod)")
	return nil
}

func (t *RodTransport) Execute(ctx context.Context, req Request) (string, error) {
	t.mu.RLock()
	sess := t.session
	t.mu.RUnlock()

	headers := ChromeHeaders(sess, 0)
	headersJSON, _ := json.Marshal(headers)

	bodyPairs := make(map[string]string)
	for k, v := range req.Body {
		bodyPairs[k] = v
	}
	bodyJSON, _ := json.Marshal(bodyPairs)

	qpJSON, _ := json.Marshal(req.QueryParams)

	script := fmt.Sprintf(`async () => {
		const qp = %s;
		const headers = %s;
		const bodyPairs = %s;
		const params = new URLSearchParams(qp);
		const body = new URLSearchParams(bodyPairs);
		const url = %q + '?' + params.toString();
		headers['Content-Length'] = String(body.toString().length);
		const resp = await fetch(url, {
			method: 'POST',
			headers: headers,
			body: body.toString(),
			credentials: 'include'
		});
		return await resp.text();
	}`, string(qpJSON), string(headersJSON), string(bodyJSON), req.URL)

	result, err := t.page.Eval(script)
	if err != nil {
		return "", fmt.Errorf("rod execute: %w", err)
	}
	return result.Value.Str(), nil
}

func (t *RodTransport) GetSession() types.NotebookRpcSession {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session
}

func (t *RodTransport) RefreshSession(ctx context.Context) error {
	if t.page == nil {
		return fmt.Errorf("no browser page available")
	}
	log.Println("NotebookLM: Refreshing session (reloading page)...")
	if err := t.page.Navigate(rpc.DashboardURL); err != nil {
		return err
	}
	if err := t.page.WaitLoad(); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	result, err := t.page.Eval(`() => {
		const d = window.WIZ_global_data;
		if (!d || !d.SNlM0e) return null;
		return { at: d.SNlM0e, bl: d.cfb2h, fsid: d.FdrFJe };
	}`)
	if err != nil || result == nil {
		return types.NewSessionError("failed to extract tokens after page reload", nil)
	}

	var tokens struct {
		AT   string `json:"at"`
		BL   string `json:"bl"`
		FSID string `json:"fsid"`
	}
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &tokens); err != nil || tokens.AT == "" {
		return types.NewSessionError("token extraction failed after refresh", nil)
	}

	t.mu.Lock()
	t.session.AT = tokens.AT
	if tokens.BL != "" {
		t.session.BL = tokens.BL
	}
	if tokens.FSID != "" {
		t.session.FSID = tokens.FSID
	}
	t.mu.Unlock()

	log.Println("NotebookLM: Session refreshed (browser)")
	return nil
}

func (t *RodTransport) ExportSession() (*types.NotebookRpcSession, error) {
	t.mu.RLock()
	s := t.session
	t.mu.RUnlock()
	return &s, nil
}

func (t *RodTransport) Close() error {
	if t.page != nil {
		t.page.Close()
	}
	if t.browser != nil {
		return t.browser.Close()
	}
	return nil
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// waitForTokens polls the page until it's on notebooklm.google.com with valid WIZ_global_data tokens.
// Returns empty strings if the timeout expires without finding tokens.
func (t *RodTransport) waitForTokens(page *rod.Page, timeout time.Duration) (at, bl, fsid, language string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := page.Eval(`() => {
			if (!location.hostname.includes('notebooklm.google.com')) return null;
			const d = window.WIZ_global_data;
			if (!d || !d.SNlM0e) return null;
			const bl = d.cfb2h || '';
			if (!bl.includes('labs-tailwind')) return null;
			return {
				at: d.SNlM0e || '',
				bl: bl,
				fsid: d.FdrFJe || '',
				lang: document.documentElement.lang || ''
			};
		}`)
		if err == nil && result != nil && result.Value.Str() != "" {
			var tokens struct {
				AT   string `json:"at"`
				BL   string `json:"bl"`
				FSID string `json:"fsid"`
				Lang string `json:"lang"`
			}
			if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &tokens); err == nil && tokens.AT != "" {
				return tokens.AT, tokens.BL, tokens.FSID, tokens.Lang
			}
		}
		time.Sleep(2 * time.Second)
	}
	return "", "", "", ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isGoogleDomain checks if a cookie domain belongs to Google services.
func isGoogleDomain(domain string) bool {
	return strings.HasSuffix(domain, "google.com") ||
		strings.HasSuffix(domain, "googleapis.com") ||
		strings.HasSuffix(domain, "googleusercontent.com")
}
