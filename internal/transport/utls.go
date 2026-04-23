package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"

	"github.com/missdeer/notebooklm-client/internal/types"
)

type UTLSTransport struct {
	mu               sync.RWMutex
	session          types.NotebookRpcSession
	proxy            string
	onSessionExpired func(context.Context) (*types.NotebookRpcSession, error)
	httpClient       *http.Client
}

type UTLSTransportOptions struct {
	Session          types.NotebookRpcSession
	Proxy            string
	OnSessionExpired func(context.Context) (*types.NotebookRpcSession, error)
}

func NewUTLSTransport(opts UTLSTransportOptions) (*UTLSTransport, error) {
	t := &UTLSTransport{
		session:          opts.Session,
		proxy:            opts.Proxy,
		onSessionExpired: opts.OnSessionExpired,
	}
	t.httpClient = t.createHTTPClient()
	return t, nil
}

func (t *UTLSTransport) Execute(ctx context.Context, req Request) (string, error) {
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
		bodyStr := body.Encode()
		fullURL := req.URL + "?" + qp.Encode()

		httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(bodyStr))
		if err != nil {
			return "", fmt.Errorf("utls execute: %w", err)
		}
		headers := ChromeHeaders(sess, len(bodyStr))
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}

		resp, err := t.httpClient.Do(httpReq)
		if err != nil {
			return "", fmt.Errorf("utls execute: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("utls execute: read body: %w", err)
		}

		if resp.StatusCode == 401 || resp.StatusCode == 400 {
			return "", types.NewSessionError(fmt.Sprintf("HTTP %d", resp.StatusCode), nil)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			snippet := string(respBody)
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
		}
		return string(respBody), nil
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

func (t *UTLSTransport) GetSession() types.NotebookRpcSession {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session
}

func (t *UTLSTransport) RefreshSession(ctx context.Context) error {
	if t.onSessionExpired == nil {
		return types.NewSessionError(
			"session expired and no refresh callback provided", nil)
	}
	log.Println("NotebookLM: Refreshing session (utls transport)...")
	newSession, err := t.onSessionExpired(ctx)
	if err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}
	t.mu.Lock()
	t.session = *newSession
	t.mu.Unlock()
	log.Println("NotebookLM: Session refreshed")
	return nil
}

func (t *UTLSTransport) UpdateSession(s types.NotebookRpcSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.session = s
}

func (t *UTLSTransport) Close() error {
	t.httpClient.CloseIdleConnections()
	return nil
}

func (t *UTLSTransport) createHTTPClient() *http.Client {
	dialer := &net.Dialer{}
	// Use proxy-aware dial so both h1 and h2 route through the proxy.
	dialConn := proxyDialContext(t.proxy, dialer)

	dialTLS := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		rawConn, err := dialConn(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		uConn := utls.UClient(rawConn, &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}, utls.HelloChrome_131)

		if err := uConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, err
		}
		return uConn, nil
	}

	// HTTP/2 transport for when ALPN negotiates h2
	h2Transport := &http2.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialTLS(ctx, network, addr)
		},
	}

	// HTTP/1.1 fallback transport
	h1Transport := &http.Transport{
		DialTLSContext: dialTLS,
	}

	// Use a round-tripper that picks h2 or h1 based on ALPN result
	return &http.Client{
		Transport: &alpnSwitchTransport{
			dialer:      dialer,
			h2:          h2Transport,
			h1:          h1Transport,
			dialTLS:     dialTLS,
			proxy:       t.proxy,
		},
	}
}

// alpnSwitchTransport probes the ALPN result and delegates to h2 or h1.
type alpnSwitchTransport struct {
	dialer  *net.Dialer
	h2      *http2.Transport
	h1      *http.Transport
	dialTLS func(ctx context.Context, network, addr string) (net.Conn, error)
	proxy   string

	once     sync.Once
	useHTTP2 bool
}

func (a *alpnSwitchTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	a.once.Do(func() {
		// Probe the server to check ALPN negotiation
		addr := req.URL.Host
		if !strings.Contains(addr, ":") {
			if req.URL.Scheme == "https" {
				addr += ":443"
			} else {
				addr += ":80"
			}
		}
		conn, err := a.dialTLS(req.Context(), "tcp", addr)
		if err != nil {
			return
		}
		if uConn, ok := conn.(*utls.UConn); ok {
			a.useHTTP2 = uConn.ConnectionState().NegotiatedProtocol == "h2"
		}
		conn.Close()
	})

	if a.useHTTP2 {
		return a.h2.RoundTrip(req)
	}
	return a.h1.RoundTrip(req)
}

func isSessionError(err error, target **types.SessionError) bool {
	for err != nil {
		if se, ok := err.(*types.SessionError); ok {
			*target = se
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
