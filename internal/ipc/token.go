package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const tokenFileName = "automation.token"

// TokenPath is the random token file next to config.toml.
func TokenPath(configPath string) string {
	return filepath.Join(filepath.Dir(strings.TrimSpace(configPath)), tokenFileName)
}

// ReadToken returns the token only when the file is readable by the user alone.
func ReadToken(path string) (string, error) {
	if err := fileIsUserOnly(path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if !validToken(token) {
		return "", fmt.Errorf("automation token file is invalid")
	}
	return token, nil
}

// EnsureToken returns the existing user-only token or writes a new one.
// A loose file is replaced so a world-readable secret cannot linger.
func EnsureToken(path string) (string, error) {
	if token, err := ReadToken(path); err == nil {
		return token, nil
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if err := writeUserOnly(path, []byte(token+"\n")); err != nil {
		return "", err
	}
	return token, nil
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

func writeUserOnly(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := hardenFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return hardenFile(path)
}
