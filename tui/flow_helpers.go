package tui

import (
	"strings"
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handlePlaylistFetched(msg msgPlaylistFetched) (tea.Model, tea.Cmd) {
	if msg.err != nil || msg.info == nil {
		m.forceSingle = true
		return m.startModeSelection()
	}
	m.plInfo = msg.info
	m.plCursor = 0
	m.plTop = 0
	m.plSelected = map[int]bool{}
	m.plInputErr = ""
	m.screen = scrPlaylist
	return m, nil
}

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

func (m Model) submitURLInput() (tea.Model, tea.Cmd) {
	rawURL := strings.TrimSpace(m.urlInput.Value())
	if rawURL == "" {
		m.urlErr = m.u().URLErrEmpty
		return m, nil
	}

	target, err := app.ParseTarget(rawURL)
	if err != nil {
		m.urlErr = m.u().URLErrBad
		return m, nil
	}

	m.urlErr = ""
	return m.startTargetFlow(rawURL, target)
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
	result := m.searchResultAt(idx)
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

func (m *Model) resetSearchState() {
	m.searchQuery = ""
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.SetValue("")
	m.searchInput.Blur()
}

func (m *Model) resetPlaylistState() {
	m.plInfo = nil
	m.plCursor = 0
	m.plTop = 0
	m.clearPlaylistSelection()
	m.plInput.SetValue("")
	m.closePlaylistInput()
}

func (m *Model) resetDownloadState() {
	m.forceSingle = false
	m.numWorkers = 1
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = ""
	m.dlEntries = nil
	m.qualityChoices = nil
	m.audioProfiles = nil
	m.slots = nil
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = 0
	m.singleOK = false
	m.dlStartedAt = time.Time{}
	m.dlElapsed = 0
	m.timerActive = false
}

func (m Model) resetForNext() (tea.Model, tea.Cmd) {
	m.screen = scrURL
	m.url = ""
	m.urlErr = ""
	m.urlInput.SetValue("")
	m.target = app.ParsedTarget{}
	m.resetSearchState()
	m.resetPlaylistState()
	m.resetDownloadState()
	return m, m.urlInput.Focus()
}

func (m Model) exitSearch() (tea.Model, tea.Cmd) {
	m.screen = scrURL
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.Blur()
	return m, m.urlInput.Focus()
}

func (m *Model) resetTargetFlowState() {
	m.plInfo = nil
	m.plCursor = 0
	m.plTop = 0
	m.clearPlaylistSelection()
	m.closePlaylistInput()
	m.flowErr = ""
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.dlEntries = nil
	m.qualityChoices = nil
	m.audioProfiles = nil
	m.forceSingle = false
	m.numWorkers = 1
	m.searchResults = nil
	m.searchErr = ""
}

func (m Model) startTargetFlow(rawURL string, target app.ParsedTarget) (tea.Model, tea.Cmd) {
	m.url = rawURL
	m.urlInput.SetValue(rawURL)
	m.target = target
	m.resetTargetFlowState()

	if target.IsPlaylist() {
		if target.Kind == app.TargetMixed {
			m.screen = scrPlaylistAsk
			m = m.syncMenu()
			return m, nil
		}
		m.screen = scrPlaylistFetch
		return m, fetchPlaylistCmd(rawURL, m.locale)
	}

	return m.startModeSelection()
}
