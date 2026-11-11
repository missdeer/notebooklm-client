package rpc

import "net/url"

// NotebookHost accepts consumer hosts only. Enterprise uses a different protocol.
func NotebookHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ""
	}
	switch u.Hostname() {
	case "notebooklm.google.com", "notebook.google.com":
		return u.Hostname()
	}
	return ""
}

// SessionBaseURL keeps old sessions compatible while honoring the login host.
func SessionBaseURL(host string) string {
	if NotebookHost("https://"+host) == host && host != "" {
		return "https://" + host
	}
	return BaseURL
}

// SessionURL changes only known Notebook endpoints, never external download URLs.
func SessionURL(rawURL, host string) string {
	u, err := url.Parse(rawURL)
	if err != nil || NotebookHost(rawURL) == "" {
		return rawURL
	}
	u.Host = SessionBaseURL(host)[len("https://"):]
	return u.String()
}
