package tui

// Subtitle multi-selection state (mirrors the playlist checklist pattern).

// Row 0 is the "no subtitles" action, rows 1..N are language tracks.
func (m Model) subtitleRowCount() int {
	return len(m.subTracks) + 1
}

func (m Model) selectedSubtitleCount() int {
	return len(m.subSelected)
}

func (m *Model) clearSubtitleSelection() {
	clear(m.subSelected)
	if m.subSelected == nil {
		m.subSelected = map[string]bool{}
	}
}

func (m *Model) toggleCurrentSubtitle() {
	if m.subCursor == 0 {
		m.clearSubtitleSelection()
		return
	}
	if m.subCursor < 0 || m.subCursor > len(m.subTracks) {
		return
	}
	lang := m.subTracks[m.subCursor-1].Lang
	if m.subSelected[lang] {
		delete(m.subSelected, lang)
		return
	}
	m.subSelected[lang] = true
}

func (m *Model) toggleAllSubtitles() {
	if len(m.subTracks) == 0 {
		m.clearSubtitleSelection()
		return
	}
	if len(m.subSelected) == len(m.subTracks) {
		m.clearSubtitleSelection()
		return
	}
	selected := make(map[string]bool, len(m.subTracks))
	for _, track := range m.subTracks {
		selected[track.Lang] = true
	}
	m.subSelected = selected
}

func (m Model) selectedSubtitleLangs() []string {
	langs := make([]string, 0, len(m.subSelected))
	for _, track := range m.subTracks {
		if m.subSelected[track.Lang] {
			langs = append(langs, track.Lang)
		}
	}
	return langs
}

func (m Model) stepSubtitleCursor(delta int) Model {
	if m.subtitleRowCount() == 0 {
		return m
	}
	m.subCursor = max(0, min(m.subCursor+delta, m.subtitleRowCount()-1))
	m.ensureSubtitleCursorVisible()
	return m
}

func (m *Model) ensureSubtitleCursorVisible() {
	height := m.playlistViewportHeight()
	if height <= 0 {
		m.subTop = 0
		return
	}
	if m.subCursor < m.subTop {
		m.subTop = m.subCursor
	}
	if m.subCursor >= m.subTop+height {
		m.subTop = m.subCursor - height + 1
	}
	maxTop := max(0, m.subtitleRowCount()-height)
	m.subTop = max(0, min(m.subTop, maxTop))
}
