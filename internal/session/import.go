package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// CookieSnapshot pairs a cookie with the most recent timestamp we know about
// it (Firefox lastAccessed / Safari creationDate) plus a human-readable label
// describing where it came from. Used by multi-source discovery to pick the
// freshest value when the same (name, domain) appears in several profiles.
type CookieSnapshot struct {
	Cookie   types.SessionCookie
	LastSeen time.Time
	Source   string
}

// CookieSource tags where the imported cookies came from.
type CookieSource string

const (
	SourceNetscape CookieSource = "netscape"
	SourceFirefox  CookieSource = "firefox"
	SourceSafari   CookieSource = "safari"
)

// FilterGoogleCookies narrows a cookie list to google.com family domains.
// Also deduplicates by (name, domain) keeping the last entry (typical DB order
// puts newer writes later; for Netscape files, later lines win).
func FilterGoogleCookies(cookies []types.SessionCookie) []types.SessionCookie {
	type key struct{ name, domain string }
	seen := make(map[key]int) // map to index in out
	var out []types.SessionCookie
	for _, c := range cookies {
		if !IsGoogleDomain(c.Domain) {
			continue
		}
		k := key{c.Name, c.Domain}
		if idx, ok := seen[k]; ok {
			out[idx] = c
			continue
		}
		seen[k] = len(out)
		out = append(out, c)
	}
	// Stable sort for deterministic output.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// IsGoogleDomain reports whether a cookie Domain belongs to a Google service
// we care about (google.com, youtube.com, googleusercontent.com and their
// subdomains). Case-insensitive; accepts leading "." per cookie convention.
//
// The suffix matches are anchored so names like "google.com.evil.com" are
// rejected.
func IsGoogleDomain(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	return d == "google.com" ||
		strings.HasSuffix(d, ".google.com") ||
		d == "youtube.com" ||
		strings.HasSuffix(d, ".youtube.com") ||
		d == "googleusercontent.com" ||
		strings.HasSuffix(d, ".googleusercontent.com")
}

// DefaultFirefoxProfile attempts to locate the user's default Firefox profile
// directory. Returns ("", err) if unable.
func DefaultFirefoxProfile() (string, error) {
	roots, err := firefoxProfileRoots()
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		candidates, err := filepath.Glob(filepath.Join(root, "*.default*"))
		if err != nil {
			continue
		}
		// Prefer entries ending in ".default-release" / ".default".
		sort.Slice(candidates, func(i, j int) bool {
			pi := firefoxProfilePriority(candidates[i])
			pj := firefoxProfilePriority(candidates[j])
			return pi < pj
		})
		for _, c := range candidates {
			db := filepath.Join(c, "cookies.sqlite")
			if fi, err := os.Stat(db); err == nil && !fi.IsDir() {
				return c, nil
			}
		}
	}
	return "", errors.New("no Firefox profile with cookies.sqlite found; pass --profile explicitly")
}

func firefoxProfilePriority(path string) int {
	base := filepath.Base(path)
	switch {
	case strings.HasSuffix(base, ".default-release"):
		return 0
	case strings.HasSuffix(base, ".default"):
		return 1
	default:
		return 2
	}
}

func firefoxProfileRoots() ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		local := os.Getenv("LOCALAPPDATA")
		var roots []string
		if appdata != "" {
			roots = append(roots, filepath.Join(appdata, "Mozilla", "Firefox", "Profiles"))
		}
		if local != "" {
			roots = append(roots,
				filepath.Join(local, "Packages", "Mozilla.Firefox_n80bbvh6b1yt2", "LocalCache", "Roaming", "Mozilla", "Firefox", "Profiles"),
			)
		}
		return roots, nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return []string{filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")}, nil
	default: // linux / *bsd
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		config := os.Getenv("XDG_CONFIG_HOME")
		if config == "" {
			config = filepath.Join(home, ".config")
		}
		return []string{
			filepath.Join(config, "mozilla", "firefox"),
			filepath.Join(home, ".mozilla", "firefox"),
			filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
			filepath.Join(home, ".var", "app", "org.mozilla.firefox", "config", "mozilla", "firefox"),
		}, nil
	}
}

// DefaultSafariCookiesPath returns the default location of Safari's
// Cookies.binarycookies file on macOS.
func DefaultSafariCookiesPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("safari cookie import is only supported on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, "Library", "Cookies", "Cookies.binarycookies"),
		filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("Safari cookies file not found; check that Safari has been used recently")
}

// NetscapeString is a convenience wrapper for tests / callers that have the
// content as bytes already.
func ParseNetscapeString(s string) ([]types.SessionCookie, error) {
	return ParseNetscape(strings.NewReader(s))
}

// FirefoxProfile describes one discovered Firefox profile.
type FirefoxProfile struct {
	Path  string // directory containing cookies.sqlite
	Label string // short form, e.g. "abc.default-release"
}

// AllFirefoxProfiles enumerates every Firefox profile directory containing a
// cookies.sqlite across all discovered root locations. Results are sorted:
// `.default-release` first, then `.default`, then others lexicographically.
// If no profiles are found the returned slice is empty and the error is nil.
func AllFirefoxProfiles() ([]FirefoxProfile, error) {
	roots, err := firefoxProfileRoots()
	if err != nil {
		return nil, err
	}
	var out []FirefoxProfile
	seen := make(map[string]bool)
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			// Root doesn't exist — not fatal; try next root.
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			full := filepath.Join(root, e.Name())
			if seen[full] {
				continue
			}
			db := filepath.Join(full, "cookies.sqlite")
			if fi, err := os.Stat(db); err != nil || fi.IsDir() {
				continue
			}
			seen[full] = true
			out = append(out, FirefoxProfile{Path: full, Label: e.Name()})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi := firefoxProfilePriority(out[i].Path)
		pj := firefoxProfilePriority(out[j].Path)
		if pi != pj {
			return pi < pj
		}
		return out[i].Label < out[j].Label
	})
	return out, nil
}

// ReconcileSnapshots merges multiple cookie sources into a single jar. When
// the same (name, domain) appears in more than one source the record with the
// latest LastSeen wins. Ties — including the "both timestamps are zero" case
// that Netscape imports produce — are resolved by input order, with the LATER
// entry winning. This matches both intuition (later lines in a cookies.txt
// or later DB rows are typically newer) and FilterGoogleCookies' last-wins
// dedup behavior.
//
// The returned slice is sorted deterministically by (domain, name).
func ReconcileSnapshots(snapshots []CookieSnapshot) []types.SessionCookie {
	type key struct{ name, domain string }
	best := make(map[key]CookieSnapshot, len(snapshots))
	for _, s := range snapshots {
		k := key{s.Cookie.Name, s.Cookie.Domain}
		if e, ok := best[k]; ok && s.LastSeen.Before(e.LastSeen) {
			// New entry is strictly older; keep existing.
			continue
		}
		best[k] = s
	}
	out := make([]types.SessionCookie, 0, len(best))
	for _, s := range best {
		out = append(out, s.Cookie)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// WinningSources returns, for each (name,domain) key, the label of the source
// whose snapshot survived reconciliation. Intended for logging so the user
// sees which profile supplied each cookie. Tie-breaking matches
// ReconcileSnapshots (last-wins).
func WinningSources(snapshots []CookieSnapshot) map[string]string {
	type key struct{ name, domain string }
	best := make(map[key]CookieSnapshot, len(snapshots))
	for _, s := range snapshots {
		k := key{s.Cookie.Name, s.Cookie.Domain}
		if e, ok := best[k]; ok && s.LastSeen.Before(e.LastSeen) {
			continue
		}
		best[k] = s
	}
	out := make(map[string]string, len(best))
	for k, s := range best {
		out[k.name+"@"+k.domain] = s.Source
	}
	return out
}
