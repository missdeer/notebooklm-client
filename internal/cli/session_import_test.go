package cli

import (
	"strings"
	"testing"
)

func TestParseImportedSessionWrapped(t *testing.T) {
	// StoredSession envelope, as written by export-session.
	data := []byte(`{
		"version": 1,
		"exportedAt": "2026-04-23T00:00:00Z",
		"session": {
			"at": "AT_wrapped",
			"bl": "BL_wrapped",
			"fsid": "FSID_wrapped",
			"cookies": "x=1",
			"userAgent": "Mozilla/5.0"
		}
	}`)
	sess, err := parseImportedSession(data)
	if err != nil {
		t.Fatalf("wrapped import failed: %v", err)
	}
	if sess.AT != "AT_wrapped" {
		t.Errorf("wrapped AT not unwrapped: got %q", sess.AT)
	}
	if sess.UserAgent != "Mozilla/5.0" {
		t.Errorf("other fields not unwrapped: %+v", sess)
	}
}

func TestParseImportedSessionRaw(t *testing.T) {
	// Bare NotebookRpcSession (TypeScript client's session.json format).
	data := []byte(`{"at":"AT_raw","bl":"BL_raw","fsid":"FSID_raw","cookies":"x=1","userAgent":"Mozilla/5.0"}`)
	sess, err := parseImportedSession(data)
	if err != nil {
		t.Fatalf("raw import failed: %v", err)
	}
	if sess.AT != "AT_raw" {
		t.Errorf("raw AT lost: %+v", sess)
	}
}

func TestParseImportedSessionMissingATOnBothShapes(t *testing.T) {
	// Wrapped but empty session → should surface as "missing 'at' token"
	// rather than falling back silently.
	data := []byte(`{"version":1,"session":{"bl":"nope"}}`)
	_, err := parseImportedSession(data)
	if err == nil {
		t.Fatal("expected error on missing AT in wrapped session")
	}
	if !strings.Contains(err.Error(), "missing 'at' token") {
		t.Errorf("error message should mention missing AT: %v", err)
	}
}

func TestParseImportedSessionMalformedJSON(t *testing.T) {
	_, err := parseImportedSession([]byte("not json"))
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
	if !strings.Contains(err.Error(), "invalid session JSON") {
		t.Errorf("error should wrap JSON error: %v", err)
	}
}

func TestParseImportedSessionWrappedWithExtraTopLevelKeysStillWorks(t *testing.T) {
	// Guard against future additions to the StoredSession envelope: unknown
	// top-level keys must not cause fallback to the raw-session path.
	data := []byte(`{
		"version": 2,
		"exportedAt": "2026-04-23T00:00:00Z",
		"futureField": {"anything": true},
		"session": {"at":"AT_future"}
	}`)
	sess, err := parseImportedSession(data)
	if err != nil {
		t.Fatalf("wrapped with extra keys: %v", err)
	}
	if sess.AT != "AT_future" {
		t.Errorf("unwrapped AT wrong: %+v", sess)
	}
}

// TestParseImportedSessionRawWithSessionKeyAsOwnField guards the edge case
// where a TypeScript-style raw session JSON happens to carry a stray
// "session" key (it shouldn't in practice, but we must not mis-treat it as
// a wrapper). We only accept a wrapper when stored.Session.AT is populated.
func TestParseImportedSessionRawIsNotConfusedByStraySessionField(t *testing.T) {
	data := []byte(`{"at":"AT_raw","bl":"BL","fsid":"F","session":null}`)
	sess, err := parseImportedSession(data)
	if err != nil {
		t.Fatalf("raw with stray session field: %v", err)
	}
	if sess.AT != "AT_raw" {
		t.Errorf("AT lost: %+v", sess)
	}
}
