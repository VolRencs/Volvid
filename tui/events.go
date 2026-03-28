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
		m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
		if m.dlElapsed < 0 {
			m.dlElapsed = 0
		}
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
		if msg.err != nil {
			m.depErr = msg.err.Error()
			return m, nil
		}
		if msg.isUpdate {
			if app.IsWindows {
				return m, tea.Quit
			}
			m.screen = scrUpdateDone
			return m, nil
		}
		if m.screen == scrDepUpdate {
			deps := app.DetectDeps()
			m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
			m.depUpdateDone = true
			return m, nil
		}
		return m.gotoURL()

	case msgPlaylistFetched:
		return m.handlePlaylistFetched(msg)

	case msgSearchResults:
		return m.handleSearchResultsMsg(msg)

	case msgQualityScanned:
		if len(msg.choices) == 0 {
			m.qualityChoices = app.DefaultQualityChoices()
		} else {
			m.qualityChoices = msg.choices
		}
		m.screen = scrQuality
		m = m.syncMenu()
		return m, nil

	case msgDlUpdate:
		return m.handleDlUpdate(msg.update)
	}

	return m.routeFocusedInputMessage(msg)
}
