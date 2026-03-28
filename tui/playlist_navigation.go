package tui

import app "YouTubeBuild/internal/app"

func (m Model) selectedPlaylistEntries() []app.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}

	selected := make([]app.PlaylistEntry, 0, len(m.plSelected))
	for _, entry := range m.plInfo.Entries {
		if m.plSelected[entry.Index] {
			selected = append(selected, entry)
		}
	}
	return selected
}

func (m Model) stepPlaylistCursor(delta int) Model {
	if m.plInfo == nil || len(m.plInfo.Entries) == 0 {
		return m
	}
	m.plCursor = max(0, min(m.plCursor+delta, len(m.plInfo.Entries)-1))
	m.ensurePlaylistCursorVisible()
	return m
}

func (m *Model) ensurePlaylistCursorVisible() {
	height := m.playlistViewportHeight()
	if height <= 0 {
		m.plTop = 0
		return
	}

	if m.plCursor < m.plTop {
		m.plTop = m.plCursor
	}
	if m.plCursor >= m.plTop+height {
		m.plTop = m.plCursor - height + 1
	}

	maxTop := max(0, len(m.playlistEntries())-height)
	m.plTop = max(0, min(m.plTop, maxTop))
}

func (m Model) playlistEntries() []app.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}
	return m.plInfo.Entries
}

func (m Model) playlistViewportHeight() int {
	lines := min(15, m.height-16)
	return max(3, lines)
}
