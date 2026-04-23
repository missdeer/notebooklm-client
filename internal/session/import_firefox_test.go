package session

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// makeFirefoxDB writes a minimal moz_cookies table at <dir>/cookies.sqlite.
func makeFirefoxDB(t *testing.T, dir string, schemaVersion int, rows [][]any) string {
	t.Helper()
	dbPath := filepath.Join(dir, "cookies.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// moz_cookies schema trimmed to the columns we read.
	_, err = db.Exec(`
		CREATE TABLE moz_cookies (
			host TEXT, name TEXT, value TEXT, path TEXT,
			expiry INTEGER, isSecure INTEGER, isHttpOnly INTEGER,
			lastAccessed INTEGER
		)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = " + itoa(int64(schemaVersion))); err != nil {
		t.Fatalf("set schema version: %v", err)
	}
	for _, row := range rows {
		// Pad older test rows (7 values) with lastAccessed = 0 so the INSERT
		// is always happy.
		if len(row) == 7 {
			row = append(row, int64(0))
		}
		_, err := db.Exec(
			"INSERT INTO moz_cookies (host, name, value, path, expiry, isSecure, isHttpOnly, lastAccessed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			row...,
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return dbPath
}

func TestReadFirefoxCookiesSecondsSchema(t *testing.T) {
	dir := t.TempDir()
	expUnix := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	makeFirefoxDB(t, dir, 15, [][]any{
		{".google.com", "__Secure-1PSID", "abc", "/", expUnix, 1, 1},
		{".google.com", "TMP", "x", "/", 0, 0, 0}, // session cookie
		{".evil.com", "LEAK", "y", "/", expUnix, 0, 0},
	})

	cookies, err := ReadFirefoxCookies(dir)
	if err != nil {
		t.Fatalf("ReadFirefoxCookies: %v", err)
	}
	if len(cookies) != 3 {
		t.Fatalf("want 3 rows (all domains), got %d", len(cookies))
	}

	// Filter to Google family to mimic the full pipeline.
	filtered := FilterGoogleCookies(cookies)
	if len(filtered) != 2 {
		t.Fatalf("want 2 Google cookies, got %d: %+v", len(filtered), filtered)
	}

	var psid, tmp *struct{}
	for i := range filtered {
		c := filtered[i]
		switch c.Name {
		case "__Secure-1PSID":
			psid = &struct{}{}
			if c.Expires == nil || c.Expires.Unix() != expUnix {
				t.Errorf("PSID expires wrong: %v (want %d)", c.Expires, expUnix)
			}
			if !c.Secure || !c.HttpOnly {
				t.Errorf("PSID flags lost: %+v", c)
			}
		case "TMP":
			tmp = &struct{}{}
			if !c.Session || c.Expires != nil {
				t.Errorf("TMP should be session cookie: %+v", c)
			}
		}
	}
	if psid == nil || tmp == nil {
		t.Fatalf("expected PSID and TMP in filtered output")
	}
}

func TestReadFirefoxCookiesMillisecondsSchema(t *testing.T) {
	dir := t.TempDir()
	expUnix := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	// Schema v16+ stores expiry in milliseconds.
	makeFirefoxDB(t, dir, 16, [][]any{
		{".google.com", "__Secure-1PSID", "abc", "/", expUnix * 1000, 1, 1},
	})
	cookies, err := ReadFirefoxCookies(dir)
	if err != nil {
		t.Fatalf("ReadFirefoxCookies: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("want 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Expires == nil || cookies[0].Expires.Unix() != expUnix {
		t.Errorf("millisecond schema not scaled: %v (want %d)", cookies[0].Expires, expUnix)
	}
}

func TestReadFirefoxCookiesMissingProfile(t *testing.T) {
	_, err := ReadFirefoxCookies(filepath.Join(t.TempDir(), "no-such-profile"))
	if err == nil {
		t.Fatal("expected error on missing cookies.sqlite")
	}
}

func TestReadFirefoxSnapshotsIncludesLastAccessed(t *testing.T) {
	dir := t.TempDir()
	expUnix := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	// Microseconds since epoch (Firefox convention).
	lastAccessed := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	lastAccessedMicros := lastAccessed.Unix()*1_000_000 + 0

	makeFirefoxDB(t, dir, 15, [][]any{
		{".google.com", "SID", "abc", "/", expUnix, 1, 1, lastAccessedMicros},
	})

	snaps, err := ReadFirefoxSnapshots(dir)
	if err != nil {
		t.Fatalf("ReadFirefoxSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("want 1, got %d", len(snaps))
	}
	if !snaps[0].LastSeen.Equal(lastAccessed) {
		t.Errorf("LastSeen mismatch: got %v want %v", snaps[0].LastSeen, lastAccessed)
	}
	if snaps[0].Source == "" || !hasPrefix(snaps[0].Source, "firefox(") {
		t.Errorf("Source label wrong: %q", snaps[0].Source)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
