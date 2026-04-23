package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestHumanDurationBoundaries(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{3 * 24 * time.Hour, "3d0h"},
		{60 * 24 * time.Hour, "~2mo0d"},
	}
	for _, c := range cases {
		got := humanDuration(c.in)
		if got != c.want {
			t.Errorf("humanDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCookieRow(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)

	exp := now.Add(10 * 24 * time.Hour)
	persistent := types.SessionCookie{
		Name:    "__Secure-1PSID",
		Domain:  ".google.com",
		Expires: &exp,
	}
	row := formatCookieRow(persistent, now)
	if !strings.Contains(row, "__Secure-1PSID") || !strings.Contains(row, "10d0h") {
		t.Errorf("persistent row missing expected fields: %q", row)
	}

	past := now.Add(-1 * time.Hour)
	expired := types.SessionCookie{Name: "OLD", Domain: ".google.com", Expires: &past}
	if !strings.Contains(formatCookieRow(expired, now), "EXPIRED") {
		t.Errorf("expired cookie not marked EXPIRED")
	}

	sessionCookie := types.SessionCookie{Name: "TMP", Domain: ".google.com", Session: true}
	if !strings.Contains(formatCookieRow(sessionCookie, now), "(session)") {
		t.Errorf("session cookie not marked (session)")
	}

	legacy := types.SessionCookie{Name: "LEGACY", Domain: ".google.com"}
	if !strings.Contains(formatCookieRow(legacy, now), "(unknown)") {
		t.Errorf("legacy cookie without Expires not marked unknown")
	}
}

func TestPrintSessionStatusHighlightsCriticalAndMissing(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	psidExp := now.Add(365 * 24 * time.Hour)
	sidccExp := now.Add(30 * 24 * time.Hour)

	sess := types.NotebookRpcSession{
		AT:   "AT_abcdefghijklmnopqrstuvwxyz", // longer than 24 to exercise truncate
		BL:   "BL",
		FSID: "FSID",
		CookieJar: []types.SessionCookie{
			{Name: "__Secure-1PSID", Domain: ".google.com", Expires: &psidExp},
			{Name: "SIDCC", Domain: ".google.com", Expires: &sidccExp},
			{Name: "UNRELATED", Domain: ".google.com", Expires: &psidExp},
		},
	}

	var buf bytes.Buffer
	printSessionStatus(&buf, sess, false, now)
	out := buf.String()

	// truncate uses len=24, so the ellipsis appears after 24 characters.
	if !strings.Contains(out, "AT_abcdefghijklmnopqrstu...") {
		t.Errorf("token AT should be truncated in output, got:\n%s", out)
	}
	if !strings.Contains(out, "__Secure-1PSID") || !strings.Contains(out, "SIDCC") {
		t.Errorf("critical cookies missing from default output:\n%s", out)
	}
	// The critical list includes HSID, SSID, etc. that are absent — they should
	// be reported as "(not present)" with the follow-up hint.
	if !strings.Contains(out, "(not present)") {
		t.Errorf("expected missing critical cookies to be flagged:\n%s", out)
	}
	if !strings.Contains(out, "--all to list every cookie") {
		t.Errorf("expected hint about --all flag:\n%s", out)
	}
	// Non-critical cookie must NOT appear unless --all is passed.
	if strings.Contains(out, "UNRELATED") {
		t.Errorf("non-critical cookie should be hidden without --all:\n%s", out)
	}

	var allBuf bytes.Buffer
	printSessionStatus(&allBuf, sess, true, now)
	if !strings.Contains(allBuf.String(), "UNRELATED") {
		t.Errorf("--all should include non-critical cookies:\n%s", allBuf.String())
	}
}

func TestPrintSessionStatusLegacyJar(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	// Session loaded from a pre-Expires file: cookies present but no expires.
	sess := types.NotebookRpcSession{
		AT: "AT", BL: "BL", FSID: "FSID",
		CookieJar: []types.SessionCookie{
			{Name: "__Secure-1PSID", Domain: ".google.com"},
		},
	}
	var buf bytes.Buffer
	printSessionStatus(&buf, sess, false, now)
	out := buf.String()
	if !strings.Contains(out, "(unknown)") || !strings.Contains(out, "legacy session") {
		t.Errorf("legacy cookies should render as (unknown)/legacy session:\n%s", out)
	}
}

func TestPrintSessionStatusEmptyJarHint(t *testing.T) {
	sess := types.NotebookRpcSession{AT: "AT", BL: "BL", FSID: "FSID"}
	var buf bytes.Buffer
	printSessionStatus(&buf, sess, false, time.Now())
	if !strings.Contains(buf.String(), "legacy session") {
		t.Errorf("expected legacy-session hint when CookieJar empty:\n%s", buf.String())
	}
}
