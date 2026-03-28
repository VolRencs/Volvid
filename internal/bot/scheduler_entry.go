package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func normalizeTimerEntry(entry TimerEntry, allowImmediatePast bool) (TimerEntry, time.Duration, bool, error) {
	if strings.TrimSpace(entry.ID) == "" {
		return TimerEntry{}, 0, false, errors.New("timer id is required")
	}

	interval, err := entry.Duration()
	if err != nil {
		return TimerEntry{}, 0, false, err
	}
	if interval <= 0 {
		return TimerEntry{}, 0, false, fmt.Errorf("invalid interval %q", entry.Interval)
	}

	originalNextRun := entry.NextRun
	if entry.NextRun.IsZero() {
		entry.NextRun = time.Now().Add(interval)
	}
	if !allowImmediatePast && entry.NextRun.Before(time.Now()) {
		entry.NextRun = time.Now().Add(interval)
	}

	changed := originalNextRun.IsZero() != entry.NextRun.IsZero() || !originalNextRun.Equal(entry.NextRun)
	return entry, interval, changed, nil
}

func timerEntryEqual(left, right TimerEntry) bool {
	return left.ID == right.ID &&
		left.Interval == right.Interval &&
		left.NextRun.Equal(right.NextRun) &&
		left.Type == right.Type &&
		left.Text == right.Text &&
		left.Caption == right.Caption &&
		left.SourceChatID == right.SourceChatID &&
		left.SourceMessageID == right.SourceMessageID
}
