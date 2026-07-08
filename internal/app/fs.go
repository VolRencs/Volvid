package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func prepareDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("directory path is empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve directory path: %w", err)
	}
	path = filepath.Clean(abs)

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
