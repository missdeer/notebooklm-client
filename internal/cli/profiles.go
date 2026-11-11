package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func profileSessionPath(name string) (string, error) {
	if name == "" || name == "default" {
		return rpc.SessionPath(), nil
	}
	if !safeIdentifier.MatchString(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid profile name %q", name)
	}
	return filepath.Join(rpc.HomeDir(), "profiles", name, "session.json"), nil
}

func selectedProfile(in commandInput, s cliState) string {
	name := in.val("profile", "name")
	if name == "" {
		name, _ = s.Config["default_profile"].(string)
	}
	if name == "" {
		name = "default"
	}
	return name
}

func savedProfiles() ([]string, error) {
	out := []string{}
	if _, err := os.Stat(rpc.SessionPath()); err == nil {
		out = append(out, "default")
	} else if _, err := os.Stat(rpc.SessionPath() + ".storage.json"); err == nil {
		out = append(out, "default")
	}
	entries, err := os.ReadDir(filepath.Join(rpc.HomeDir(), "profiles"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			p := filepath.Join(rpc.HomeDir(), "profiles", e.Name(), "session.json")
			for _, suffix := range []string{"", ".storage.json"} {
				if _, err := os.Stat(p + suffix); err == nil {
					out = append(out, e.Name())
					break
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func loginProfile(in commandInput, state cliState) error {
	name := selectedProfile(in, state)
	path, err := profileSessionPath(name)
	if err != nil {
		return err
	}
	if flagBool(in.cmd, "check") {
		s, err := session.Load(path)
		if err != nil {
			return err
		}
		if s == nil {
			return fmt.Errorf("profile has no session")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c := client.New()
		if err := c.Connect(ctx, client.ConnectOptions{Transport: client.TransportHTTP, Session: s, SessionPath: path, Proxy: resolveProxy(in.cmd)}); err != nil {
			return err
		}
		defer c.Disconnect()
		_, err = c.ListNotebooks(ctx)
		if err != nil {
			return err
		}
		return emitResult(in.cmd, map[string]any{"profile": name, "valid": true})
	}
	mode := in.val("storage")
	if mode != "" {
		if err := session.SetStorageMode(path, mode); err != nil {
			return err
		}
	}
	if flagBool(in.cmd, "manual") {
		file := in.val("file")
		if file == "" {
			return fmt.Errorf("--manual requires --file")
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var sess types.NotebookRpcSession
		if json.Unmarshal(b, &sess) != nil || sess.Cookies == "" {
			sess.Cookies = strings.TrimSpace(string(b))
			sess.CookieJar = session.InferCookieJar(sess.Cookies)
		}
		if sess.Cookies == "" {
			return fmt.Errorf("cookie file is empty")
		}
		if _, err := session.RefreshTokens(in.cmd.Context(), sess, transport.NewUTLSHTTPClient(resolveProxy(in.cmd)), path); err != nil {
			return err
		}
		return emitResult(in.cmd, map[string]any{"profile": name, "saved": true})
	}
	if provider := in.val("provider"); provider != "" && provider != "builtin" {
		if provider != "openclaw" {
			return fmt.Errorf("provider must be builtin or openclaw")
		}
		sess, err := transport.CaptureCDPSession(in.cmd.Context(), in.val("cdp_url"))
		if err != nil {
			return err
		}
		if _, err := session.Save(*sess, path); err != nil {
			return err
		}
		return emitResult(in.cmd, map[string]any{"profile": name, "saved": true})
	}
	c := client.New()
	profileDir := filepath.Join(filepath.Dir(path), "chrome-profile")
	if name == "default" {
		profileDir = rpc.ProfileDir()
	}
	if flagBool(in.cmd, "clear") {
		if _, err := os.Stat(profileDir); err == nil {
			backup := profileDir + ".backup-" + time.Now().UTC().Format("20060102T150405.000000000")
			if err := os.Rename(profileDir, backup); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := c.Connect(context.Background(), client.ConnectOptions{Transport: client.TransportBrowser, ProfileDir: profileDir, ChromePath: in.val("chrome_path"), Proxy: resolveProxy(in.cmd)}); err != nil {
		return err
	}
	defer c.Disconnect()
	saved, err := c.ExportSession(path)
	if err != nil {
		return err
	}
	return emitResult(in.cmd, map[string]string{"profile": name, "session_path": saved})
}

func refreshProfile(in commandInput, state cliState) error {
	path, err := profileSessionPath(selectedProfile(in, state))
	if err != nil {
		return err
	}
	s, err := session.Load(path)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("profile has no session")
	}
	_, err = session.RefreshTokens(in.cmd.Context(), *s, transport.NewUTLSHTTPClient(resolveProxy(in.cmd)), path)
	if err != nil {
		return err
	}
	return emitResult(in.cmd, map[string]any{"profile": selectedProfile(in, state), "refreshed": true})
}
