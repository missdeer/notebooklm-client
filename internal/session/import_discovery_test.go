package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestReconcileSnapshotsNewestWins(t *testing.T) {
	older := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	snaps := []CookieSnapshot{
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "stale"}, LastSeen: older, Source: "firefox(A)"},
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "fresh"}, LastSeen: newer, Source: "firefox(B)"},
		{Cookie: types.SessionCookie{Name: "OTHER", Domain: ".google.com", Value: "x"}, LastSeen: older, Source: "firefox(A)"},
	}

	jar := ReconcileSnapshots(snaps)
	if len(jar) != 2 {
		t.Fatalf("want 2 cookies, got %d: %+v", len(jar), jar)
	}
	var sid types.SessionCookie
	for _, c := range jar {
		if c.Name == "SID" {
			sid = c
		}
	}
	if sid.Value != "fresh" {
		t.Errorf("newer timestamp should win: got value %q", sid.Value)
	}

	winners := WinningSources(snaps)
	if winners["SID@.google.com"] != "firefox(B)" {
		t.Errorf("winner metadata wrong: %v", winners)
	}
	if winners["OTHER@.google.com"] != "firefox(A)" {
		t.Errorf("OTHER should be attributed to A: %v", winners)
	}
}

func TestReconcileSnapshotsZeroTimestampLosesToNonZero(t *testing.T) {
	snaps := []CookieSnapshot{
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "no-ts"}, Source: "netscape"},
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "ts"}, LastSeen: time.Unix(100, 0), Source: "firefox(A)"},
	}
	jar := ReconcileSnapshots(snaps)
	if len(jar) != 1 || jar[0].Value != "ts" {
		t.Fatalf("non-zero timestamp should win over zero: %+v", jar)
	}
}

// Regression: Netscape imports produce all-zero LastSeen. The earlier
// behavior (first-wins on ties) let stale duplicates survive even when a
// fresher value followed later in the file. The fix is last-wins on ties.
func TestReconcileSnapshotsAllZeroTimestampsKeepLastEntry(t *testing.T) {
	snaps := []CookieSnapshot{
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "stale"}, Source: "netscape"},
		{Cookie: types.SessionCookie{Name: "OTHER", Domain: ".google.com", Value: "keep"}, Source: "netscape"},
		{Cookie: types.SessionCookie{Name: "SID", Domain: ".google.com", Value: "fresh"}, Source: "netscape"},
	}
	jar := ReconcileSnapshots(snaps)
	if len(jar) != 2 {
		t.Fatalf("want 2 after dedup, got %d: %+v", len(jar), jar)
	}
	for _, c := range jar {
		if c.Name == "SID" && c.Value != "fresh" {
			t.Errorf("later duplicate should win on tie: got %q", c.Value)
		}
	}

	// WinningSources must also reflect last-wins so the log doesn't claim
	// the stale entry supplied the final cookie.
	winners := WinningSources(snaps)
	if winners["SID@.google.com"] == "" {
		t.Errorf("winner not reported for SID")
	}
}

func TestReconcileSnapshotsEqualNonZeroTimestampsKeepLastEntry(t *testing.T) {
	// Two Firefox profiles with the same lastAccessed for a cookie — still
	// last-wins in input order. (Unlikely in practice, but the invariant
	// should hold uniformly.)
	ts := time.Unix(1_700_000_000, 0)
	snaps := []CookieSnapshot{
		{Cookie: types.SessionCookie{Name: "PSID", Domain: ".google.com", Value: "profileA"}, LastSeen: ts, Source: "firefox(A)"},
		{Cookie: types.SessionCookie{Name: "PSID", Domain: ".google.com", Value: "profileB"}, LastSeen: ts, Source: "firefox(B)"},
	}
	jar := ReconcileSnapshots(snaps)
	if len(jar) != 1 || jar[0].Value != "profileB" {
		t.Fatalf("equal timestamps should let later entry win: %+v", jar)
	}
	if got := WinningSources(snaps)["PSID@.google.com"]; got != "firefox(B)" {
		t.Errorf("winner label should match the surviving entry: got %q", got)
	}
}

func TestAllFirefoxProfilesFindsEveryDirWithCookiesDB(t *testing.T) {
	root := t.TempDir()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Populate every platform's default Firefox profile root under the temp
	// dir. firefoxProfileRoots() reads HOME/USERPROFILE/APPDATA/LOCALAPPDATA/
	// XDG_CONFIG_HOME, so overriding them all lands every OS in the same fake
	// tree. We create both the macOS/Linux layout and the Windows APPDATA
	// layout so the test exercises whichever one the current OS picks.
	profileRootsForOS := []string{
		filepath.Join(root, ".mozilla", "firefox"),                         // linux (HOME)
		filepath.Join(root, "Library", "Application Support", "Firefox", "Profiles"),  // darwin
		filepath.Join(root, "Mozilla", "Firefox", "Profiles"),              // windows (APPDATA)
	}
	mkProfile := func(name string, withDB bool) {
		for _, base := range profileRootsForOS {
			p := filepath.Join(base, name)
			must(os.MkdirAll(p, 0o755))
			if withDB {
				must(os.WriteFile(filepath.Join(p, "cookies.sqlite"), []byte("x"), 0o644))
			}
		}
	}
	mkProfile("zzz.default-release", true)
	mkProfile("abc.default", true)
	mkProfile("dev-edition", true)
	mkProfile("broken.default", false) // no cookies.sqlite — must be skipped

	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("APPDATA", root)
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "no-such-local"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "no-such-xdg"))

	profiles, err := AllFirefoxProfiles()
	if err != nil {
		t.Fatalf("AllFirefoxProfiles: %v", err)
	}
	if len(profiles) == 0 {
		t.Fatalf("expected profiles, got none")
	}

	if !hasSuffix(profiles[0].Label, ".default-release") {
		t.Errorf("first profile should be default-release, got %q", profiles[0].Label)
	}
	for _, p := range profiles {
		if p.Label == "broken.default" {
			t.Errorf("profile without cookies.sqlite leaked into results")
		}
	}
	// Validate that the three real profiles each appear at least once
	// (duplicate detection via the seen[] map inside AllFirefoxProfiles
	// prevents counting the same dir twice across roots).
	names := map[string]bool{}
	for _, p := range profiles {
		names[p.Label] = true
	}
	for _, want := range []string{"zzz.default-release", "abc.default", "dev-edition"} {
		if !names[want] {
			t.Errorf("profile %q missing from discovery", want)
		}
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
