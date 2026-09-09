package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

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

func parseDigits(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return 0, false
		}
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return n, true
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
		return clockSeconds(0, numbers[0], numbers[1])
	}
	return clockSeconds(numbers[0], numbers[1], numbers[2])
}

func clockSeconds(hours, minutes, seconds int) (int, error) {
	if seconds >= 60 {
		return 0, fmt.Errorf("seconds must be < 60")
	}
	if hours > 0 && minutes >= 60 {
		return 0, fmt.Errorf("minutes and seconds must be < 60")
	}
	if hours > math.MaxInt/3600 || minutes > math.MaxInt/60 || seconds > math.MaxInt-hours*3600-minutes*60 {
		return 0, fmt.Errorf("timestamp is too large")
	}
	return hours*3600 + minutes*60 + seconds, nil
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
			mult := 1
			switch r {
			case 'h':
				mult = 3600
			case 'm':
				mult = 60
			}
			if n > math.MaxInt/mult || secs > math.MaxInt-n*mult {
				return 0, false
			}
			secs += n * mult
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
