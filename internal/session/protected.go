package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/zalando/go-keyring"
)

const keyringService = "notebooklm-client-go"

type protectionMetadata struct {
	Mode  string `json:"mode"`
	KeyID string `json:"key_id,omitempty"`
}

func StorageMode(path string) (string, error) { m, err := readProtection(path); return m.Mode, err }
func readProtection(path string) (protectionMetadata, error) {
	m := protectionMetadata{Mode: "file"}
	b, err := os.ReadFile(path + ".storage.json")
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("invalid credential storage metadata: %w", err)
	}
	if m.Mode != "file" && m.Mode != "protected" {
		return m, fmt.Errorf("unknown credential storage mode %q", m.Mode)
	}
	return m, nil
}
func credentialBytes(path string) ([]byte, error) {
	m, err := readProtection(path)
	if err != nil {
		return nil, err
	}
	if m.Mode == "file" {
		return os.ReadFile(path)
	}
	data, err := os.ReadFile(path + ".enc")
	if err != nil {
		return nil, err
	}
	gcm, err := credentialCipher(m.KeyID)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted credentials are truncated")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(m.KeyID))
}
func credentialCipher(id string) (cipher.AEAD, error) {
	secret, err := keyring.Get(keyringService, id)
	if err != nil {
		return nil, fmt.Errorf("OS keyring unavailable: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("invalid credential key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func saveCredentialBytes(path string, data []byte) error {
	m, err := readProtection(path)
	if err != nil {
		return err
	}
	if m.Mode == "file" {
		return atomicCredentialWrite(path, data)
	}
	gcm, err := credentialCipher(m.KeyID)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	encrypted := gcm.Seal(nonce, nonce, data, []byte(m.KeyID))
	return atomicCredentialWrite(path+".enc", encrypted)
}
func atomicCredentialWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

// SetStorageMode migrates a session only after the destination is written and verified.
func SetStorageMode(path, mode string) error {
	if mode != "file" && mode != "protected" {
		return fmt.Errorf("mode must be file or protected")
	}
	old, err := readProtection(path)
	if err != nil {
		return err
	}
	if old.Mode == mode {
		return nil
	}
	data, err := credentialBytes(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	exists := err == nil
	next := protectionMetadata{Mode: mode}
	if mode == "protected" {
		next.KeyID = uuid.NewString()
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		if err := keyring.Set(keyringService, next.KeyID, base64.StdEncoding.EncodeToString(key)); err != nil {
			return fmt.Errorf("OS keyring unavailable: %w", err)
		}
	}
	metadata, _ := json.Marshal(next)
	previous, _ := json.Marshal(old)
	if err := atomicCredentialWrite(path+".storage.json", metadata); err != nil {
		return err
	}
	if exists {
		if err := saveCredentialBytes(path, data); err != nil {
			_ = atomicCredentialWrite(path+".storage.json", previous)
			return err
		}
		check, err := credentialBytes(path)
		if err != nil || string(check) != string(data) {
			_ = atomicCredentialWrite(path+".storage.json", previous)
			return fmt.Errorf("credential migration verification failed")
		}
		stale := path
		if mode == "file" {
			stale = path + ".enc"
		}
		if err := os.Remove(stale); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if old.KeyID != "" {
		if err := keyring.Delete(keyringService, old.KeyID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	return nil
}

// RemoveStoredSession removes the profile's credentials and associated key.
func RemoveStoredSession(path string) error {
	m, err := readProtection(path)
	if err != nil {
		return err
	}
	for _, suffix := range []string{"", ".enc"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if m.KeyID != "" {
		if err := keyring.Delete(keyringService, m.KeyID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	if err := os.Remove(path + ".storage.json"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// QuarantineInaccessibleSession preserves inaccessible files before resetting mode.
func QuarantineInaccessibleSession(path string) error {
	suffix := ".quarantine-" + uuid.NewString()
	for _, ext := range []string{".enc", ".storage.json"} {
		if _, err := os.Stat(path + ext); err == nil {
			if err := os.Rename(path+ext, path+ext+suffix); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return atomicCredentialWrite(path+".storage.json", []byte(`{"mode":"file"}`))
}
