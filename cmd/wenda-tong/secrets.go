package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func ensureHexSecret(path string, nbytes int) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		return string(b), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	buf := make([]byte, nbytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	text := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	if err := restrictFile(path); err != nil {
		return "", fmt.Errorf("restrict %s: %w", path, err)
	}
	return text, nil
}
