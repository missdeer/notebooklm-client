package rpc

import "testing"

func TestNotebookHost(t *testing.T) {
	for _, raw := range []string{
		"https://accounts.google.com/?continue=https://notebook.google.com/",
		"https://notebook.google.com.evil.test/", "https://evil-notebooklm.google.com/",
		"http://notebook.google.com/", "https://user@notebook.google.com/",
		"https://notebook.google.com:8443/",
	} {
		if got := NotebookHost(raw); got != "" {
			t.Errorf("NotebookHost(%q) = %q", raw, got)
		}
	}
	for _, host := range []string{"notebooklm.google.com", "notebook.google.com"} {
		if got := NotebookHost("https://" + host + "/?original_referer=https://accounts.google.com"); got != host {
			t.Errorf("got %q, want %q", got, host)
		}
	}
}

func TestSessionURL(t *testing.T) {
	if got := SessionURL(BatchExecuteURL, "notebook.google.com"); got != "https://notebook.google.com/_/LabsTailwindUi/data/batchexecute" {
		t.Fatal(got)
	}
	if got := SessionURL(ChatStreamURL, ""); got != ChatStreamURL {
		t.Fatal(got)
	}
	if got := SessionURL(UploadURL, "evil.test"); got != UploadURL {
		t.Fatal(got)
	}
	cdn := "https://lh3.googleusercontent.com/download?id=1"
	if got := SessionURL(cdn, "notebook.google.com"); got != cdn {
		t.Fatal(got)
	}
}
