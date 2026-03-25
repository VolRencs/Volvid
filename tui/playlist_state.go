package tui

import tea "charm.land/bubbletea/v2"

func (m Model) selectedPlaylistCount() int {
	return len(m.plSelected)
}

func (m *Model) clearPlaylistSelection() {
	m.plSelected = map[int]bool{}
}

func (m *Model) applyPlaylistSelectionIndices(indices []int) {
	if len(indices) == 0 {
		m.clearPlaylistSelection()
		return
	}

	selected := make(map[int]bool, len(indices))
	for _, idx := range indices {
		selected[idx] = true
	}
	m.plSelected = selected
}

func (m *Model) toggleCurrentPlaylistEntry() {
	if m.plInfo == nil || m.plCursor < 0 || m.plCursor >= len(m.plInfo.Entries) {
		return
	}

	idx := m.plInfo.Entries[m.plCursor].Index
	if m.plSelected[idx] {
		delete(m.plSelected, idx)
		return
	}
	m.plSelected[idx] = true
}

func (m *Model) toggleAllPlaylistEntries() {
	total := len(m.playlistEntries())
	if total == 0 {
		m.clearPlaylistSelection()
		return
	}
	if m.selectedPlaylistCount() == total {
		m.clearPlaylistSelection()
		return
	}

	selected := make(map[int]bool, total)
	for _, entry := range m.playlistEntries() {
		selected[entry.Index] = true
	}
	m.plSelected = selected
}

func (m *Model) openPlaylistInput() tea.Cmd {
	m.plInputMode = true
	m.plInputErr = ""
	m.plInput.SetValue("")
	return m.plInput.Focus()
}

func (m *Model) closePlaylistInput() {
	m.plInput.Blur()
	m.plInputMode = false
	m.plInputErr = ""
}
