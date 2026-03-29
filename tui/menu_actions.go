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
			info := m.updateInfo
			return m.startDependencyDownload(scrUpdateDl, "", true, func(ch chan<- app.FileProgress) error {
				return app.ApplyUpdateFor(m.locale, info, ch)
			})
		}
		return m.gotoChecks()

	case scrDepUpdate:
		actions := m.depActions()
		if idx < 0 || idx >= len(actions) {
			return m, nil
		}
		action := actions[idx]
		switch action.Kind {
		case depActionInstall:
			return m.startDependencyDownload(scrDepDl, action.Key, false, func(ch chan<- app.FileProgress) error {
				return app.InstallDependencyFor(action.Key, m.locale, ch)
			})
		case depActionRefresh:
			return m.startDepsRefresh()
		case depActionContinue:
			return m.gotoURL()
		case depActionBack:
			return m.returnFromDependencyScreen()
		case depActionExit:
			return m, tea.Quit
		}

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
		return m.activateFragmentChoice(idx)

	case scrMode:
		switch idx {
		case 1:
			m.mode = app.ModeAudio
		case 2:
			m.mode = app.ModeThumbnail
		default:
			m.mode = app.ModeVideo
		}
		m.profile = app.OutputProfile{}
		m.qualityChoices = nil
		m.audioProfiles = nil
		m.flowErr = ""

		switch m.mode {
		case app.ModeThumbnail:
			m.profile = app.ThumbnailOutputProfile(m.locale)
			return m.continueAfterProfileSelection()
		case app.ModeAudio:
			m.audioProfiles = app.AudioOutputProfiles(m.locale)
			m.screen = scrAudio
			m = m.syncMenu()
			return m, nil
		default:
			return m.startQualityScan()
		}

	case scrAudio:
		if idx < 0 || idx >= len(m.audioProfiles) {
			return m, nil
		}
		m.profile = m.audioProfiles[idx]
		m.flowErr = ""
		return m.continueAfterProfileSelection()

	case scrQuality:
		if idx < 0 || idx >= len(m.qualityChoices) {
			return m, nil
		}
		m.profile = m.qualityChoices[idx].Profile(m.locale)
		m.flowErr = ""
		return m.continueAfterProfileSelection()

	case scrWorkers:
		m.numWorkers = idx + 1
		return m.startDownload()
	}

	return m, nil
}

func (m Model) continueAfterProfileSelection() (tea.Model, tea.Cmd) {
	if len(m.dlEntries) > 1 {
		m.screen = scrWorkers
		m = m.syncMenu()
		return m, nil
	}
	return m.startDownload()
}

func (m Model) activateFragmentChoice(idx int) (tea.Model, tea.Cmd) {
	options := m.fragmentChoiceOptions()
	if idx < 0 || idx >= len(options) {
		return m, nil
	}

	switch {
	case idx == 0:
		m.fragment = nil
		return m.startModeSelection()
	case m.canUseURLStartFragment() && idx == 1:
		fragment := app.DownloadFragment{StartAt: m.target.URLStartAt}
		if err := app.ValidateFragmentDuration(fragment, m.mediaDuration); err != nil {
			m.flowErr = app.FragmentURLStartOutOfBoundsText(m.locale, m.mediaDuration)
			m = m.syncMenu()
			return m, nil
		}
		m.fragment = &fragment
		return m.startModeSelection()
	default:
		m.fragmentErr = ""
		m.screen = scrFragmentInput
		return m, m.fragmentIn.Focus()
	}
}
