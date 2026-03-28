package app

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

type DownloadFragment struct {
	StartAt int
	EndAt   *int
}

func (f DownloadFragment) IsValid() bool {
	if f.StartAt < 0 {
		return false
	}
	if f.EndAt == nil {
		return true
	}
	return *f.EndAt > f.StartAt
}

func (f DownloadFragment) sectionArg() (string, bool) {
	if !f.IsValid() {
		return "", false
	}

	start := FormatClockTimestamp(f.StartAt)
	end := "inf"
	if f.EndAt != nil {
		end = FormatClockTimestamp(*f.EndAt)
	}
	return "*" + start + "-" + end, true
}

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

func ParseFragmentRange(raw string) (DownloadFragment, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DownloadFragment{}, fmt.Errorf("empty fragment range")
	}
	if strings.Count(value, "-") != 1 {
		return DownloadFragment{}, fmt.Errorf("invalid fragment range format")
	}

	startRaw, endRaw, _ := strings.Cut(value, "-")
	startAt, err := parseClockTimestamp(strings.TrimSpace(startRaw))
	if err != nil {
		return DownloadFragment{}, fmt.Errorf("invalid start time: %w", err)
	}
	endAt, err := parseClockTimestamp(strings.TrimSpace(endRaw))
	if err != nil {
		return DownloadFragment{}, fmt.Errorf("invalid end time: %w", err)
	}
	if startAt >= endAt {
		return DownloadFragment{}, fmt.Errorf("invalid fragment range bounds")
	}

	return DownloadFragment{
		StartAt: startAt,
		EndAt:   &endAt,
	}, nil
}

func ParseURLStartAt(rawURL string) (int, bool) {
	normalized := strings.TrimSpace(rawURL)
	if normalized == "" {
		return 0, false
	}
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return 0, false
	}

	if secs, ok := parseStartFromQuery(u.Query()); ok {
		return secs, true
	}
	return parseStartFromFragment(u.Fragment)
}

func parseStartFromQuery(values url.Values) (int, bool) {
	for _, key := range []string{"t", "start", "time_continue"} {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			if secs, ok := parseFlexibleTimestamp(value); ok && secs > 0 {
				return secs, true
			}
		}
	}
	return 0, false
}

func parseStartFromFragment(fragment string) (int, bool) {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return 0, false
	}

	if values, err := url.ParseQuery(fragment); err == nil {
		if secs, ok := parseStartFromQuery(values); ok {
			return secs, true
		}
	}

	secs, ok := parseFlexibleTimestamp(fragment)
	if !ok || secs <= 0 {
		return 0, false
	}
	return secs, true
}

func parseFlexibleTimestamp(raw string) (int, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return 0, false
	}

	if strings.Contains(value, ":") {
		secs, err := parseClockTimestamp(value)
		return secs, err == nil
	}

	if secs, ok := parseDigits(value); ok {
		return secs, true
	}

	secs := 0
	current := strings.Builder{}
	consumed := false
	for _, r := range value {
		switch {
		case unicode.IsDigit(r):
			current.WriteRune(r)
		case r == 'h' || r == 'm' || r == 's':
			if current.Len() == 0 {
				return 0, false
			}
			n, ok := parseDigits(current.String())
			if !ok {
				return 0, false
			}
			switch r {
			case 'h':
				secs += n * 3600
			case 'm':
				secs += n * 60
			case 's':
				secs += n
			}
			current.Reset()
			consumed = true
		default:
			return 0, false
		}
	}
	if current.Len() > 0 {
		if consumed {
			return 0, false
		}
		return parseDigits(current.String())
	}
	return secs, consumed
}

func parseClockTimestamp(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty timestamp")
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("timestamp must be mm:ss or hh:mm:ss")
	}

	numbers := make([]int, len(parts))
	for i, part := range parts {
		n, ok := parseDigits(part)
		if !ok {
			return 0, fmt.Errorf("invalid timestamp token %q", part)
		}
		numbers[i] = n
	}

	if len(numbers) == 2 {
		return parseClockParts(numbers[0], 0, numbers[1])
	}
	return parseClockParts(numbers[1], numbers[0], numbers[2])
}

func FormatClockTimestamp(totalSeconds int) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func parseDigits(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func parseClockParts(minutes, hours, seconds int) (int, error) {
	if seconds >= 60 {
		return 0, fmt.Errorf("seconds must be < 60")
	}
	if hours > 0 && minutes >= 60 {
		return 0, fmt.Errorf("minutes and seconds must be < 60")
	}
	return hours*3600 + minutes*60 + seconds, nil
}
