package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appDirName      = "Volvid"
	envConfigDir    = "VOLVID_CONFIG_DIR"
	envDataDir      = "VOLVID_DATA_DIR"
	envDownloadsDir = "VOLVID_DOWNLOADS_DIR"
	envDepsDir      = "VOLVID_DEPS_DIR"
)

func (env *Env) initRuntimePaths(exeDir string) {
	env.AppDir = cleanAbsPath(exeDir)
	env.ConfigDir = resolveConfigDir(env)
	env.DataDir = resolveDataDir(env)
	env.DepsDir = resolveArtifactDir(envDepsDir, filepath.Join(env.DataDir, "deps"))
	env.downloadsDir = resolveDownloadsDir(env)
}

func resolveConfigDir(env *Env) string {
	if path := envPath(envConfigDir); path != "" {
		return path
	}
	if root, ok := userConfigRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(env.AppDir, ".volvid", "config")
}

func resolveDataDir(env *Env) string {
	if path := envPath(envDataDir); path != "" {
		return path
	}
	if root, ok := userDataRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(env.AppDir, ".volvid", "data")
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
	if path := cleanAbsPath(os.Getenv(key)); path != "" {
		return path
	}
	if legacy, ok := legacyEnvKey(key); ok {
		return cleanAbsPath(os.Getenv(legacy))
	}
	return ""
}

func legacyEnvKey(key string) (string, bool) {
	if rest, ok := strings.CutPrefix(key, "VOLVID_"); ok {
		return "VOLREN_" + rest, true
	}
	return "", false
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return filepath.Clean(abs)
}
