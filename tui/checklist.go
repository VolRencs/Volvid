package tui

import "strconv"

// checklist is the shared multi-select list behind the subtitle and audio
// track pickers. Row 0 is the "no override" action; rows 1..N map to tracks.
// Selection is keyed by a track key (the language tag).
type checklist[T any] struct {
	cursor   int
	top      int
	selected map[string]bool
	key      func(T) string
}

func newChecklist[T any](key func(T) string) checklist[T] {
	return checklist[T]{selected: map[string]bool{}, key: key}
}

// reset clears cursor, scroll offset and selection.
func (c *checklist[T]) reset() {
	c.cursor = 0
	c.top = 0
	c.selected = map[string]bool{}
}

func (c checklist[T]) rowCount(tracks []T) int {
	return len(tracks) + 1
}

func (c checklist[T]) isSelected(track T) bool {
	return c.selected[c.key(track)]
}

func (c *checklist[T]) clearSelection() {
	clear(c.selected)
	if c.selected == nil {
		c.selected = map[string]bool{}
	}
}

func (c *checklist[T]) trackAt(tracks []T) (T, bool) {
	var zero T
	idx := c.cursor - 1
	if idx < 0 || idx >= len(tracks) {
		return zero, false
	}
	return tracks[idx], true
}

func (c *checklist[T]) toggle(tracks []T) {
	if c.cursor == 0 {
		c.clearSelection()
		return
	}
	track, ok := c.trackAt(tracks)
	if !ok {
		return
	}
	key := c.key(track)
	if c.selected[key] {
		delete(c.selected, key)
		return
	}
	c.selected[key] = true
}

func (c *checklist[T]) toggleAll(tracks []T) {
	if len(tracks) == 0 {
		c.clearSelection()
		return
	}
	if len(c.selected) == len(tracks) {
		c.clearSelection()
		return
	}
	selected := make(map[string]bool, len(tracks))
	for _, track := range tracks {
		selected[c.key(track)] = true
	}
	c.selected = selected
}

// selectedKeys returns selected track keys in track order.
func (c checklist[T]) selectedKeys(tracks []T) []string {
	keys := make([]string, 0, len(c.selected))
	for _, track := range tracks {
		if key := c.key(track); c.selected[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

func (c *checklist[T]) move(delta int, tracks []T, viewportHeight int) {
	rows := c.rowCount(tracks)
	c.cursor = max(0, min(c.cursor+delta, rows-1))
	c.top = clampWindowTop(c.cursor, c.top, rows, viewportHeight)
}

// clampWindowTop keeps cursor inside the visible [top, top+height) window.
func clampWindowTop(cursor, top, rows, height int) int {
	if height <= 0 {
		return 0
	}
	if cursor < top {
		top = cursor
	}
	if cursor >= top+height {
		top = cursor - height + 1
	}
	return max(0, min(top, max(0, rows-height)))
}

// viewportRangeText renders the "  ·  start-end/total" scroll hint.
func viewportRangeText(top, rows, height int) string {
	if rows <= 0 || height <= 0 || rows <= height {
		return ""
	}
	start := top + 1
	end := min(rows, top+height)
	return "  ·  " + strconv.Itoa(start) + "-" + strconv.Itoa(end) + "/" + strconv.Itoa(rows)
}
