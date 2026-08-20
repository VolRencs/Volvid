package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const downloadsDirFileName = ".volren_downloads_dir"

var ErrDownloadsDirLocked = errors.New("download location is fixed by VOLREN_DOWNLOADS_DIR")

func resolveDownloadsDir() string {
	if path := envPath(envDownloadsDir); path != "" {
		return path
	}
	if path := loadSavedDownloadsDir(); path != "" {
		return path
	}
	return systemDownloadsDir()
}

func DownloadsDirLocked() bool {
	return envPath(envDownloadsDir) != ""
}

func SetDownloadsDir(path string) error {
	if DownloadsDirLocked() {
		return ErrDownloadsDirLocked
	}

	path, err := prepareDir(path)
	if err != nil {
		return err
	}
	if err := saveDownloadsDir(path); err != nil {
		return err
	}

	DlDir = path
	return nil
}

func downloadsDirPath() string {
	return filepath.Join(ConfigDir, downloadsDirFileName)
}

func loadSavedDownloadsDir() string {
	path := downloadsDirPath()
	if strings.TrimSpace(path) == "" {
		return ""
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return cleanAbsPath(string(b))
}

func saveDownloadsDir(path string) error {
	path = cleanAbsPath(path)
	if path == "" {
		return errors.New("download location is empty")
	}
	return writeAppConfig(downloadsDirPath(), path+"\n")
}

func systemDownloadsDir() string {
	if path := systemDownloadsDirPlatform(); path != "" {
		return path
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "Downloads")
	}
	return filepath.Join(DataDir, "downloads")
}
