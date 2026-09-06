package core

import "fmt"

// FormatClockTimestamp renders seconds as mm:ss or hh:mm:ss (locale-free).
func FormatClockTimestamp(totalSeconds int) string {
	totalSeconds = max(totalSeconds, 0)

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// FormatFragmentLabel renders a fragment as "mm:ss+" or "mm:ss-mm:ss".
func FormatFragmentLabel(fragment *DownloadFragment) string {
	if fragment == nil || !fragment.IsValid() {
		return ""
	}
	start := FormatClockTimestamp(fragment.StartAt)
	if fragment.EndAt == nil {
		return start + "+"
	}
	return start + "-" + FormatClockTimestamp(*fragment.EndAt)
}

// FormatDuration renders seconds as m:ss / h:mm:ss ("??:??" when unknown).
func FormatDuration(secs int) string {
	if secs <= 0 {
		return "??:??"
	}
	h, m, s := secs/3600, (secs%3600)/60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
