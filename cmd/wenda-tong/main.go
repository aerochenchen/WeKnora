package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "start")
	}
	exe, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	installDir := filepath.Dir(exe)
	root, err := resolveDataDir()
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "files"), 0o700); err != nil {
		fatal(err)
	}
	switch os.Args[1] {
	case "start":
		err = cmdStart(installDir, root)
	case "docreader":
		err = cmdDocreader(installDir, root)
	case "bootstrap":
		err = cmdBootstrap(installDir, root)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "wenda-tong: %v\n", err)
	os.Exit(1)
}
