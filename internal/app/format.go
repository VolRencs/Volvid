package app

import (
	"fmt"
	"regexp"
	"strings"
)

func FmtBytesFor(n int64, l Locale) string {
	ru := l == LocaleRU
	switch {
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

var invalidFilenameRE = regexp.MustCompile(`[<>:"/\\|?*]`)

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
	return name
}
