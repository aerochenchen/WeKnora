package main

import (
	"fmt"
	"strings"
)

type commandRunner interface {
	Output(name string, args ...string) ([]byte, error)
}

func docreaderRunArgs(image string) []string {
	return []string{
		"run", "--rm", "-d",
		"--name", "wenda-tong-docreader",
		"-p", "127.0.0.1:50051:50051",
		image,
	}
}

func ensureDocreader(run commandRunner, image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return fmt.Errorf("DOCREADER_IMAGE is empty")
	}
	if _, err := run.Output("docker", "image", "inspect", image); err != nil {
		return fmt.Errorf("docreader image %s is not loaded locally: %w", image, err)
	}
	if _, err := run.Output("docker", docreaderRunArgs(image)...); err != nil {
		return fmt.Errorf("start docreader: %w", err)
	}
	out, err := run.Output("docker", "port", "wenda-tong-docreader", "50051")
	if err != nil {
		_, _ = run.Output("docker", "rm", "-f", "wenda-tong-docreader")
		return fmt.Errorf("docker port: %w", err)
	}
	if strings.TrimSpace(string(out)) != "127.0.0.1:50051" {
		_, _ = run.Output("docker", "rm", "-f", "wenda-tong-docreader")
		return fmt.Errorf("docreader published on %q, want 127.0.0.1:50051", strings.TrimSpace(string(out)))
	}
	return nil
}
