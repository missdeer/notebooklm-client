package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestSaveLoadRoundTripPreservesCookieExpires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	exp := time.Date(2028, 4, 1, 0, 0, 0, 0, time.UTC)
	orig := types.NotebookRpcSession{
		AT:      "AT",
		BL:      "BL",
		FSID:    "FSID",
		Cookies: "__Secure-1PSID=v",
		CookieJar: []types.SessionCookie{
			{
				Name:    "__Secure-1PSID",
				Value:   "v",
				Domain:  ".google.com",
				Path:    "/",
				Secure:  true,
				Expires: &exp,
			},
			{
				Name:    "SESSION_TMP",
				Value:   "s",
				Domain:  ".google.com",
				Session: true,
			},
		},
		UserAgent: "ua",
	}

	if _, err := Save(orig, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load returned nil")
	}
	if len(got.CookieJar) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(got.CookieJar))
	}

	first := got.CookieJar[0]
	if first.Expires == nil || !first.Expires.Equal(exp) {
		t.Fatalf("first cookie expires not preserved: got %v want %v", first.Expires, exp)
	}
	if first.Session {
		t.Fatalf("first cookie should not be session-scoped")
	}

	second := got.CookieJar[1]
	if second.Expires != nil {
		t.Fatalf("session cookie must have nil Expires, got %v", second.Expires)
	}
	if !second.Session {
		t.Fatalf("session cookie Session flag lost")
	}
}

func TestLoadLegacySessionSynthesizesJarWithoutExpires(t *testing.T) {
	// Simulate an old session.json that lacks cookieJar — Load should still
	// infer a jar from the flat Cookies string without producing expiration data.
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	legacy := types.NotebookRpcSession{
		AT:      "AT",
		BL:      "BL",
		FSID:    "FSID",
		Cookies: "SID=abc; HSID=def",
	}
	if _, err := Save(legacy, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil || len(got.CookieJar) != 2 {
		t.Fatalf("expected 2 inferred cookies, got %+v", got)
	}
	for _, c := range got.CookieJar {
		if c.Expires != nil {
			t.Errorf("inferred cookie %s should not have Expires, got %v", c.Name, c.Expires)
		}
	}
}
