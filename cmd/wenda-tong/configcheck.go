package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type serverFile struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

func checkLoopbackConfig(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var cfg serverFile
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		return fmt.Errorf("server.host is %q, want 127.0.0.1", cfg.Server.Host)
	}
	return nil
}
