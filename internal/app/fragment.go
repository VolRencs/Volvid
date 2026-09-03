package app

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrFragmentFormat           = errors.New("invalid fragment format")
	ErrFragmentBounds           = errors.New("invalid fragment bounds")
	ErrFragmentDurationRequired = errors.New("fragment duration is required")
	ErrFragmentOutOfRange       = errors.New("fragment exceeds media duration")
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

func parseFragmentRange(raw string) (DownloadFragment, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DownloadFragment{}, ErrFragmentFormat
	}

	if rest, ok := strings.CutSuffix(value, "+"); ok {
		startAt, err := parseClockTimestamp(strings.TrimSpace(rest))
		if err != nil {
			return DownloadFragment{}, fmt.Errorf("%w: %w", ErrFragmentFormat, err)
		}
		return DownloadFragment{StartAt: startAt}, nil
	}

	if strings.Count(value, "-") != 1 {
		return DownloadFragment{}, ErrFragmentFormat
	}

	startRaw, endRaw, _ := strings.Cut(value, "-")
	startAt, err := parseClockTimestamp(strings.TrimSpace(startRaw))
	if err != nil {
		return DownloadFragment{}, fmt.Errorf("%w: %w", ErrFragmentFormat, err)
	}
	endRaw = strings.TrimSpace(endRaw)
	if endRaw == "" {
		return DownloadFragment{StartAt: startAt}, nil
	}
	endAt, err := parseClockTimestamp(strings.TrimSpace(endRaw))
	if err != nil {
		return DownloadFragment{}, fmt.Errorf("%w: %w", ErrFragmentFormat, err)
	}
	if startAt >= endAt {
		return DownloadFragment{}, ErrFragmentBounds
	}

	return DownloadFragment{
		StartAt: startAt,
		EndAt:   &endAt,
	}, nil
}

func ParseBoundedFragmentRange(raw string, mediaDuration int) (DownloadFragment, error) {
	fragment, err := parseFragmentRange(raw)
	if err != nil {
		return DownloadFragment{}, err
	}
	if err := ValidateFragmentDuration(fragment, mediaDuration); err != nil {
		return DownloadFragment{}, err
	}
	return fragment, nil
}

func ValidateFragmentDuration(fragment DownloadFragment, mediaDuration int) error {
	if !fragment.IsValid() {
		return ErrFragmentBounds
	}
	if mediaDuration <= 0 {
		return ErrFragmentDurationRequired
	}
	if fragment.StartAt >= mediaDuration {
		return ErrFragmentOutOfRange
	}
	if fragment.EndAt != nil && *fragment.EndAt > mediaDuration {
		return ErrFragmentOutOfRange
	}
	return nil
}

func FragmentDurationText(l Locale, mediaDuration int) string {
	if mediaDuration <= 0 {
		return ""
	}
	return fmt.Sprintf(StringsFor(l).FragmentDurationFmt, FormatClockTimestamp(mediaDuration))
}

func FragmentInputHintFor(l Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentInputHint
	}
	return fmt.Sprintf(strs.FragmentInputHintWithDurationFmt, FormatClockTimestamp(mediaDuration))
}

func FragmentUnavailableText(l Locale) string {
	return StringsFor(l).FragmentUnavailable
}

func fragmentRangeOutOfBoundsText(l Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentUnavailable
	}
	return fmt.Sprintf(strs.FragmentInputOutOfBoundsFmt, FormatClockTimestamp(mediaDuration))
}

func FragmentURLStartOutOfBoundsText(l Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentUnavailable
	}
	return fmt.Sprintf(strs.FragmentURLStartOutOfBoundsFmt, FormatClockTimestamp(mediaDuration))
}

func FragmentInputErrorText(l Locale, err error, mediaDuration int) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrFragmentOutOfRange):
		return fragmentRangeOutOfBoundsText(l, mediaDuration)
	case errors.Is(err, ErrFragmentDurationRequired):
		return FragmentUnavailableText(l)
	case errors.Is(err, ErrFragmentBounds):
		return StringsFor(l).FragmentInputBadRange
	default:
		return StringsFor(l).FragmentInputBadFormat
	}
}

func parseURLStartAt(rawURL string) (int, bool) {
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
	totalSeconds = max(totalSeconds, 0)

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

func parseClockParts(minutes, hours, seconds int) (int, error) {
	if minutes < 0 || hours < 0 || seconds < 0 {
		return 0, fmt.Errorf("timestamp must not be negative")
	}
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
