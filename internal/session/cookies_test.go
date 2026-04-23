package session

import (
	"strings"
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func findCookie(jar []types.SessionCookie, name string) *types.SessionCookie {
	for i := range jar {
		if jar[i].Name == name {
			return &jar[i]
		}
	}
	return nil
}

func TestMergeCookieJarPreservesExpiresWhenServerDoesNotResend(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	oldExp := now.Add(365 * 24 * time.Hour)

	prev := []types.SessionCookie{
		{Name: "__Secure-1PSID", Value: "old-psid", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
		{Name: "__Secure-1PSIDTS", Value: "old-ts", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
	}
	// Server only rotates the PSIDTS cookie; PSID is not re-sent.
	newTSExp := now.Add(400 * 24 * time.Hour)
	headers := []string{
		"__Secure-1PSIDTS=new-ts; Expires=" + newTSExp.Format(time.RFC1123) + "; Path=/; Domain=.google.com; Secure; HttpOnly",
	}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 2 {
		t.Fatalf("want 2 cookies, got %d: %+v", len(got), got)
	}

	psid := findCookie(got, "__Secure-1PSID")
	if psid == nil {
		t.Fatalf("__Secure-1PSID was dropped")
	}
	if psid.Value != "old-psid" {
		t.Errorf("untouched cookie value changed: %q", psid.Value)
	}
	if psid.Expires == nil || !psid.Expires.Equal(oldExp) {
		t.Errorf("untouched cookie Expires lost: %v", psid.Expires)
	}

	ts := findCookie(got, "__Secure-1PSIDTS")
	if ts.Value != "new-ts" {
		t.Errorf("value not updated: %q", ts.Value)
	}
	if ts.Expires == nil || !ts.Expires.Equal(newTSExp.UTC()) {
		t.Errorf("Expires not updated from header: got %v want %v", ts.Expires, newTSExp.UTC())
	}
}

func TestMergeCookieJarMaxAgeOverridesExpires(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	// Expires says "in 10 seconds" but Max-Age says "in 3600 seconds" — Max-Age wins.
	expiresAttr := now.Add(10 * time.Second).Format(time.RFC1123)
	headers := []string{
		"ROTATE=v; Expires=" + expiresAttr + "; Max-Age=3600; Path=/",
	}
	got := MergeCookieJar(nil, headers, now)
	if len(got) != 1 {
		t.Fatalf("want 1 cookie, got %d", len(got))
	}
	wantExp := now.Add(3600 * time.Second)
	if got[0].Expires == nil || !got[0].Expires.Equal(wantExp) {
		t.Errorf("Max-Age should win: got %v want %v", got[0].Expires, wantExp)
	}
}

func TestMergeCookieJarDeletion(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	oldExp := now.Add(24 * time.Hour)
	prev := []types.SessionCookie{
		{Name: "KEEP", Value: "a", Domain: ".google.com", Expires: &oldExp},
		{Name: "GONE", Value: "b", Domain: ".google.com", Expires: &oldExp},
		{Name: "ALSO_GONE", Value: "c", Domain: ".google.com", Expires: &oldExp},
	}
	headers := []string{
		"GONE=; Max-Age=-1; Path=/",                                          // explicit Max-Age negative
		"ALSO_GONE=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Path=/; Secure", // past Expires
	}
	got := MergeCookieJar(prev, headers, now)
	if len(got) != 1 {
		t.Fatalf("want 1 cookie, got %d: %+v", len(got), got)
	}
	if got[0].Name != "KEEP" {
		t.Errorf("wrong survivor: %+v", got[0])
	}
}

func TestMergeCookieJarInsertsNewCookiesInOrder(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{
		{Name: "A", Value: "1", Domain: ".google.com"},
		{Name: "B", Value: "2", Domain: ".google.com"},
	}
	headers := []string{
		"C=3; Max-Age=60",
		"D=4; Max-Age=60",
	}
	got := MergeCookieJar(prev, headers, now)
	if len(got) != 4 {
		t.Fatalf("want 4 cookies, got %d", len(got))
	}
	want := []string{"A", "B", "C", "D"}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("order[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestMergeCookieJarIgnoresMalformedHeaders(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{{Name: "KEEP", Value: "v", Domain: ".google.com"}}
	headers := []string{
		"",
		"  ",
		"not-a-cookie",
	}
	got := MergeCookieJar(prev, headers, now)
	if len(got) != 1 || got[0].Name != "KEEP" {
		t.Fatalf("unexpected merge result: %+v", got)
	}
}

func TestMergeCookieJarPreservesDomainWhenHeaderOmits(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{
		{Name: "SID", Value: "v", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true},
	}
	// Server echoes SID without Domain attribute — we must keep .google.com.
	headers := []string{"SID=v2; Path=/; Max-Age=3600"}
	got := MergeCookieJar(prev, headers, now)
	c := findCookie(got, "SID")
	if c == nil || c.Domain != ".google.com" {
		t.Errorf("Domain not preserved: %+v", c)
	}
	if !c.Secure || !c.HttpOnly {
		t.Errorf("Secure/HttpOnly must not be downgraded: %+v", c)
	}
	if c.Value != "v2" {
		t.Errorf("value not updated: %q", c.Value)
	}
}

// Regression: keying the jar by cookie name alone let .google.com and
// .youtube.com NIDs collapse into a single entry. Per RFC 6265 §5.3 cookies
// are identified by (name, domain, path) and must coexist.
func TestMergeCookieJarKeepsSameNameAcrossDomains(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	oldExp := now.Add(30 * 24 * time.Hour)
	prev := []types.SessionCookie{
		{Name: "NID", Value: "g", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
		{Name: "NID", Value: "y", Domain: ".youtube.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
	}

	// Pass-through: no Set-Cookie headers. Both cookies must survive.
	got := MergeCookieJar(prev, nil, now)
	if len(got) != 2 {
		t.Fatalf("both domain-scoped NIDs must survive, got %d: %+v", len(got), got)
	}
	var g, y *types.SessionCookie
	for i := range got {
		switch got[i].Domain {
		case ".google.com":
			g = &got[i]
		case ".youtube.com":
			y = &got[i]
		}
	}
	if g == nil || y == nil {
		t.Fatalf("missing domain: g=%v y=%v", g, y)
	}
	if g.Value != "g" || y.Value != "y" {
		t.Errorf("values crossed domains: g=%q y=%q", g.Value, y.Value)
	}
}

func TestMergeCookieJarTargetsCorrectDomainOnUpdate(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	oldExp := now.Add(30 * 24 * time.Hour)
	prev := []types.SessionCookie{
		{Name: "NID", Value: "g", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
		{Name: "NID", Value: "y", Domain: ".youtube.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
	}
	newExp := now.Add(60 * 24 * time.Hour)
	headers := []string{
		"NID=g-rotated; Domain=.google.com; Path=/; Expires=" + newExp.Format(time.RFC1123) + "; Secure; HttpOnly",
	}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 2 {
		t.Fatalf("want 2 cookies after update, got %d: %+v", len(got), got)
	}

	for _, c := range got {
		switch c.Domain {
		case ".google.com":
			if c.Value != "g-rotated" {
				t.Errorf(".google.com NID not updated: %q", c.Value)
			}
			if c.Expires == nil || !c.Expires.Equal(newExp.UTC()) {
				t.Errorf(".google.com NID Expires not refreshed: %v", c.Expires)
			}
		case ".youtube.com":
			if c.Value != "y" {
				t.Errorf(".youtube.com NID spuriously changed: %q", c.Value)
			}
			if c.Expires == nil || !c.Expires.Equal(oldExp) {
				t.Errorf(".youtube.com Expires must be untouched: %v", c.Expires)
			}
		default:
			t.Errorf("unexpected domain survived: %+v", c)
		}
	}
}

func TestMergeCookieJarAmbiguousNameWithoutDomainInsertsNew(t *testing.T) {
	// Two same-name cookies on different domains: a Set-Cookie without
	// Domain is ambiguous and must not silently overwrite either entry.
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{
		{Name: "NID", Value: "g", Domain: ".google.com", Path: "/"},
		{Name: "NID", Value: "y", Domain: ".youtube.com", Path: "/"},
	}
	headers := []string{"NID=mystery; Path=/; Max-Age=3600"}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 3 {
		t.Fatalf("ambiguous header should insert a new host-only entry, got %d: %+v", len(got), got)
	}
	// Neither original NID should have been overwritten with "mystery".
	for _, c := range got {
		if (c.Domain == ".google.com" && c.Value != "g") ||
			(c.Domain == ".youtube.com" && c.Value != "y") {
			t.Errorf("domain-scoped entry clobbered by ambiguous header: %+v", c)
		}
	}
}

// Regression: http.ParseSetCookie returns Path="" when the Set-Cookie header
// omits the Path attribute. A strict (name, domain, path) lookup must still
// match the stored Path="/" entry (RFC 6265 §5.1.4 default-path), otherwise
// a rotation like "SID=new; Domain=.google.com; Max-Age=3600" appends a
// duplicate cookie instead of updating in place — which in turn duplicates
// "SID=..." in the Cookie header sent on the next request.
func TestMergeCookieJarUpdatesWhenSetCookieOmitsPath(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	oldExp := now.Add(10 * 24 * time.Hour)
	prev := []types.SessionCookie{
		{Name: "SID", Value: "old", Domain: ".google.com", Path: "/", Secure: true, HttpOnly: true, Expires: &oldExp},
	}
	headers := []string{"SID=new; Domain=.google.com; Max-Age=3600"}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 1 {
		t.Fatalf("want in-place update, got %d entries: %+v", len(got), got)
	}
	if got[0].Value != "new" {
		t.Errorf("value not updated: %+v", got[0])
	}
	// Path and Secure/HttpOnly must be preserved on the updated entry.
	if got[0].Path != "/" || !got[0].Secure || !got[0].HttpOnly {
		t.Errorf("preserved attributes lost: %+v", got[0])
	}

	// The downstream FlattenCookies should emit SID exactly once.
	flat := FlattenCookies(got)
	if strings.Count(flat, "SID=") != 1 {
		t.Errorf("Cookie header has duplicate SID: %q", flat)
	}
}

// Sibling scenario: when Set-Cookie specifies a Path that doesn't match the
// stored entry's Path, we correctly treat it as a new cookie scope — this
// must NOT regress along with the Path=="" fix.
func TestMergeCookieJarDistinctPathsStayIndependent(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{
		{Name: "PREF", Value: "root", Domain: ".google.com", Path: "/"},
	}
	headers := []string{"PREF=deep; Domain=.google.com; Path=/accounts; Max-Age=3600"}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 2 {
		t.Fatalf("different Paths must coexist, got %d: %+v", len(got), got)
	}
}

func TestMergeCookieJarDomainNormalizationCollapsesLeadingDot(t *testing.T) {
	// Existing jar uses ".google.com"; Set-Cookie echoes "google.com"
	// (no leading dot). Per RFC 6265 these are the same scope — we must
	// update in place rather than insert a duplicate.
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	prev := []types.SessionCookie{
		{Name: "SID", Value: "old", Domain: ".google.com", Path: "/"},
	}
	headers := []string{"SID=new; Domain=google.com; Path=/; Max-Age=3600"}

	got := MergeCookieJar(prev, headers, now)
	if len(got) != 1 {
		t.Fatalf(".google.com and google.com should collapse, got %d: %+v", len(got), got)
	}
	if got[0].Value != "new" {
		t.Errorf("value not updated across dot-normalization: %+v", got[0])
	}
}

func TestFlattenCookiesPreservesOrderAndDedupes(t *testing.T) {
	jar := []types.SessionCookie{
		{Name: "A", Value: "1"},
		{Name: "B", Value: "2"},
		{Name: "A", Value: "1"}, // exact duplicate → dropped
		{Name: "", Value: "nope"},
	}
	got := FlattenCookies(jar)
	want := "A=1; B=2"
	if got != want {
		t.Errorf("FlattenCookies = %q, want %q", got, want)
	}
	if strings.Contains(got, ";;") {
		t.Errorf("unexpected empty slot in %q", got)
	}
}
