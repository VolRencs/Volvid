package tui

import (
	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) activateMenu() (tea.Model, tea.Cmd) {
	idx := m.menu.Index()

	switch m.screen {
	case scrUpdateReady:
		if idx == 0 {
			m.screen = scrUpdateDl
			m.depProgress = app.FileProgress{}
			m.depErr = ""

			var cmd tea.Cmd
			info := m.updateInfo
			m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
				return app.ApplyUpdateFor(m.locale, info, ch)
			}, true)
			return m, cmd
		}
		return m.gotoChecks()

	case scrFFmpegAsk:
		if idx == 0 {
			m.depLabel = "ffmpeg"
			m.depProgress = app.FileProgress{}
			m.depErr = ""
			m.screen = scrDepDl

			var cmd tea.Cmd
			m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
				return app.InstallFFmpegFor(m.locale, ch)
			}, false)
			return m, cmd
		}
		return m.gotoURL()

	case scrPlaylistAsk:
		if idx == 0 {
			m.forceSingle = true
			return m.startFragmentFlow()
		}
		m.screen = scrPlaylistFetch
		return m, fetchPlaylistCmd(m.url, m.locale)

	case scrSummary:
		if idx == 0 {
			return m.resetForNext()
		}
		return m, tea.Quit

	case scrSearchResults:
		return m.activateSearchResult(idx)

	case scrFragmentChoice:
		options := m.fragmentChoiceOptions()
		if idx < 0 || idx >= len(options) {
			return m, nil
		}
		switch {
		case idx == 0:
			m.fragment = nil
			return m.startModeSelection()
		case m.target.HasURLStart && m.target.URLStartAt > 0 && idx == 1:
			fragment := app.DownloadFragment{StartAt: m.target.URLStartAt}
			m.fragment = &fragment
			return m.startModeSelection()
		default:
			m.fragmentErr = ""
			m.screen = scrFragmentInput
			return m, m.fragmentIn.Focus()
		}

	case scrMode:
		m.mode = m.modeAt(idx)
		m.profile = app.OutputProfile{}
		m.qualityChoices = nil
		m.audioProfiles = nil
		m.flowErr = ""

		switch m.mode {
		case app.ModeThumbnail:
			m.profile = app.ThumbnailOutputProfile(m.locale)
			if m.shouldAskWorkers() {
				m.screen = scrWorkers
				m = m.syncMenu()
				return m, nil
			}
			return m.startDownload()
		case app.ModeAudio:
			m.audioProfiles = app.AudioOutputProfiles(m.locale)
			m.screen = scrAudio
			m = m.syncMenu()
			return m, nil
		default:
			return m.startQualityScan()
		}

	case scrAudio:
		m.profile = m.audioProfileAt(idx)
		m.flowErr = ""
		if m.shouldAskWorkers() {
			m.screen = scrWorkers
			m = m.syncMenu()
			return m, nil
		}
		return m.startDownload()

	case scrQuality:
		m.profile = m.qualityProfileAt(idx)
		m.flowErr = ""
		if m.shouldAskWorkers() {
			m.screen = scrWorkers
			m = m.syncMenu()
			return m, nil
		}
		return m.startDownload()

	case scrWorkers:
		m.numWorkers = idx + 1
		return m.startDownload()
	}

	return m, nil
}
