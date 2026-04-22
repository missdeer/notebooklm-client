package rpc

import (
	"os"
	"path/filepath"
)

var homeOverride string

func HomeDir() string {
	if homeOverride != "" {
		return homeOverride
	}
	if env := os.Getenv("NOTEBOOKLM_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notebooklm")
}

func SetHomeDir(dir string) {
	homeOverride = dir
}

func SessionPath() string {
	return filepath.Join(HomeDir(), "session.json")
}

func ProfileDir() string {
	return filepath.Join(HomeDir(), "chrome-profile")
}

func RpcIDsPath() string {
	return filepath.Join(HomeDir(), "rpc-ids.json")
}
