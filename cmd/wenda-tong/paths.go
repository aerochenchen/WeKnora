package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func dataDir(home string) (string, error) {
	if home == "" {
		return "", fmt.Errorf("home directory is empty")
	}
	return filepath.Join(home, ".wenda-tong"), nil
}

func resolveDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return dataDir(home)
}
