package session

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// ReadFirefoxCookies reads cookies from a Firefox profile's cookies.sqlite.
//
// Kept as a convenience for callers that don't care about last-access
// timestamps; internally delegates to ReadFirefoxSnapshots.
func ReadFirefoxCookies(profilePath string) ([]types.SessionCookie, error) {
	snaps, err := ReadFirefoxSnapshots(profilePath)
	if err != nil {
		return nil, err
	}
	out := make([]types.SessionCookie, len(snaps))
	for i, s := range snaps {
		out[i] = s.Cookie
	}
	return out, nil
}

// ReadFirefoxSnapshots reads every cookie from a Firefox profile and returns
// it along with the cookie's lastAccessed timestamp (used to pick the freshest
// value across multiple profiles).
//
// The DB file is copied to a temp path first so that a running Firefox's
// exclusive lock doesn't block us (mirrors yt-dlp behavior).
//
// Firefox 142+ (schema version 16) stores expiry in milliseconds rather than
// seconds; lastAccessed is always microseconds regardless of schema.
func ReadFirefoxSnapshots(profilePath string) ([]CookieSnapshot, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("firefox profile path not specified")
	}
	dbPath := filepath.Join(profilePath, "cookies.sqlite")
	info, err := os.Stat(dbPath)
	if err != nil {
		return nil, fmt.Errorf("firefox cookies.sqlite: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", dbPath)
	}

	tmpDir, err := os.MkdirTemp("", "notebooklm-ff-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, "cookies.sqlite")
	if err := copyFile(dbPath, tmpDB); err != nil {
		return nil, fmt.Errorf("copy firefox cookies db: %w", err)
	}

	db, err := sql.Open("sqlite", tmpDB+"?mode=ro&_pragma=journal_mode(off)")
	if err != nil {
		return nil, fmt.Errorf("open firefox cookies db: %w", err)
	}
	defer db.Close()

	var schemaVersion int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&schemaVersion); err != nil {
		return nil, fmt.Errorf("read schema version: %w", err)
	}
	expiryInMillis := schemaVersion >= 16

	// lastAccessed has been present in moz_cookies since Firefox 4 (2011) so
	// we can rely on it without a column-existence check.
	rows, err := db.Query("SELECT host, name, value, path, expiry, isSecure, isHttpOnly, lastAccessed FROM moz_cookies")
	if err != nil {
		return nil, fmt.Errorf("query moz_cookies: %w", err)
	}
	defer rows.Close()

	label := "firefox(" + filepath.Base(profilePath) + ")"
	var out []CookieSnapshot
	for rows.Next() {
		var (
			host, name, value, path     string
			expiry, lastAccessedMicros  int64
			isSecure, isHTTPOnly        int
		)
		if err := rows.Scan(&host, &name, &value, &path, &expiry, &isSecure, &isHTTPOnly, &lastAccessedMicros); err != nil {
			return nil, fmt.Errorf("scan moz_cookies row: %w", err)
		}
		if name == "" || host == "" {
			continue
		}
		c := types.SessionCookie{
			Name:     name,
			Value:    value,
			Domain:   host,
			Path:     path,
			Secure:   isSecure != 0,
			HttpOnly: isHTTPOnly != 0,
		}
		if expiry > 0 {
			unixSec := expiry
			if expiryInMillis {
				unixSec = expiry / 1000
			}
			t := time.Unix(unixSec, 0).UTC()
			c.Expires = &t
		} else {
			c.Session = true
		}
		var lastSeen time.Time
		if lastAccessedMicros > 0 {
			lastSeen = time.Unix(lastAccessedMicros/1_000_000, (lastAccessedMicros%1_000_000)*1_000).UTC()
		}
		out = append(out, CookieSnapshot{Cookie: c, LastSeen: lastSeen, Source: label})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
