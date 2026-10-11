package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type corpusConfig struct {
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

type accountFile struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func writeCorpusConfig(path string, cfg corpusConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return restrictFile(path)
}

func provision(client *http.Client, apiBase string, root string, llm llmConfig) (corpusConfig, error) {
	if client == nil {
		client = http.DefaultClient
	}
	apiBase = strings.TrimRight(apiBase, "/")
	acct, err := ensureAccount(root)
	if err != nil {
		return corpusConfig{}, err
	}
	if _, err := postJSON(client, apiBase+"/api/v1/auth/register", map[string]string{
		"username": "wendatong",
		"email":    acct.Email,
		"password": acct.Password,
	}, ""); err != nil && !strings.Contains(err.Error(), "status 400") && !strings.Contains(err.Error(), "status 409") {
		return corpusConfig{}, err
	}
	loginBody, err := postJSON(client, apiBase+"/api/v1/auth/login", map[string]string{
		"email":    acct.Email,
		"password": acct.Password,
	}, "")
	if err != nil {
		return corpusConfig{}, err
	}
	var login struct {
		Token        string `json:"token"`
		ActiveTenant struct {
			ID uint64 `json:"id"`
		} `json:"active_tenant"`
	}
	if err := json.Unmarshal(loginBody, &login); err != nil {
		return corpusConfig{}, err
	}
	if login.Token == "" || login.ActiveTenant.ID == 0 {
		return corpusConfig{}, fmt.Errorf("login response missing token or tenant")
	}
	kbBody, err := postJSON(client, apiBase+"/api/v1/knowledge-bases", map[string]string{
		"name": "问答资料",
	}, login.Token)
	if err != nil {
		return corpusConfig{}, err
	}
	var kb struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(kbBody, &kb); err != nil {
		return corpusConfig{}, err
	}
	if kb.Data.ID == "" {
		return corpusConfig{}, fmt.Errorf("knowledge base response missing id")
	}
	initBody := map[string]any{
		"llm": map[string]string{
			"source":    "remote",
			"modelName": llm.ModelName,
			"baseUrl":   llm.BaseURL,
			"apiKey":    llm.APIKey,
		},
		"embedding": map[string]any{
			"source":    "local",
			"modelName": "bge-m3",
			"baseUrl":   "http://127.0.0.1:11434",
			"dimension": 1024,
		},
		"rerank":             map[string]bool{"enabled": false},
		"multimodal":         map[string]bool{"enabled": false},
		"documentSplitting":  map[string]any{"chunkSize": 512, "chunkOverlap": 80, "separators": []string{"\n\n", "\n", "。"}},
		"nodeExtract":        map[string]bool{"enabled": false},
		"questionGeneration": map[string]bool{"enabled": false},
	}
	if _, err := postJSON(client, apiBase+"/api/v1/initialization/initialize/"+kb.Data.ID, initBody, login.Token); err != nil {
		return corpusConfig{}, err
	}
	keyBody, err := postJSON(client, fmt.Sprintf("%s/api/v1/tenants/%d/api-keys", apiBase, login.ActiveTenant.ID), map[string]any{
		"name":               "文书通检索",
		"full_access":        false,
		"knowledge_base_ids": []string{kb.Data.ID},
		"capabilities":       []string{"retrieve"},
	}, login.Token)
	if err != nil {
		return corpusConfig{}, err
	}
	var key struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(keyBody, &key); err != nil {
		return corpusConfig{}, err
	}
	if key.Data.Token == "" {
		return corpusConfig{}, fmt.Errorf("api key response missing token")
	}
	return corpusConfig{
		BaseURL:         apiBase,
		APIKey:          key.Data.Token,
		KnowledgeBaseID: kb.Data.ID,
	}, nil
}

func ensureAccount(root string) (accountFile, error) {
	path := filepath.Join(root, "account.json")
	if b, err := os.ReadFile(path); err == nil {
		var acct accountFile
		if err := json.Unmarshal(b, &acct); err != nil {
			return accountFile{}, err
		}
		return acct, nil
	} else if !os.IsNotExist(err) {
		return accountFile{}, err
	}
	password, err := ensureHexSecret(filepath.Join(root, "account.password"), 12)
	if err != nil {
		return accountFile{}, err
	}
	acct := accountFile{Email: "wendatong@local.test", Password: "Aa1" + password[:13]}
	b, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return accountFile{}, err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return accountFile{}, err
	}
	if err := restrictFile(path); err != nil {
		return accountFile{}, err
	}
	return acct, nil
}

func postJSON(client *http.Client, url string, body any, token string) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}
