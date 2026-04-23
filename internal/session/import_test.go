package session

import (
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestFilterGoogleCookiesKeepsOnlyGoogleFamily(t *testing.T) {
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []types.SessionCookie{
		{Name: "A", Domain: ".google.com", Expires: &exp},
		{Name: "B", Domain: "accounts.google.com"},
		{Name: "C", Domain: ".youtube.com"},
		{Name: "D", Domain: ".googleusercontent.com"},
		{Name: "X", Domain: ".example.com"},
		{Name: "Y", Domain: "facebook.com"},
	}
	got := FilterGoogleCookies(in)
	if len(got) != 4 {
		t.Fatalf("want 4 kept, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Domain == ".example.com" || c.Domain == "facebook.com" {
			t.Errorf("non-Google cookie leaked: %+v", c)
		}
	}
}

func TestFilterGoogleCookiesDedupesByNameAndDomainKeepsLast(t *testing.T) {
	t1 := time.Unix(1000000000, 0).UTC()
	t2 := time.Unix(2000000000, 0).UTC()
	in := []types.SessionCookie{
		{Name: "SID", Domain: ".google.com", Value: "old", Expires: &t1},
		{Name: "SID", Domain: ".google.com", Value: "new", Expires: &t2},
	}
	got := FilterGoogleCookies(in)
	if len(got) != 1 {
		t.Fatalf("want 1 after dedup, got %d", len(got))
	}
	if got[0].Value != "new" {
		t.Errorf("expected last-write-wins, got value %q", got[0].Value)
	}
}

func TestIsGoogleDomainCoverage(t *testing.T) {
	ok := []string{
		"google.com",
		".google.com",
		"accounts.google.com",
		"www.google.com",
		"youtube.com",
		".youtube.com",
		"m.youtube.com",
		"googleusercontent.com",
		"ssl.gstatic.googleusercontent.com",
		"GOOGLE.com", // case-insensitive
	}
	for _, d := range ok {
		if !IsGoogleDomain(d) {
			t.Errorf("%q should be Google", d)
		}
	}
	bad := []string{
		"",
		"example.com",
		"notgoogle.com",
		"google.com.evil.com",
		"facebook.com",
	}
	for _, d := range bad {
		if IsGoogleDomain(d) {
			t.Errorf("%q must not be Google", d)
		}
	}
}
