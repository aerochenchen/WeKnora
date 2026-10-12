package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDirUsesHome(t *testing.T) {
	got, err := dataDir(filepath.Join(string(os.PathSeparator), "Users", "ada"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join(".wenda-tong")) {
		t.Fatalf("data dir %q does not end with .wenda-tong", got)
	}
}

func TestDataDirRejectsEmptyHome(t *testing.T) {
	if _, err := dataDir(""); err == nil {
		t.Fatal("expected empty home to fail")
	}
}

func TestApplyDataPathsRejectsLiteralHome(t *testing.T) {
	err := applyDataPaths(map[string]string{"DB_PATH": "$HOME/.wenda-tong/data/weknora.db"}, "/tmp/x")
	if err == nil || !strings.Contains(err.Error(), "$HOME") {
		t.Fatalf("got %v", err)
	}
}

func TestApplyDataPathsOverridesEmpty(t *testing.T) {
	env := map[string]string{"DB_PATH": "", "LOCAL_STORAGE_BASE_DIR": ""}
	root := filepath.Join(string(os.PathSeparator), "home", "ada", ".wenda-tong")
	if err := applyDataPaths(env, root); err != nil {
		t.Fatal(err)
	}
	if env["DB_PATH"] != filepath.Join(root, "data", "weknora.db") {
		t.Fatalf("db path %q", env["DB_PATH"])
	}
	if env["LOCAL_STORAGE_BASE_DIR"] != filepath.Join(root, "data", "files") {
		t.Fatalf("files path %q", env["LOCAL_STORAGE_BASE_DIR"])
	}
}

func TestChatBaseURL(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8000/v1",
		"http://10.0.0.8:8000/v1",
		"https://api.openai.com/v1",
	} {
		if err := validateChatBaseURL(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "ftp://example.com/v1", "not a url"} {
		if err := validateChatBaseURL(raw); err == nil {
			t.Fatalf("%q should fail", raw)
		}
	}
}

func TestLoopbackConfig(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	if err := os.WriteFile(good, []byte("server:\n  host: \"127.0.0.1\"\n  port: 8090\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkLoopbackConfig(good); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("server:\n  host: \"0.0.0.0\"\n  port: 8090\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkLoopbackConfig(bad); err == nil {
		t.Fatal("expected non-loopback host to fail")
	}
}

func TestDocreaderDoesNotPullMissingImage(t *testing.T) {
	var calls []string
	run := fakeRunner{fn: func(name string, args []string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[0] == "image" {
			return nil, os.ErrNotExist
		}
		return []byte("127.0.0.1:50051"), nil
	}}
	err := ensureDocreader(run, "wechatopenai/weknora-docreader:wenda-tong")
	if err == nil {
		t.Fatal("expected missing image to fail")
	}
	for _, c := range calls {
		if strings.Contains(c, "pull") {
			t.Fatalf("pulled image: %s", c)
		}
	}
}

func TestDocreaderPublishAddress(t *testing.T) {
	run := fakeRunner{fn: func(name string, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "port" {
			return []byte("0.0.0.0:50051\n"), nil
		}
		return []byte("ok"), nil
	}}
	err := ensureDocreader(run, "local/docreader")
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:50051") {
		t.Fatalf("got %v", err)
	}
}

func TestOllamaRequiresBGE(t *testing.T) {
	if !ollamaHasModel([]byte(`{"models":[{"name":"bge-m3:latest"}]}`), "bge-m3") {
		t.Fatal("expected bge-m3:latest to match")
	}
	if ollamaHasModel([]byte(`{"models":[{"name":"qwen2"}]}`), "bge-m3") {
		t.Fatal("unexpected match")
	}
}

func TestProvisionAllowsPublicChatURL(t *testing.T) {
	var sawInit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/auth/register":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.URL.Path == "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"token":"jwt","active_tenant":{"id":7}}`))
		case r.URL.Path == "/api/v1/knowledge-bases":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"kb1"}}`))
		case r.URL.Path == "/api/v1/initialization/initialize/kb1":
			sawInit = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			llm := body["llm"].(map[string]any)
			if llm["baseUrl"] != "https://api.openai.com/v1" {
				t.Fatalf("baseUrl %v", llm["baseUrl"])
			}
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.URL.Path == "/api/v1/tenants/7/api-keys":
			var body struct {
				Capabilities []string `json:"capabilities"`
				FullAccess   bool     `json:"full_access"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.FullAccess || len(body.Capabilities) != 1 || body.Capabilities[0] != "retrieve" {
				t.Fatalf("key body %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"token":"rk_test"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	cfg, err := provision(srv.Client(), srv.URL, root, llmConfig{
		BaseURL:   "https://api.openai.com/v1",
		APIKey:    "sk-test",
		ModelName: "gpt-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawInit {
		t.Fatal("initialize was not called")
	}
	if cfg.APIKey != "rk_test" || cfg.KnowledgeBaseID != "kb1" || cfg.BaseURL != srv.URL {
		t.Fatalf("%+v", cfg)
	}
	info, err := os.Stat(filepath.Join(root, "account.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("account mode %o", info.Mode().Perm())
	}
}

func TestEmptyBaseURLDoesNotInitialize(t *testing.T) {
	if err := validateChatBaseURL(""); err == nil {
		t.Fatal("empty base url should fail before any request")
	}
}

func TestLoopbackOpenClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	if err := loopbackOpen(addr); err == nil {
		t.Fatal("closed port should stop start before the server process")
	}
}

func TestOllamaWithoutBGEDoesNotProvision(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest"}]}`))
	}))
	defer ollama.Close()
	if err := checkOllamaModel(ollama.Client(), ollama.URL, "bge-m3"); err == nil {
		t.Fatal("missing bge-m3 should fail before register")
	}
}

type fakeRunner struct {
	fn func(name string, args []string) ([]byte, error)
}

func (f fakeRunner) Output(name string, args ...string) ([]byte, error) {
	return f.fn(name, args)
}
