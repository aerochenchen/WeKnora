package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env: %w", err)
	}
	defer f.Close()

	env := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("env line %q has no '='", line)
		}
		env[strings.TrimSpace(key)] = val
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return env, nil
}

func applyDataPaths(env map[string]string, root string) error {
	for _, key := range []string{"DB_PATH", "LOCAL_STORAGE_BASE_DIR"} {
		if strings.Contains(env[key], "$HOME") {
			return fmt.Errorf("%s contains literal $HOME", key)
		}
	}
	env["DB_PATH"] = filepath.Join(root, "data", "weknora.db")
	env["LOCAL_STORAGE_BASE_DIR"] = filepath.Join(root, "data", "files")
	return nil
}
