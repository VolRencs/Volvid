package tui

import (
	"volvid/internal/core"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if k == "tab" {
		m.locale = core.NextLocale(m.locale)
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

	if m.screen == scrAudioTrack {
		return m.handleAudioTrackKey(msg)
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
	var model tea.Model
	var cmd tea.Cmd
	switch m.screen {
	case scrSearchInput, scrSearchResults, scrSearchFetch:
		model, cmd = m.exitSearch()
	case scrPlaylistAsk, scrPlaylist, scrFragmentChoice, scrMode:
		model, cmd = m.exitToURL()
	case scrPlaylistFetch, scrFragmentProbe:
		m = m.cancelOps()
		model, cmd = m.exitToURL()
	case scrQualityFetch:
		m = m.cancelOps()
		model, cmd = m.startModeSelectionWithNotice("")
	case scrTracksFetch:
		m = m.cancelOps()
		m.screen = scrVideoOutput
		model, cmd = m.syncMenu(), nil
	case scrAudio, scrQuality:
		model, cmd = m.startModeSelectionWithNotice("")
	case scrVideoOutput:
		model, cmd = m.gotoQualitySelection()
	case scrAudioTrack:
		m.screen = scrVideoOutput
		model, cmd = m.syncMenu(), nil
	case scrSubtitles:
		m.screen = scrVideoOutput
		if m.audioOffered {
			m.screen = scrAudioTrack
		}
		model, cmd = m.syncMenu(), nil
	case scrWorkers:
		model, cmd = m.gotoWorkersBack()
	case scrDownload:
		model, cmd = m.cancelDownload()
	case scrSummary:
		model, cmd = m.resetForNext()
	case scrDepUpdate:
		model, cmd = m.returnFromDependencyScreen()
	default:
		return m, nil, false
	}
	return model, cmd, true
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
	case "ctrl+o", "ctrl+щ":
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
