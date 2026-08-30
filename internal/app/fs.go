package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func writeAppConfig(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func prepareDir(path string) (string, error) {
	path = cleanAbsPath(path)
	if path == "" {
		return "", errors.New("directory path is empty")
	}

	info, err := os.Stat(path)
	switch {
	case err == nil:
		if !info.IsDir() {
			return "", fmt.Errorf("path is not a directory: %s", path)
		}
		return path, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("access directory %s: %w", path, err)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", path, err)
	}
	return path, nil
}

func OpenInFileManager(path string) error {
	path, err := prepareDir(path)
	if err != nil {
		return fmt.Errorf("prepare folder: %w", err)
	}

	name := "xdg-open"
	switch runtime.GOOS {
	case "windows":
		name = "explorer.exe"
	case "darwin":
		name = "open"
	}

	cmd := exec.Command(name, path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open file manager: %w", err)
	}
	go cmd.Wait()
	return nil
}
