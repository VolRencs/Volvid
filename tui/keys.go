package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if !m.isMenuScreen() || !isDigitKey(k) {
		m.menuDigits = ""
	}

	if k == "tab" {
		m.locale = m.api.NextLocale(m.locale)
		_ = m.api.SaveLocale(m.locale)
		m.syncLocalizedInputs()
		m = m.syncMenu()
		return m, nil
	}

	if k == "ctrl+u" && m.canOpenDependencyScreen() {
		return m.startDepUpdate()
	}

	if isPickFolderKey(msg) && m.canPickDownloadsFolder() {
		if m.api.DownloadsDirLocked() {
			m.urlErr = m.u().DownloadsDirLocked
			return m, nil
		}
		return m.startPickDownloadsDir()
	}

	if isOpenFolderKey(msg) && m.canOpenDownloadsFolder() &&
		(m.screen != scrURL || msg.String() == "O" || msg.String() == "Щ") {
		return m.startOpenDownloadsDir()
	}

	if k == "esc" {
		if model, cmd, handled := m.handleEscape(msg); handled {
			return model, cmd
		}
	}

	if m.isMenuScreen() {
		switch k {
		case "up":
			m.menu.Move(-1)
			return m, nil
		case "down":
			m.menu.Move(1)
			return m, nil
		case "enter":
			return m.activateMenu()
		default:
			if isDigitKey(k) {
				return m.handleMenuDigit(k)
			}
			return m, nil
		}
	}

	if m.screen == scrPlaylist {
		return m.handlePlaylistKey(msg)
	}

	if m.screen == scrSubtitles {
		return m.handleSubtitlesKey(msg)
	}

	switch m.screen {
	case scrUpdateDone:
		return m, tea.Quit
	case scrURL:
		return m.handleURLKey(msg)
	case scrSearchInput:
		return m.handleSearchInputKey(msg)
	case scrFragmentInput:
		return m.handleFragmentInputKey(msg)
	}

	return m, nil
}
func (m Model) handleEscape(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// FragmentInput delegates to its input handler (clears error, blurs).
	if m.screen == scrFragmentInput {
		model, cmd := m.handleFragmentInputKey(msg)
		return model, cmd, true
	}
	// UpdateDl/DepDl: cancel in-flight progress download.
	if m.screen == scrUpdateDl || m.screen == scrDepDl {
		if m.depCancel != nil {
			m.depCancel()
		}
		return m, nil, true
	}
	if handler, ok := escHandlers[m.screen]; ok {
		model, cmd := handler(m)
		return model, cmd, true
	}
	return m, nil, false
}

var escHandlers = map[screen]func(Model) (tea.Model, tea.Cmd){
	scrSearchInput:    func(m Model) (tea.Model, tea.Cmd) { return m.exitSearch() },
	scrSearchResults:  func(m Model) (tea.Model, tea.Cmd) { return m.exitSearch() },
	scrSearchFetch:    func(m Model) (tea.Model, tea.Cmd) { return m.exitSearch() },
	scrPlaylistAsk:    func(m Model) (tea.Model, tea.Cmd) { return m.exitToURL() },
	scrPlaylist:       func(m Model) (tea.Model, tea.Cmd) { return m.exitToURL() },
	scrFragmentChoice: func(m Model) (tea.Model, tea.Cmd) { return m.exitToURL() },
	scrMode:           func(m Model) (tea.Model, tea.Cmd) { return m.exitToURL() },
	scrPlaylistFetch: func(m Model) (tea.Model, tea.Cmd) {
		m = m.cancelOps()
		return m.exitToURL()
	},
	scrFragmentProbe: func(m Model) (tea.Model, tea.Cmd) {
		m = m.cancelOps()
		return m.exitToURL()
	},
	scrQualityFetch: func(m Model) (tea.Model, tea.Cmd) {
		m = m.cancelOps()
		return m.startModeSelectionWithNotice("")
	},
	scrSubsFetch: func(m Model) (tea.Model, tea.Cmd) {
		m = m.cancelOps()
		m.screen = scrVideoOutput
		m = m.syncMenu()
		return m, nil
	},
	scrAudio:       func(m Model) (tea.Model, tea.Cmd) { return m.startModeSelectionWithNotice("") },
	scrQuality:     func(m Model) (tea.Model, tea.Cmd) { return m.startModeSelectionWithNotice("") },
	scrVideoOutput: func(m Model) (tea.Model, tea.Cmd) { return m.gotoQualitySelection() },
	scrSubtitles: func(m Model) (tea.Model, tea.Cmd) {
		m.screen = scrVideoOutput
		m = m.syncMenu()
		return m, nil
	},
	scrWorkers:   func(m Model) (tea.Model, tea.Cmd) { return m.gotoWorkersBack() },
	scrDownload:  func(m Model) (tea.Model, tea.Cmd) { return m.cancelDownload() },
	scrSummary:   func(m Model) (tea.Model, tea.Cmd) { return m.resetForNext() },
	scrDepUpdate: func(m Model) (tea.Model, tea.Cmd) { return m.returnFromDependencyScreen() },
}

func isOpenFolderKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "o", "O", "щ", "Щ":
		return true
	default:
		return false
	}
}
func isPickFolderKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "ctrl+o", "ctrl+O", "ctrl+щ", "ctrl+Щ":
		return true
	default:
		return false
	}
}
func isDigitKey(k string) bool {
	if len(k) != 1 {
		return false
	}
	c := k[0]
	return (c >= '1' && c <= '9') || c == '0'
}
