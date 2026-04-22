package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

type StoredSession struct {
	Version    int                      `json:"version"`
	ExportedAt string                   `json:"exportedAt"`
	Session    types.NotebookRpcSession `json:"session"`
}

func Save(session types.NotebookRpcSession, path string) (string, error) {
	if path == "" {
		path = rpc.SessionPath()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stored := StoredSession{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Session:    session,
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func Load(path string) (*types.NotebookRpcSession, error) {
	if path == "" {
		path = rpc.SessionPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var stored StoredSession
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, nil
	}
	if stored.Version != 1 || stored.Session.AT == "" {
		return nil, nil
	}
	if len(stored.Session.CookieJar) == 0 && stored.Session.Cookies != "" {
		stored.Session.CookieJar = InferCookieJar(stored.Session.Cookies)
	}
	return &stored.Session, nil
}

func HasValid(path string, maxAge time.Duration) (bool, error) {
	if path == "" {
		path = rpc.SessionPath()
	}
	if maxAge == 0 {
		maxAge = 2 * time.Hour
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	var stored StoredSession
	if err := json.Unmarshal(data, &stored); err != nil {
		return false, nil
	}
	if stored.ExportedAt == "" || stored.Session.AT == "" {
		return false, nil
	}
	exported, err := time.Parse(time.RFC3339, stored.ExportedAt)
	if err != nil {
		return false, nil
	}
	return time.Since(exported) < maxAge, nil
}

func InferCookieJar(cookies string) []types.SessionCookie {
	if cookies == "" {
		return nil
	}
	var jar []types.SessionCookie
	for _, pair := range strings.Split(cookies, ";") {
		pair = strings.TrimSpace(pair)
		eq := strings.Index(pair, "=")
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(pair[:eq])
		value := strings.TrimSpace(pair[eq+1:])
		if name == "" || value == "" {
			continue
		}
		secure := strings.HasPrefix(name, "__Secure") || strings.HasPrefix(name, "__Host")
		jar = append(jar, types.SessionCookie{
			Name:     name,
			Value:    value,
			Domain:   ".google.com",
			Path:     "/",
			Secure:   secure,
			HttpOnly: true,
		})
	}
	return jar
}
