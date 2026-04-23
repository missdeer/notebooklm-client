package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSessionCookieJSONRoundTripPreservesExpires(t *testing.T) {
	exp := time.Date(2027, 4, 1, 12, 0, 0, 0, time.UTC)
	orig := SessionCookie{
		Name:     "__Secure-1PSIDTS",
		Value:    "abc",
		Domain:   ".google.com",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		Expires:  &exp,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"expires":"2027-04-01T12:00:00Z"`) {
		t.Fatalf("expected RFC3339 expires in JSON, got: %s", data)
	}

	var got SessionCookie
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Expires == nil || !got.Expires.Equal(exp) {
		t.Fatalf("expires not round-tripped: got %v want %v", got.Expires, exp)
	}
	if got.Session {
		t.Fatalf("session should default to false")
	}
}

func TestSessionCookieOmitsExpiresWhenNil(t *testing.T) {
	// Legacy cookies without expiration must serialize without the field.
	c := SessionCookie{Name: "x", Value: "y", Domain: ".google.com"}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "expires") {
		t.Fatalf("expected no expires key when nil, got: %s", data)
	}
	if strings.Contains(string(data), "session") {
		t.Fatalf("expected no session key when false, got: %s", data)
	}
}

func TestSessionCookieLegacyJSONStillParses(t *testing.T) {
	// A session.json written before the Expires field existed must still load.
	legacy := `{"name":"SID","value":"v","domain":".google.com","path":"/","secure":true,"httpOnly":true}`
	var c SessionCookie
	if err := json.Unmarshal([]byte(legacy), &c); err != nil {
		t.Fatalf("legacy unmarshal: %v", err)
	}
	if c.Expires != nil {
		t.Fatalf("legacy cookie should have nil Expires, got %v", c.Expires)
	}
	if c.Session {
		t.Fatalf("legacy cookie should have Session=false")
	}
	if c.Name != "SID" || c.Value != "v" {
		t.Fatalf("legacy fields not preserved: %+v", c)
	}
}
