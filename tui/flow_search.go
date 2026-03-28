package tui

import (
	"strings"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleSearchResultsMsg(msg msgSearchResults) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed + ": " + msg.err.Error()
		return m, m.searchInput.Focus()
	}
	if len(msg.results) == 0 {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchNoResults
		return m, m.searchInput.Focus()
	}
	m.searchResults = msg.results
	m.searchErr = ""
	m.screen = scrSearchResults
	m = m.syncMenu()
	return m, nil
}

func (m Model) openSearchInput() (tea.Model, tea.Cmd) {
	m.screen = scrSearchInput
	m.searchErr = ""
	m.searchResults = nil
	if strings.TrimSpace(m.searchInput.Value()) == "" {
		m.searchInput.SetValue("")
	}
	return m, m.searchInput.Focus()
}

func (m Model) submitSearchInput() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		m.searchErr = m.u().SearchErrEmpty
		return m, nil
	}

	m.searchQuery = query
	m.searchErr = ""
	m.searchResults = nil
	m.screen = scrSearchFetch
	return m, searchYouTubeCmd(query)
}

func (m Model) activateSearchResult(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.searchResults) {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed
		return m, m.searchInput.Focus()
	}
	result := m.searchResults[idx]
	if strings.TrimSpace(result.URL) == "" {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed
		return m, m.searchInput.Focus()
	}

	target, err := app.ParseTarget(result.URL)
	if err != nil {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed + ": " + err.Error()
		return m, m.searchInput.Focus()
	}
	return m.startTargetFlow(result.URL, target)
}

func (m Model) exitSearch() (tea.Model, tea.Cmd) {
	m.screen = scrURL
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.Blur()
	return m, m.urlInput.Focus()
}
