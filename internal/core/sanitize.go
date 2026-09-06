package core

import (
	"regexp"
	"strings"
)

var (
	invalidFilenameRE    = regexp.MustCompile(`[<>:"/\\|?*]`)
	windowsReservedNames = map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
		"COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
		"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
)

const maxSanitizedFilenameLen = 180

// SanitizeDirname makes a playlist title safe as a directory name:
// invalid chars become "_", trailing dots/spaces are trimmed, long names
// are cut and Windows-reserved basenames are prefixed.
func SanitizeDirname(name string) string {
	name = strings.TrimRight(
		invalidFilenameRE.ReplaceAllString(strings.TrimSpace(name), "_"),
		" .",
	)
	if r := []rune(name); len(r) > maxSanitizedFilenameLen {
		name = string(r[:maxSanitizedFilenameLen])
	}
	if name == "" {
		return "playlist"
	}
	base := name
	if idx := strings.IndexByte(name, '.'); idx >= 0 {
		base = name[:idx]
	}
	if windowsReservedNames[strings.ToUpper(base)] {
		name = "_" + name
	}
	return name
}

// SanitizeFileStem is the shared invalid-char replacement with a fallback
// for empty names (used for HTTP temp-file patterns).
func SanitizeFileStem(name, fallback string) string {
	name = invalidFilenameRE.ReplaceAllString(strings.TrimSpace(name), "_")
	if name == "" || name == "." {
		return fallback
	}
	return name
}
