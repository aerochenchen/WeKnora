package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func ollamaHasModel(body []byte, want string) bool {
	var tags struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return false
	}
	for _, m := range tags.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		if name == want || strings.HasPrefix(name, want+":") {
			return true
		}
	}
	return false
}

func checkOllamaModel(client *http.Client, baseURL, model string) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama tags: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama tags: status %d", resp.StatusCode)
	}
	if !ollamaHasModel(body, model) {
		return fmt.Errorf("ollama is missing %s", model)
	}
	return nil
}
