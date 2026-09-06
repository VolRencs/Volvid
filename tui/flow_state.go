package tui

import (
	"time"
	"volvid/internal/core"

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
	m.mode = core.ModeVideo
	m.profile = m.defaultVideoProfile()
	m.flowErr = ""
	m.dlEntries = nil
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.audioProfiles = nil
}
func (m *Model) defaultVideoProfile() core.OutputProfile {
	return m.api.DefaultVideoProfile(m.locale)
}
func (m *Model) resetFragmentState() {
	m.mediaDuration = 0
	m.fragment = nil
	m.fragmentErr = ""
	m.fragmentIn.SetValue("")
	m.fragmentIn.Blur()
}
func (m *Model) resetDownloadProgressState() {
	if m.dlCancel != nil {
		m.dlCancel()
		m.dlCancel = nil
	}
	m.slots = nil
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = 0
	m.singleOK = false
	m.downloadErr = ""
	m.dlStartedAt = time.Time{}
	m.dlElapsed = 0
	m.timerActive = false
	m.dlCh = nil
	m.dlCancelled = false
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
	m.target = core.ParsedTarget{}
	m.resetSearchState()
	m.resetPlaylistState()
	m.resetDownloadState()
	return m, m.urlInput.Focus()
}
func (m Model) restoreActiveScreen() (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrVideoOutput, scrSearchResults, scrFragmentChoice, scrPlaylistAsk:
		m = m.syncMenu()
		return m, nil
	case scrURL:
		return m, m.urlInput.Focus()
	case scrSearchInput:
		return m, m.searchInput.Focus()
	case scrFragmentInput:
		return m, m.fragmentIn.Focus()
	case scrPlaylist:
		if m.plInputMode {
			return m, m.plInput.Focus()
		}
	}
	return m, nil
}
func (m Model) exitToURL() (tea.Model, tea.Cmd) {
	m = m.cancelOps()
	m.resetTargetFlowState()
	m.screen = scrURL
	return m, m.urlInput.Focus()
}
