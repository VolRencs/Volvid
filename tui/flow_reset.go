package tui

import (
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

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

func (m *Model) resetProfileState() {
	m.forceSingle = false
	m.numWorkers = 1
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = ""
	m.dlEntries = nil
	m.qualityChoices = nil
	m.audioProfiles = nil
}

func (m *Model) resetFragmentState() {
	m.mediaDuration = 0
	m.fragment = nil
	m.fragmentErr = ""
	m.fragmentIn.SetValue("")
	m.fragmentIn.Blur()
}

func (m *Model) resetDownloadProgressState() {
	m.slots = nil
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = 0
	m.singleOK = false
	m.dlStartedAt = time.Time{}
	m.dlElapsed = 0
	m.timerActive = false
}

func (m *Model) resetDownloadState() {
	m.resetProfileState()
	m.resetFragmentState()
	m.resetDownloadProgressState()
}

func (m *Model) resetTargetFlowState() {
	m.resetPlaylistState()
	m.resetProfileState()
	m.resetFragmentState()
	m.searchResults = nil
	m.searchErr = ""
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
