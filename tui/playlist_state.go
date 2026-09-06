package tui

import (
	"volvid/internal/core"

	tea "charm.land/bubbletea/v2"
)

func (m Model) selectedPlaylistCount() int {
	return len(m.plSelected)
}
func (m *Model) clearPlaylistSelection() {
	clear(m.plSelected)
	if m.plSelected == nil {
		m.plSelected = map[int]bool{}
	}
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
	if len(m.plSelected) == total {
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
func (m Model) selectedPlaylistEntries() []core.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}

	selected := make([]core.PlaylistEntry, 0, len(m.plSelected))
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
func (m Model) playlistEntries() []core.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}
	return m.plInfo.Entries
}
func (m Model) playlistViewportHeight() int {
	lines := min(14, m.height-18)
	return max(4, lines)
}
