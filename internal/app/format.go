package app

import (
	"fmt"
	"regexp"
	"strings"
)

func FmtBytesFor(n int64, l Locale) string {
	ru := l == LocaleRU
	switch {
	case n >= 1_099_511_627_776:
		if ru {
			return fmt.Sprintf("%.2f ТБ", float64(n)/1_099_511_627_776)
		}
		return fmt.Sprintf("%.2f TB", float64(n)/1_099_511_627_776)
	case n >= 1_073_741_824:
		if ru {
			return fmt.Sprintf("%.2f ГБ", float64(n)/1_073_741_824)
		}
		return fmt.Sprintf("%.2f GB", float64(n)/1_073_741_824)
	case n >= 1_048_576:
		if ru {
			return fmt.Sprintf("%.1f МБ", float64(n)/1_048_576)
		}
		return fmt.Sprintf("%.1f MB", float64(n)/1_048_576)
	case n >= 1_024:
		if ru {
			return fmt.Sprintf("%d КБ", n/1_024)
		}
		return fmt.Sprintf("%d KB", n/1_024)
	default:
		if ru {
			return fmt.Sprintf("%d Б", n)
		}
		return fmt.Sprintf("%d B", n)
	}
}

func FmtDuration(secs int) string {
	if secs <= 0 {
		return "??:??"
	}
	h, m, s := secs/3600, (secs%3600)/60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

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

func SanitizeDirname(name string) string {
	name = strings.TrimRight(
		invalidFilenameRE.ReplaceAllString(strings.TrimSpace(name), "_"),
		" .",
	)
	if r := []rune(name); len(r) > 180 {
		name = string(r[:180])
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
