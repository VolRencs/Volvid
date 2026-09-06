package i18n

import (
	"errors"
	"fmt"

	"volvid/internal/core"
)

func FragmentDurationText(l core.Locale, mediaDuration int) string {
	if mediaDuration <= 0 {
		return ""
	}
	return fmt.Sprintf(StringsFor(l).FragmentDurationFmt, core.FormatClockTimestamp(mediaDuration))
}

func FragmentInputHintFor(l core.Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentInputHint
	}
	return fmt.Sprintf(strs.FragmentInputHintWithDurationFmt, core.FormatClockTimestamp(mediaDuration))
}

func FragmentUnavailableText(l core.Locale) string {
	return StringsFor(l).FragmentUnavailable
}

func fragmentRangeOutOfBoundsText(l core.Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentUnavailable
	}
	return fmt.Sprintf(strs.FragmentInputOutOfBoundsFmt, core.FormatClockTimestamp(mediaDuration))
}

func FragmentURLStartOutOfBoundsText(l core.Locale, mediaDuration int) string {
	strs := StringsFor(l)
	if mediaDuration <= 0 {
		return strs.FragmentUnavailable
	}
	return fmt.Sprintf(strs.FragmentURLStartOutOfBoundsFmt, core.FormatClockTimestamp(mediaDuration))
}

func FragmentInputErrorText(l core.Locale, err error, mediaDuration int) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, core.ErrFragmentOutOfRange):
		return fragmentRangeOutOfBoundsText(l, mediaDuration)
	case errors.Is(err, core.ErrFragmentDurationRequired):
		return FragmentUnavailableText(l)
	case errors.Is(err, core.ErrFragmentBounds):
		return StringsFor(l).FragmentInputBadRange
	default:
		return StringsFor(l).FragmentInputBadFormat
	}
}
