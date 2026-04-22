package transport

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type CurlTransport struct {
	mu               sync.RWMutex
	session          types.NotebookRpcSession
	binaryPath       string
	proxy            string
	onSessionExpired func(context.Context) (*types.NotebookRpcSession, error)
}

type CurlTransportOptions struct {
	Session          types.NotebookRpcSession
	BinaryPath       string
	Proxy            string
	OnSessionExpired func(context.Context) (*types.NotebookRpcSession, error)
}

func NewCurlTransport(opts CurlTransportOptions) (*CurlTransport, error) {
	binary := opts.BinaryPath
	if binary == "" {
		binary = FindCurlBinary()
	}
	if binary == "" {
		return nil, fmt.Errorf("curl-impersonate binary not found")
	}
	return &CurlTransport{
		session:          opts.Session,
		binaryPath:       binary,
		proxy:            opts.Proxy,
		onSessionExpired: opts.OnSessionExpired,
	}, nil
}

func (t *CurlTransport) Execute(ctx context.Context, req Request) (string, error) {
	doCall := func() (string, error) {
		t.mu.RLock()
		sess := t.session
		t.mu.RUnlock()

		qp := url.Values{}
		for k, v := range req.QueryParams {
			qp.Set(k, v)
		}
		body := url.Values{}
		for k, v := range req.Body {
			body.Set(k, v)
		}
		fullURL := req.URL + "?" + qp.Encode()
		bodyStr := body.Encode()

		headers := ChromeHeaders(sess, len(bodyStr))

		tmpCookieFile, err := writeCookieFile(sess)
		if err != nil {
			return "", fmt.Errorf("curl: write cookies: %w", err)
		}
		defer os.Remove(tmpCookieFile)

		args := []string{
			"--impersonate", "chrome136",
			"-s", "-X", "POST",
			"--cookie", tmpCookieFile,
			"-w", "\n%{http_code}",
			"--max-time", "60",
			"-d", bodyStr,
		}
		for k, v := range headers {
			if strings.EqualFold(k, "Cookie") {
				continue // using cookie file instead
			}
			args = append(args, "-H", fmt.Sprintf("%s: %s", k, v))
		}
		if t.proxy != "" {
			args = append(args, "--proxy", t.proxy)
		}
		args = append(args, fullURL)

		cmd := exec.CommandContext(ctx, t.binaryPath, args...)
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("curl execute: %w", err)
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) < 2 {
			return string(output), nil
		}
		statusLine := lines[len(lines)-1]
		responseBody := strings.Join(lines[:len(lines)-1], "\n")

		if statusLine == "401" || statusLine == "400" {
			return "", types.NewSessionError(fmt.Sprintf("HTTP %s", statusLine), nil)
		}

		return responseBody, nil
	}

	text, err := doCall()
	if err != nil {
		var sessErr *types.SessionError
		if isSessionError(err, &sessErr) && t.onSessionExpired != nil {
			if refreshErr := t.RefreshSession(ctx); refreshErr != nil {
				return "", refreshErr
			}
			return doCall()
		}
		return "", err
	}
	return text, nil
}

func (t *CurlTransport) GetSession() types.NotebookRpcSession {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session
}

func (t *CurlTransport) RefreshSession(ctx context.Context) error {
	if t.onSessionExpired == nil {
		return types.NewSessionError("session expired and no refresh callback provided", nil)
	}
	log.Println("NotebookLM: Refreshing session (curl transport)...")
	newSession, err := t.onSessionExpired(ctx)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.session = *newSession
	t.mu.Unlock()
	log.Println("NotebookLM: Session refreshed")
	return nil
}

func (t *CurlTransport) Close() error { return nil }

func CurlIsAvailable(binaryPath string) bool {
	if binaryPath != "" {
		_, err := exec.LookPath(binaryPath)
		return err == nil
	}
	return FindCurlBinary() != ""
}

func FindCurlBinary() string {
	candidates := []string{"curl-impersonate", "curl_chrome131", "curl_chrome116"}
	if runtime.GOOS == "windows" {
		for i, c := range candidates {
			candidates[i] = c + ".exe"
		}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func writeCookieFile(sess types.NotebookRpcSession) (string, error) {
	f, err := os.CreateTemp("", "notebooklm-cookies-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()

	fmt.Fprintln(f, "# Netscape HTTP Cookie File")
	for _, c := range sess.CookieJar {
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		httpOnly := ""
		if c.HttpOnly {
			httpOnly = "#HttpOnly_"
		}
		domain := c.Domain
		if domain == "" {
			domain = ".google.com"
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		fmt.Fprintf(f, "%s%s\tTRUE\t%s\t%s\t0\t%s\t%s\n",
			httpOnly, domain, path, secure, c.Name, c.Value)
	}

	if len(sess.CookieJar) == 0 && sess.Cookies != "" {
		for _, pair := range strings.Split(sess.Cookies, ";") {
			pair = strings.TrimSpace(pair)
			eq := strings.Index(pair, "=")
			if eq <= 0 {
				continue
			}
			name := pair[:eq]
			value := pair[eq+1:]
			fmt.Fprintf(f, ".google.com\tTRUE\t/\tTRUE\t0\t%s\t%s\n", name, value)
		}
	}

	return f.Name(), nil
}

// Helper to resolve temp dir
func tmpDir() string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir())
}
