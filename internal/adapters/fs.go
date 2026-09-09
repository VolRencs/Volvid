package adapters

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func writeAppConfig(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(content), 0o644)
}

// binaryBaseName returns the base name of a binary path.
func binaryBaseName(path string) string {
	return filepath.Base(strings.TrimSpace(path))
}

// writeStagedFile streams src (up to maxBytes, <0 = unlimited) into a temp
// file next to dest, chmods and renames it atomically.
func writeStagedFile(src io.Reader, dest string, perm os.FileMode, maxBytes int64) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".staged-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if maxBytes >= 0 {
		src = io.LimitReader(src, maxBytes+1)
	}
	n, err := io.Copy(tmp, src)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if maxBytes >= 0 && n > maxBytes {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("staged file exceeds %d bytes", maxBytes)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return writeStagedFile(bytes.NewReader(data), path, perm, -1)
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
	}

	cmd := exec.Command(name, path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open file manager: %w", err)
	}
	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = cmd.Wait()
		}()
		select {
		case <-done:
		case <-timer.C:
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	return nil
}
