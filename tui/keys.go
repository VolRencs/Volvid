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
	switch m.screen {
	case scrSearchInput, scrSearchResults, scrSearchFetch:
		model, cmd := m.exitSearch()
		return model, cmd, true
	case scrPlaylistAsk, scrPlaylist:
		model, cmd := m.exitToURL()
		return model, cmd, true
	case scrPlaylistFetch:
		m = m.cancelOps()
		model, cmd := m.exitToURL()
		return model, cmd, true
	case scrFragmentChoice:
		model, cmd := m.exitToURL()
		return model, cmd, true
	case scrFragmentProbe:
		m = m.cancelOps()
		model, cmd := m.exitToURL()
		return model, cmd, true
	case scrQualityFetch:
		m = m.cancelOps()
		model, cmd := m.startModeSelectionWithNotice("")
		return model, cmd, true
	case scrMode:
		model, cmd := m.exitToURL()
		return model, cmd, true
	case scrFragmentInput:
		model, cmd := m.handleFragmentInputKey(msg)
		return model, cmd, true
	case scrAudio:
		model, cmd := m.startModeSelectionWithNotice("")
		return model, cmd, true
	case scrQuality:
		model, cmd := m.startModeSelectionWithNotice("")
		return model, cmd, true
	case scrVideoOutput:
		model, cmd := m.gotoQualitySelection()
		return model, cmd, true
	case scrWorkers:
		model, cmd := m.gotoWorkersBack()
		return model, cmd, true
	case scrDownload:
		model, cmd := m.cancelDownload()
		return model, cmd, true
	case scrSummary:
		model, cmd := m.resetForNext()
		return model, cmd, true
	case scrDepUpdate:
		model, cmd := m.returnFromDependencyScreen()
		return model, cmd, true
	case scrUpdateDl, scrDepDl:
		if m.depCancel != nil {
			m.depCancel()
		}
		return m, nil, true
	default:
		return m, nil, false
	}
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
