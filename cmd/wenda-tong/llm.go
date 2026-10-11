package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type llmConfig struct {
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	ModelName string `json:"model_name"`
}

func readLLM(path string) (llmConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return llmConfig{}, fmt.Errorf("read llm.json: %w", err)
	}
	var cfg llmConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return llmConfig{}, fmt.Errorf("parse llm.json: %w", err)
	}
	if err := validateChatBaseURL(cfg.BaseURL); err != nil {
		return llmConfig{}, err
	}
	return cfg, nil
}

func validateChatBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("base_url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("base_url is invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("base_url scheme %q is not http or https", u.Scheme)
	}
}

func ssrfWhitelistExtra(chatBase string) (string, error) {
	if err := validateChatBaseURL(chatBase); err != nil {
		return "", err
	}
	u, _ := url.Parse(strings.TrimSpace(chatBase))
	hosts := []string{"127.0.0.1"}
	if host := u.Hostname(); host != "" && host != "127.0.0.1" {
		hosts = append(hosts, host)
	}
	return strings.Join(hosts, ","), nil
}
