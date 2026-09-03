package tui

import (
	"context"
	"errors"
	"time"

	app "volvid/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.syncLayout()
		m.ensurePlaylistCursorVisible()
		return m, nil

	case tea.PasteMsg:
		return m.handleTerminalPaste(msg.Content)

	case tea.ClipboardMsg:
		return m.handleTerminalPaste(msg.Content)

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		if !m.spinnerVisible() {
			return m, nil
		}
		return m, spinnerTickCmd()

	case timerTickMsg:
		if !m.timerActive {
			return m, nil
		}
		m.updateElapsed()
		return m, timerTickCmd()

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case menuDigitTickMsg:
		return m.activatePendingDigits()

	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo = msg.info
		m.screen = scrUpdateReady
		m = m.syncMenu()
		return m, nil

	case msgDepProgress:
		if msg.gen != m.depGen {
			return m, nil
		}
		m.depProgress = msg.progress
		return m, streamFileProgressCmd(m.depCh, m.screen == scrUpdateDl, m.depGen)

	case msgDepDone:
		return m.handleDepDone(msg)

	case msgDepsRefreshed:
		if msg.token != m.depRefreshToken {
			return m, nil
		}
		m.depRefreshing = false
		m.deps = msg.deps
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
		return m.handleDlUpdate(msg.update, msg.gen)

	case msgOpenDownloadsDirDone:
		if msg.err == nil {
			return m, nil
		}
		if m.screen == scrURL {
			m.urlErr = msg.err.Error()
			return m, nil
		}
		if m.downloadErr == "" {
			m.downloadErr = msg.err.Error()
		}
		return m, nil

	case msgPickDownloadsDirDone:
		return m.handlePickDownloadsDirDone(msg)
	}

	return m.routeFocusedInputMessage(msg)
}

func (m *Model) updateElapsed() {
	m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
}

func (m Model) handlePickDownloadsDirDone(msg msgPickDownloadsDirDone) (tea.Model, tea.Cmd) {
	switch {
	case msg.err == nil && msg.path == "":
		return m, nil
	case app.IsFolderPickerCancelled(msg.err):
		return m, nil
	case msg.err != nil:
		m.urlErr = m.u().PickDownloadsFailed + ": " + msg.err.Error()
		return m, nil
	}

	if err := app.SetDownloadsDir(m.env, msg.path); err != nil {
		if err == app.ErrDownloadsDirLocked {
			m.urlErr = m.u().DownloadsDirLocked
			return m, nil
		}
		m.urlErr = err.Error()
		return m, nil
	}
	return m, nil
}

func (m Model) handleDepDone(msg msgDepDone) (tea.Model, tea.Cmd) {
	if msg.gen != m.depGen {
		return m, nil
	}
	m.depCancel = nil
	m.depCh = nil
	if errors.Is(msg.err, context.Canceled) {
		m.depErr = ""
		return m.navigateDepBack(), nil
	}
	if msg.err != nil {
		m.depErr = msg.err.Error()
		return m.navigateDepBack(), nil
	}
	if msg.isUpdate {
		if m.env.IsWindows {
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

func (m Model) navigateDepBack() Model {
	switch m.screen {
	case scrDepDl:
		m.screen = scrDepUpdate
		return m.syncMenu()
	case scrUpdateDl:
		m.screen = scrUpdateReady
		return m.syncMenu()
	default:
		return m
	}
}

func (m Model) handleQualityScanned(msg msgQualityScanned) (tea.Model, tea.Cmd) {
	if msg.gen != m.opGen {
		return m, nil
	}
	m = m.clearOpCancel()
	m.qualityChoices = msg.choices
	if len(m.qualityChoices) == 0 {
		m.qualityChoices = app.DefaultQualityChoices()
	}
	if msg.err != nil {
		m.flowErr = msg.err.Error()
		m.screen = scrMode
		m = m.syncMenu()
		return m, nil
	}
	m.screen = scrQuality
	m = m.syncMenu()
	return m, nil
}

func (m Model) handleSearchResultsMsg(msg msgSearchResults) (tea.Model, tea.Cmd) {
	if msg.gen != m.opGen {
		return m, nil
	}
	m = m.clearOpCancel()
	if msg.err != nil {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed + ": " + msg.err.Error()
		return m, m.searchInput.Focus()
	}
	if len(msg.results) == 0 {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchNoResults
		return m, m.searchInput.Focus()
	}
	m.searchResults = msg.results
	m.searchErr = ""
	m.screen = scrSearchResults
	m = m.syncMenu()
	return m, nil
}

func (m Model) handlePlaylistFetched(msg msgPlaylistFetched) (tea.Model, tea.Cmd) {
	if msg.gen != m.opGen {
		return m, nil
	}
	m = m.clearOpCancel()
	if msg.err != nil || msg.info == nil {
		m.forceSingle = true
		return m.startModeSelectionWithNotice("")
	}

	m.plInfo = msg.info
	m.plCursor = 0
	m.plTop = 0
	m.plSelected = map[int]bool{}
	m.plInputErr = ""
	m.screen = scrPlaylist
	return m, nil
}

func (m Model) handleFragmentDurationMsg(msg msgFragmentDuration) (tea.Model, tea.Cmd) {
	if msg.gen != m.opGen {
		return m, nil
	}
	m = m.clearOpCancel()
	if msg.err != nil || msg.duration <= 0 {
		m.mediaDuration = 0
		m.fragment = nil
		return m.startModeSelectionWithNotice(app.FragmentUnavailableText(m.locale))
	}

	m.mediaDuration = msg.duration
	m.screen = scrFragmentChoice
	m = m.syncMenu()
	return m, nil
}

func (m Model) handleDlUpdate(u app.DlUpdate, gen int) (tea.Model, tea.Cmd) {
	if gen != m.dlGen {
		return m, nil
	}
	if u.Type == app.EvClosed {
		return m, nil
	}

	if m.dlCancelled {
		return m, nil
	}

	if u.Slot >= 0 && u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case app.EvStart:
			*s = slotState{title: trunc(u.Text, m.slotTitleWidth())}
		case app.EvDest:
			s.title = trunc(u.Text, m.slotTitleWidth())
		case app.EvProgress:
			s.pct = u.Pct
			s.doneB = u.DoneB
			s.totalB = u.TotalB
			s.speed = u.Speed
			s.proc = false
		case app.EvProc, app.EvFallback:
			s.proc = true
			s.label = u.Text
		case app.EvReset:
			*s = slotState{}
		case app.EvDone:
			s.done = u.OK
			s.failed = !u.OK
			s.proc = false
			s.label = trunc(u.ErrText, max(20, m.cardBodyWidth()-10))
			s.pct = 100
		}
	}

	if u.Type != app.EvDone {
		return m, listenDownloadCmd(m.dlCh, m.dlGen)
	}

	if u.OK {
		m.dlDone++
	} else {
		m.dlFailed++
		if m.downloadErr == "" {
			m.downloadErr = u.ErrText
		}
	}

	if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
		if m.dlCancel != nil {
			m.dlCancel()
			m.dlCancel = nil
		}
		if m.dlTotal == 0 {
			m.singleOK = u.OK
		}
		label := m.downloadLabel()
		if m.dlTotal > 0 {
			label += app.PlaylistSuffix(m.locale, m.dlTotal)
		}
		m.session.Record(label, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
		m.screen = scrSummary
		m.timerActive = false
		m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
		m = m.syncMenu()
		return m, nil
	}

	return m, listenDownloadCmd(m.dlCh, m.dlGen)
}
