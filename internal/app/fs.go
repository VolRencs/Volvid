package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

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
