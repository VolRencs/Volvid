package adapters

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"volvid/internal/core"
)

const downloadsDirFileName = ".volvid_downloads_dir"

func resolveDownloadsDir(env *Env) string {
	if path := envPath(envDownloadsDir); path != "" {
		return path
	}
	if path := loadValidDirDotFile(env, downloadsDirFileName); path != "" {
		return path
	}
	return systemDownloadsDir(env)
}

func DownloadsDirLocked() bool {
	return envPath(envDownloadsDir) != ""
}

func SetDownloadsDir(env *Env, path string) error {
	if DownloadsDirLocked() {
		return core.ErrDownloadsDirLocked
	}

	path, err := prepareDir(path)
	if err != nil {
		return err
	}
	if err := saveDownloadsDir(env, path); err != nil {
		return err
	}

	env.setDownloadsDir(path)
	return nil
}

func saveDownloadsDir(env *Env, path string) error {
	path = cleanAbsPath(path)
	if path == "" {
		return errors.New("download location is empty")
	}
	return saveDotFile(env, downloadsDirFileName, path)
}

func systemDownloadsDir(env *Env) string {
	if path := systemDownloadsDirPlatform(); path != "" {
		return path
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "Downloads")
	}
	return filepath.Join(env.DataDir, "downloads")
}
