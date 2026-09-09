package core

import (
	"errors"
	"fmt"
	"strings"
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

// SectionArg renders the yt-dlp --download-sections value ("*start-end").
func (f DownloadFragment) SectionArg() (string, bool) {
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
