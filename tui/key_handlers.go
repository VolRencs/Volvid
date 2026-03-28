package tui

import (
	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if k == "ctrl+c" {
		return m, tea.Quit
	}

	if k == "tab" {
		m.locale = app.NextLocale(m.locale)
		app.SyncLoc(m.locale)
		_ = app.SaveLocale(m.locale)
		m.syncLocalizedInputs()
		m = m.syncMenu()
		return m, nil
	}

	if k == "ctrl+u" && !m.uiBusy() {
		return m.startDepUpdate()
	}

	if k == "esc" {
		switch m.screen {
		case scrSearchInput, scrSearchResults:
			return m.exitSearch()
		}
	}

	if m.isMenuScreen() {
		switch k {
		case "up", "k":
			m.menu.Move(-1)
			return m, nil
		case "down", "j":
			m.menu.Move(1)
			return m, nil
		case "enter":
			return m.activateMenu()
		default:
			return m, nil
		}
	}

	if m.screen == scrPlaylist {
		return m.handlePlaylistKey(msg)
	}

	switch m.screen {
	case scrUpdateDone:
		return m, tea.Quit

	case scrDepUpdate:
		if (m.depUpdateDone || m.depErr != "") && (k == "enter" || k == "esc" || k == "q") {
			m.screen = m.prevScreen
			if m.screen == scrURL {
				return m, m.urlInput.Focus()
			}
			return m, nil
		}
		return m, nil

	case scrURL:
		return m.handleURLKey(msg)

	case scrSearchInput:
		return m.handleSearchInputKey(msg)

	case scrFragmentInput:
		return m.handleFragmentInputKey(msg)
	}

	return m, nil
}

func (m Model) isMenuScreen() bool {
	switch m.screen {
	case scrUpdateReady, scrFFmpegAsk, scrPlaylistAsk, scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrSearchResults, scrFragmentChoice:
		return true
	}
	return false
}
