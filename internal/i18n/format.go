package i18n

import (
	"fmt"

	"volvid/internal/core"
)

func FormatBytes(n int64, l core.Locale) string {
	ru := l == core.LocaleRU
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

func FormatSpeed(bytesPerSec int64, l core.Locale) string {
	suffix := "/s"
	if l == core.LocaleRU {
		suffix = "/с"
	}
	return FormatBytes(bytesPerSec, l) + suffix
}
