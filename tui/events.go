package tui

import (
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ensurePlaylistCursorVisible()
		return m, nil

	case tea.PasteMsg:
		return m.handleTerminalPaste(msg.Content)

	case tea.ClipboardMsg:
		return m.handleTerminalPaste(msg.Content)

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTickCmd()

	case timerTickMsg:
		if !m.timerActive {
			return m, nil
		}
		m.updateElapsed()
		return m, timerTickCmd()

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo = msg.info
		m.screen = scrUpdateReady
		m = m.syncMenu()
		return m, nil

	case msgDepProgress:
		m.depProgress = msg.progress
		return m, streamFileProgressCmd(m.depCh, m.screen == scrUpdateDl)

	case msgDepDone:
		return m.handleDepDone(msg)

	case msgDepsRefreshed:
		if msg.token != m.depRefreshToken {
			return m, nil
		}
		m.depRefreshing = false
		m = m.withDeps(msg.deps)
		if m.screen == scrDepUpdate {
			m = m.syncMenu()
		}
		return m, nil

	case msgPlaylistFetched:
		return m.handlePlaylistFetched(msg)

	case msgSearchResults:
		return m.handleSearchResultsMsg(msg)

	case msgQualityScanned:
		return m.handleQualityScanned(msg)

	case msgFragmentDuration:
		return m.handleFragmentDurationMsg(msg)

	case msgDlUpdate:
		return m.handleDlUpdate(msg.update)
	}

	return m.routeFocusedInputMessage(msg)
}

func (m *Model) updateElapsed() {
	m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
	if m.dlElapsed < 0 {
		m.dlElapsed = 0
	}
}

func (m Model) handleDepDone(msg msgDepDone) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.depErr = msg.err.Error()
		if m.screen == scrDepDl {
			m.screen = scrDepUpdate
			m = m.syncMenu()
		} else if m.screen == scrUpdateDl {
			m.screen = scrUpdateReady
			m = m.syncMenu()
		}
		return m, nil
	}
	if msg.isUpdate {
		if app.IsWindows {
			return m, tea.Quit
		}
		m.screen = scrUpdateDone
		return m, nil
	}

	m.depErr = ""
	m.depUpdateDone = true
	m.screen = scrDepUpdate
	m = m.syncMenu()
	return m.startDepsRefresh()
}

func (m Model) handleQualityScanned(msg msgQualityScanned) (tea.Model, tea.Cmd) {
	m.qualityChoices = msg.choices
	if len(m.qualityChoices) == 0 {
		m.qualityChoices = app.DefaultQualityChoices()
	}
	m.screen = scrQuality
	m = m.syncMenu()
	return m, nil
}
