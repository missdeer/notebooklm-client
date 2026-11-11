package session

import (
	"strings"
	"testing"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func TestCookieHeaderRespectsBrowserScopes(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	sess := types.NotebookRpcSession{Cookies: "legacy=all", CookieJar: []types.SessionCookie{
		{Name: "SID", Value: "shared", Domain: ".google.com", Path: "/", Secure: true},
		{Name: "GAPS", Value: "login-only", Domain: "accounts.google.com", Path: "/", Secure: true},
		{Name: "local", Value: "new-host", Domain: "notebook.google.com", Path: "/"},
		{Name: "local", Value: "old-host", Domain: "notebooklm.google.com", Path: "/"},
		{Name: "rpc", Value: "path-only", Domain: "notebook.google.com", Path: "/_/"},
		{Name: "expired", Value: "stale", Domain: ".google.com", Path: "/", Expires: &past},
	}}
	got := CookieHeaderForURL(sess, "https://notebook.google.com/_/LabsTailwindUi/data/batchexecute")
	for _, want := range []string{"SID=shared", "local=new-host", "rpc=path-only"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, forbidden := range []string{"GAPS=", "old-host", "expired=", "legacy="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("sent cookie outside its scope: %s", forbidden)
		}
	}
	if got := CookieHeaderForURL(sess, "https://notebook.google.com/"); strings.Contains(got, "rpc=") {
		t.Fatal("path cookie sent to dashboard")
	}
	if got := CookieHeaderForURL(sess, "http://notebook.google.com/"); strings.Contains(got, "SID=") {
		t.Fatal("secure cookie sent over HTTP")
	}
	if got := CookieHeaderForURL(sess, "https://child.notebook.google.com/"); strings.Contains(got, "local=") {
		t.Fatal("host-only cookie sent to child host")
	}
	if got := CookieHeaderForURL(types.NotebookRpcSession{Cookies: "SID=legacy"}, "https://notebook.google.com/"); got != "SID=legacy" {
		t.Fatal("legacy cookies lost")
	}
}
