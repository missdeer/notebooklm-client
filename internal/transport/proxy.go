package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

// NewProxyHTTPClient creates an *http.Client that routes traffic through the given proxy.
// Supports http, https, and socks5 proxy URLs with optional user:pass authentication.
// If proxyURL is empty, returns a default http.Client.
func NewProxyHTTPClient(proxyURL string) *http.Client {
	if proxyURL == "" {
		return &http.Client{}
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: func(_ *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
		},
	}
}

// proxyDialContext returns a dial function that connects through the given proxy.
// This is used by the uTLS transport where we need proxy tunneling at the raw TCP level
// (before custom TLS handshake), since http.Transport.Proxy doesn't work with DialTLSContext.
// If proxyURL is empty, returns the base dialer's DialContext.
func proxyDialContext(proxyURL string, dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if proxyURL == "" {
		return dialer.DialContext
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return dialer.DialContext
	}

	switch parsed.Scheme {
	case "socks5", "socks5h":
		return socks5DialContext(parsed, dialer)
	case "http", "https":
		return httpConnectDialContext(parsed, dialer)
	default:
		return dialer.DialContext
	}
}

func socks5DialContext(proxyURL *url.URL, dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		var auth *proxy.Auth
		if proxyURL.User != nil {
			auth = &proxy.Auth{User: proxyURL.User.Username()}
			if p, ok := proxyURL.User.Password(); ok {
				auth.Password = p
			}
		}
		host := proxyHostPort(proxyURL)
		socks, err := proxy.SOCKS5("tcp", host, auth, dialer)
		if err != nil {
			return nil, fmt.Errorf("socks5 proxy: %w", err)
		}
		if cd, ok := socks.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, network, addr)
		}
		return socks.Dial(network, addr)
	}
}

func httpConnectDialContext(proxyURL *url.URL, dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host := proxyHostPort(proxyURL)

		var conn net.Conn
		var err error
		if proxyURL.Scheme == "https" {
			tlsDialer := &tls.Dialer{NetDialer: dialer}
			conn, err = tlsDialer.DialContext(ctx, "tcp", host)
		} else {
			conn, err = dialer.DialContext(ctx, "tcp", host)
		}
		if err != nil {
			return nil, fmt.Errorf("connect to proxy %s: %w", host, err)
		}

		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
		if proxyURL.User != nil {
			creds := proxyURL.User.String()
			encoded := base64.StdEncoding.EncodeToString([]byte(creds))
			connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", encoded)
		}
		connectReq += "\r\n"

		if _, err := conn.Write([]byte(connectReq)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("proxy CONNECT write: %w", err)
		}

		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("proxy CONNECT response: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			conn.Close()
			return nil, fmt.Errorf("proxy CONNECT failed: HTTP %d", resp.StatusCode)
		}

		return conn, nil
	}
}

func proxyHostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	switch u.Scheme {
	case "https":
		return u.Hostname() + ":443"
	case "socks5", "socks5h":
		return u.Hostname() + ":1080"
	default:
		return u.Hostname() + ":80"
	}
}
