package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func loopbackOpen(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("%s is not accepting connections: %w", addr, err)
	}
	return conn.Close()
}

func loadRuntimeEnv(installDir, root string) (map[string]string, error) {
	env, err := readEnvFile(filepath.Join(installDir, "env"))
	if err != nil {
		return nil, err
	}
	if err := applyDataPaths(env, root); err != nil {
		return nil, err
	}
	if err := checkLoopbackConfig(filepath.Join(installDir, "config.yaml")); err != nil {
		return nil, err
	}
	aes, err := ensureHexSecret(filepath.Join(root, "system_aes_key"), 16)
	if err != nil {
		return nil, err
	}
	jwt, err := ensureHexSecret(filepath.Join(root, "jwt_secret"), 32)
	if err != nil {
		return nil, err
	}
	env["SYSTEM_AES_KEY"] = aes
	env["JWT_SECRET"] = jwt
	if b, err := os.ReadFile(filepath.Join(root, "ssrf_whitelist_extra")); err == nil {
		env["SSRF_WHITELIST_EXTRA"] = string(b)
	}
	return env, nil
}

func envList(base []string, overlay map[string]string) []string {
	out := append([]string{}, base...)
	for k, v := range overlay {
		out = append(out, k+"="+v)
	}
	return out
}

type execRunner struct{}

func (execRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func cmdStart(installDir, root string) error {
	env, err := loadRuntimeEnv(installDir, root)
	if err != nil {
		return err
	}
	addr := env["DOCREADER_ADDR"]
	if addr == "" {
		addr = "127.0.0.1:50051"
	}
	if err := loopbackOpen(addr); err != nil {
		return err
	}
	lite := filepath.Join(installDir, "WeKnora-lite")
	if _, err := os.Stat(lite); err != nil {
		liteExe := lite + ".exe"
		if _, exeErr := os.Stat(liteExe); exeErr != nil {
			return fmt.Errorf("lite binary not found next to the launcher")
		}
		lite = liteExe
	}
	cmd := exec.Command(lite)
	cmd.Dir = installDir
	cmd.Env = envList(os.Environ(), env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdDocreader(installDir, root string) error {
	env, err := loadRuntimeEnv(installDir, root)
	if err != nil {
		return err
	}
	return ensureDocreader(execRunner{}, env["DOCREADER_IMAGE"])
}

func cmdBootstrap(installDir, root string) error {
	env, err := loadRuntimeEnv(installDir, root)
	if err != nil {
		return err
	}
	llm, err := readLLM(filepath.Join(root, "llm.json"))
	if err != nil {
		return err
	}
	ollama := env["OLLAMA_BASE_URL"]
	if ollama == "" {
		ollama = "http://127.0.0.1:11434"
	}
	if err := checkOllamaModel(nil, ollama, "bge-m3"); err != nil {
		return err
	}
	extra, err := ssrfWhitelistExtra(llm.BaseURL)
	if err != nil {
		return err
	}
	extraPath := filepath.Join(root, "ssrf_whitelist_extra")
	if err := os.WriteFile(extraPath, []byte(extra), 0o600); err != nil {
		return err
	}
	if err := restrictFile(extraPath); err != nil {
		return err
	}
	cfgPath := filepath.Join(root, "config.json")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	}
	cfg, err := provision(nil, "http://127.0.0.1:8090", root, llm)
	if err != nil {
		return err
	}
	return writeCorpusConfig(cfgPath, cfg)
}
