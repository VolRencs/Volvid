package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appDirName      = "VolRenDownloader"
	envConfigDir    = "VOLREN_CONFIG_DIR"
	envDataDir      = "VOLREN_DATA_DIR"
	envDownloadsDir = "VOLREN_DOWNLOADS_DIR"
	envDepsDir      = "VOLREN_DEPS_DIR"
)

func initRuntimePaths(exeDir string) {
	AppDir = cleanAbsPath(exeDir)
	ConfigDir = resolveConfigDir()
	DataDir = resolveDataDir()
	DepsDir = resolveArtifactDir(envDepsDir, filepath.Join(DataDir, "deps"))
	DlDir = resolveDownloadsDir()
}

func resolveConfigDir() string {
	if path := envPath(envConfigDir); path != "" {
		return path
	}
	if root, ok := userConfigRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(AppDir, ".volren", "config")
}

func resolveDataDir() string {
	if path := envPath(envDataDir); path != "" {
		return path
	}
	if root, ok := userDataRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(AppDir, ".volren", "data")
}

func resolveArtifactDir(envKey, defaultPath string) string {
	if path := envPath(envKey); path != "" {
		return path
	}
	return cleanAbsPath(defaultPath)
}

func userConfigRoot() (string, bool) {
	root, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(root) == "" {
		return "", false
	}
	return root, true
}

func userDataRoot() (string, bool) {
	switch runtime.GOOS {
	case "windows":
		if root := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); root != "" {
			return cleanAbsPath(root), true
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, "Library", "Application Support"), true
		}
	default:
		if root := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); root != "" {
			return cleanAbsPath(root), true
		}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, ".local", "share"), true
		}
	}
	if root, ok := userConfigRoot(); ok {
		return root, true
	}
	return "", false
}

func envPath(key string) string {
	return cleanAbsPath(os.Getenv(key))
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}
