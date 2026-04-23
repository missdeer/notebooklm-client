package session

import (
	"net/http"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// cookieKey identifies a cookie per RFC 6265 §5.3: the tuple (name, domain,
// path). Domain is normalized (lowercased, leading "." stripped) so that
// ".google.com" and "google.com" — which differ only in the leading-dot
// cosmetic — collapse to the same key.
//
// Path is normalized to "/" when absent. RFC 6265 §5.1.4 defines the default
// Path from the request URI, which we don't have here; "/" is the pragmatic
// default that matches browser behavior for the root-scoped auth cookies we
// care about. Without this, a Set-Cookie header that omits the Path attribute
// (parsed.Path == "") would fail to match an existing Path="/" jar entry and
// insert a duplicate instead of rotating in place.
type cookieKey struct {
	name, domain, path string
}

func makeCookieKey(name, domain, path string) cookieKey {
	p := path
	if p == "" {
		p = "/"
	}
	return cookieKey{
		name:   name,
		domain: strings.TrimPrefix(strings.ToLower(domain), "."),
		path:   p,
	}
}

// MergeCookieJar merges Set-Cookie response headers into an existing cookie jar.
//
// Behavior (RFC 6265-ish, pragmatic subset sufficient for Google session cookies):
//   - Each Set-Cookie is parsed by net/http's ParseSetCookie so that Expires,
//     Max-Age, Domain, Path, Secure, HttpOnly are respected.
//   - Cookies are identified by (name, domain, path). The jar can therefore
//     hold cookies with the same name scoped to different domains
//     (e.g. `NID` on `.google.com` vs `.youtube.com`) without collision.
//   - Max-Age takes priority over Expires when both are present.
//   - Max-Age <= 0, or an Expires in the past, is treated as deletion — the
//     corresponding (name, domain, path) entry is removed.
//   - Cookies not mentioned by any Set-Cookie header retain their existing
//     value and, crucially, their existing Expires metadata.
//   - When a Set-Cookie header omits Domain, we fall back to name-only
//     matching *only if the existing jar has exactly one cookie by that
//     name* — this preserves Domain for host-only rotations (server re-sends
//     PSID without repeating Domain) without silently pulling values onto
//     the wrong-domain entry when multiple same-name cookies exist.
//   - Secure/HttpOnly are never downgraded.
//   - Insertion order is preserved; new cookies appear at the end.
func MergeCookieJar(prev []types.SessionCookie, setCookieHeaders []string, now time.Time) []types.SessionCookie {
	byKey := make(map[cookieKey]*types.SessionCookie, len(prev))
	order := make([]cookieKey, 0, len(prev))
	for i := range prev {
		c := prev[i]
		k := makeCookieKey(c.Name, c.Domain, c.Path)
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = &c
	}

	// Build a snapshot of the original jar's name → keys index so that
	// Set-Cookie headers with no Domain attribute can still be matched to an
	// existing entry when unambiguous. Only populated from `prev`; not
	// maintained as new cookies get inserted below (those wouldn't help
	// disambiguate the next header anyway).
	initialByName := make(map[string][]cookieKey, len(byKey))
	for k := range byKey {
		initialByName[k.name] = append(initialByName[k.name], k)
	}

	for _, header := range setCookieHeaders {
		parsed, err := http.ParseSetCookie(header)
		if err != nil || parsed == nil || parsed.Name == "" {
			continue
		}

		newExp, deletion := effectiveExpiry(parsed, now)

		var matchedKey cookieKey
		var haveMatch bool
		switch {
		case parsed.Domain != "":
			matchedKey = makeCookieKey(parsed.Name, parsed.Domain, parsed.Path)
			_, haveMatch = byKey[matchedKey]
		default:
			// No Domain attribute: match by name only if exactly one
			// candidate existed in the original jar.
			if cands := initialByName[parsed.Name]; len(cands) == 1 {
				matchedKey = cands[0]
				_, haveMatch = byKey[matchedKey]
			} else {
				// Ambiguous or no candidate: treat as a new host-only
				// cookie (Domain left empty).
				matchedKey = makeCookieKey(parsed.Name, "", parsed.Path)
				_, haveMatch = byKey[matchedKey]
			}
		}

		if deletion {
			if haveMatch {
				delete(byKey, matchedKey)
			}
			continue
		}

		if !haveMatch {
			sc := types.SessionCookie{
				Name:     parsed.Name,
				Value:    parsed.Value,
				Domain:   parsed.Domain,
				Path:     parsed.Path,
				Secure:   parsed.Secure,
				HttpOnly: parsed.HttpOnly,
				Expires:  newExp,
			}
			byKey[matchedKey] = &sc
			order = append(order, matchedKey)
			continue
		}

		existing := byKey[matchedKey]
		existing.Value = parsed.Value
		if newExp != nil {
			existing.Expires = newExp
		}
		if parsed.Domain != "" {
			existing.Domain = parsed.Domain
		}
		if parsed.Path != "" {
			existing.Path = parsed.Path
		}
		if parsed.Secure {
			existing.Secure = true
		}
		if parsed.HttpOnly {
			existing.HttpOnly = true
		}
	}

	out := make([]types.SessionCookie, 0, len(byKey))
	for _, k := range order {
		if c, ok := byKey[k]; ok {
			out = append(out, *c)
		}
	}
	return out
}

// effectiveExpiry returns the absolute expiration time to apply for a parsed
// Set-Cookie, following RFC 6265 §5.3 (Max-Age wins over Expires). The second
// return value is true when the cookie should be deleted from the jar.
func effectiveExpiry(c *http.Cookie, now time.Time) (*time.Time, bool) {
	if c.MaxAge != 0 {
		if c.MaxAge < 0 {
			return nil, true
		}
		t := now.Add(time.Duration(c.MaxAge) * time.Second).UTC()
		return &t, false
	}
	if !c.Expires.IsZero() {
		t := c.Expires.UTC()
		if !t.After(now) {
			return nil, true
		}
		return &t, false
	}
	// No expiration info in header: leave whatever the existing entry has.
	return nil, false
}

// FlattenCookies builds a "name=value; name=value" Cookie request-header string
// from a jar, preserving order and deduplicating by name+value.
func FlattenCookies(jar []types.SessionCookie) string {
	seen := make(map[string]bool, len(jar))
	parts := make([]string, 0, len(jar))
	for _, c := range jar {
		if c.Name == "" {
			continue
		}
		key := c.Name + "=" + c.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, key)
	}
	return strings.Join(parts, "; ")
}
