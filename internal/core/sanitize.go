package core

import (
	"regexp"
	"slices"
	"strings"
)

var (
	invalidFilenameRE    = regexp.MustCompile(`[<>:"/\\|?*]`)
	windowsReservedNames = []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
)

const maxSanitizedFilenameLen = 180

func sanitize(name, fallback string, trimDots bool, checkReserved bool) string {
	name = invalidFilenameRE.ReplaceAllString(strings.TrimSpace(name), "_")
	if trimDots {
		name = strings.TrimRight(name, " .")
		if r := []rune(name); len(r) > maxSanitizedFilenameLen {
			name = string(r[:maxSanitizedFilenameLen])
		}
	}
	if name == "" || name == "." {
		return fallback
	}
	if checkReserved {
		base := name
		if idx := strings.IndexByte(name, '.'); idx >= 0 {
			base = name[:idx]
		}
		if slices.Contains(windowsReservedNames, strings.ToUpper(base)) {
			name = "_" + name
		}
	}
	return name
}

// SanitizeDirname makes a playlist title safe as a directory name.
func SanitizeDirname(name string) string {
	return sanitize(name, "playlist", true, true)
}

// SanitizeFileStem is the shared invalid-char replacement with a fallback.
func SanitizeFileStem(name, fallback string) string {
	return sanitize(name, fallback, false, false)
}
