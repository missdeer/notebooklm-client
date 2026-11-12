package session

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// CookieHeaderForURL applies browser domain, path, secure and expiry rules.
// The full export also contains accounts.google.com cookies; sending those to
// Gemini Notebook can trigger Google's CookieMismatch redirect even after login.
func CookieHeaderForURL(sess types.NotebookRpcSession, rawURL string) string {
	if len(sess.CookieJar) == 0 {
		return sess.Cookies
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	jar, _ := cookiejar.New(nil)
	for _, c := range sess.CookieJar {
		host := strings.TrimPrefix(c.Domain, ".")
		if host == "" {
			continue
		}
		origin := &url.URL{Scheme: "https", Host: host, Path: "/"}
		cookie := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HttpOnly}
		// CDP prefixes domain cookies with a dot; bare domains are host-only.
		if strings.HasPrefix(c.Domain, ".") {
			cookie.Domain = c.Domain
		}
		if !c.Session && c.Expires != nil {
			cookie.Expires = *c.Expires
		}
		jar.SetCookies(origin, []*http.Cookie{cookie})
	}
	req := &http.Request{Header: make(http.Header)}
	for _, c := range jar.Cookies(target) {
		req.AddCookie(c)
	}
	return req.Header.Get("Cookie")
}
