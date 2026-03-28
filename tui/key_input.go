package tui

import (
	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleURLKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		return m.openSearchInput()
	case "enter":
		return m.submitURLInput()
	default:
		m.urlErr = ""
		return m, m.urlInput.Update(msg)
	}
}

func (m Model) handleSearchInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		return m.submitSearchInput()
	}
	m.searchErr = ""
	return m, m.searchInput.Update(msg)
}

func (m Model) handleFragmentInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fragmentErr = ""
		m.fragmentIn.Blur()
		m.screen = scrFragmentChoice
		m = m.syncMenu()
		return m, nil
	case "enter":
		fragment, err := app.ParseFragmentRange(m.fragmentIn.Value())
		if err != nil {
			m.fragmentErr = m.u().FragmentInputBadFormat
			return m, nil
		}
		if !fragment.IsValid() {
			m.fragmentErr = m.u().FragmentInputBadRange
			return m, nil
		}
		m.fragmentErr = ""
		m.fragment = &fragment
		m.fragmentIn.Blur()
		return m.startModeSelection()
	default:
		m.fragmentErr = ""
		return m, m.fragmentIn.Update(msg)
	}
}
