package tui

// Audio track multi-selection state (mirrors the subtitle checklist pattern).

// Row 0 is the "original" action, rows 1..N are language tracks.
func (m Model) audioRowCount() int {
	return len(m.audioTracks) + 1
}

func (m Model) selectedAudioCount() int {
	return len(m.audioSelected)
}

func (m *Model) clearAudioSelection() {
	clear(m.audioSelected)
	if m.audioSelected == nil {
		m.audioSelected = map[string]bool{}
	}
}

func (m *Model) toggleCurrentAudioTrack() {
	if m.audioCursor == 0 {
		m.clearAudioSelection()
		return
	}
	if m.audioCursor < 0 || m.audioCursor > len(m.audioTracks) {
		return
	}
	lang := m.audioTracks[m.audioCursor-1].Lang
	if m.audioSelected[lang] {
		delete(m.audioSelected, lang)
		return
	}
	m.audioSelected[lang] = true
}

func (m *Model) toggleAllAudioTracks() {
	if len(m.audioTracks) == 0 {
		m.clearAudioSelection()
		return
	}
	if len(m.audioSelected) == len(m.audioTracks) {
		m.clearAudioSelection()
		return
	}
	selected := make(map[string]bool, len(m.audioTracks))
	for _, track := range m.audioTracks {
		selected[track.Lang] = true
	}
	m.audioSelected = selected
}

func (m Model) selectedAudioLangs() []string {
	langs := make([]string, 0, len(m.audioSelected))
	for _, track := range m.audioTracks {
		if m.audioSelected[track.Lang] {
			langs = append(langs, track.Lang)
		}
	}
	return langs
}

func (m Model) stepAudioCursor(delta int) Model {
	if m.audioRowCount() == 0 {
		return m
	}
	m.audioCursor = max(0, min(m.audioCursor+delta, m.audioRowCount()-1))
	m.ensureAudioCursorVisible()
	return m
}

func (m *Model) ensureAudioCursorVisible() {
	height := m.playlistViewportHeight()
	if height <= 0 {
		m.audioTop = 0
		return
	}
	if m.audioCursor < m.audioTop {
		m.audioTop = m.audioCursor
	}
	if m.audioCursor >= m.audioTop+height {
		m.audioTop = m.audioCursor - height + 1
	}
	maxTop := max(0, m.audioRowCount()-height)
	m.audioTop = max(0, min(m.audioTop, maxTop))
}
