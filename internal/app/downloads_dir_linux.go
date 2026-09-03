//go:build linux

package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func systemDownloadsDirPlatform() string {
	configDir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}

	b, err := os.ReadFile(filepath.Join(configDir, "user-dirs.dirs"))
	if err != nil {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	for _, rawLine := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(rawLine)
		rest, ok := strings.CutPrefix(line, "XDG_DOWNLOAD_DIR=")
		if !ok {
			continue
		}

		value := strings.TrimSpace(rest)
		if value == "" {
			return ""
		}

		unquoted, err := strconv.Unquote(value)
		if err != nil {
			unquoted = strings.Trim(value, `"'`)
		}

		if home == "" && (strings.Contains(unquoted, "$HOME") || strings.Contains(unquoted, "~")) {
			return ""
		}
		path := strings.ReplaceAll(unquoted, "${HOME}", home)
		path = strings.ReplaceAll(path, "$HOME", home)
		if rest, ok := strings.CutPrefix(path, "~/"); ok {
			path = filepath.Join(home, rest)
		}
		return cleanAbsPath(path)
	}

	return ""
}
