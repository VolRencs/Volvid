//go:build !windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func applyUpdatePlatform(tmp, dest string) error {
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("chmod нового бинарника: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("замена бинарника: %w", err)
	}
	return nil
}

func OpenInFileManager(path string) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
		path = abs
	}

	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}

	cmd := exec.Command(name, path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open file manager: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}
