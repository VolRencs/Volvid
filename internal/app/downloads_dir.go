package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const downloadsDirFileName = ".volren_downloads_dir"

var ErrDownloadsDirLocked = errors.New("download location is fixed by VOLREN_DOWNLOADS_DIR")

func resolveDownloadsDir(env *Env) string {
	if path := envPath(envDownloadsDir); path != "" {
		return path
	}
	if path := loadSavedDownloadsDir(env); path != "" {
		return path
	}
	return systemDownloadsDir(env)
}

func DownloadsDirLocked() bool {
	return envPath(envDownloadsDir) != ""
}

func SetDownloadsDir(env *Env, path string) error {
	if DownloadsDirLocked() {
		return ErrDownloadsDirLocked
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

func downloadsDirPath(env *Env) string {
	return filepath.Join(env.ConfigDir, downloadsDirFileName)
}

func loadSavedDownloadsDir(env *Env) string {
	path := downloadsDirPath(env)
	if strings.TrimSpace(path) == "" {
		return ""
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return cleanAbsPath(string(b))
}

func saveDownloadsDir(env *Env, path string) error {
	path = cleanAbsPath(path)
	if path == "" {
		return errors.New("download location is empty")
	}
	return writeAppConfig(downloadsDirPath(env), path+"\n")
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
