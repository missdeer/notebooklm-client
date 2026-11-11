package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/zalando/go-keyring"
)

func TestProtectedSessionMigrationRefreshAndRelocation(t *testing.T) {
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "account", "session.json")
	sess := types.NotebookRpcSession{AT: "secret-at", Cookies: "SID=secret-cookie"}
	if _, err := Save(sess, path); err != nil {
		t.Fatal(err)
	}
	if err := SetStorageMode(path, "protected"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plaintext remains after migration")
	}
	encrypted, err := os.ReadFile(path + ".enc")
	if err != nil || strings.Contains(string(encrypted), "secret") {
		t.Fatal("ciphertext leaked session")
	}
	sess.AT = "refreshed"
	if _, err := Save(sess, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.AT != "refreshed" {
		t.Fatalf("refresh %v %v", loaded, err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(filepath.Dir(path), moved); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(moved, "session.json")
	loaded, err = Load(path)
	if err != nil || loaded.AT != "refreshed" {
		t.Fatalf("relocation %v %v", loaded, err)
	}
	if err := SetStorageMode(path, "file"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".enc"); !os.IsNotExist(err) {
		t.Fatal("ciphertext remains after migration to file")
	}
	if loaded, err := Load(path); err != nil || loaded.AT != "refreshed" {
		t.Fatalf("restore %v %v", loaded, err)
	}
}
