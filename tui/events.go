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
			m.screen = scrUpdateDone
			if app.IsWindows {
				return m, updateRestartCmd()
			}
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

	case msgUpdateRestart:
		return m, tea.Quit
	}

	switch {
	case m.screen == scrURL:
		if cmd := m.urlInput.Update(msg); cmd != nil {
			return m, cmd
		}
	case m.screen == scrSearchInput:
		if cmd := m.searchInput.Update(msg); cmd != nil {
			return m, cmd
		}
	case m.screen == scrPlaylist && m.plInputMode:
		if cmd := m.plInput.Update(msg); cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) gotoChecks() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps()
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if deps.YtdlpVer == "" {
		m.depLabel = "yt-dlp"
		m.depProgress = app.FileProgress{}
		m.depErr = ""
		m.screen = scrDepDl

		var cmd tea.Cmd
		m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
			return app.InstallYtDlpFor(m.locale, ch)
		}, false)
		return m, cmd
	}
	return m.gotoURLWithDeps(deps)
}

func (m Model) gotoURL() (tea.Model, tea.Cmd) {
	return m.gotoURLWithDeps(app.DetectDeps())
}

func (m Model) gotoURLWithDeps(deps app.CheckDepsResult) (tea.Model, tea.Cmd) {
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if app.IsWindows && deps.FFmpegMissing {
		m.screen = scrFFmpegAsk
		m = m.syncMenu()
		return m, nil
	}
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) startDepUpdate() (tea.Model, tea.Cmd) {
	m.prevScreen = m.screen
	m.depProgress = app.FileProgress{}
	m.depErr = ""
	m.depUpdateDone = false
	m.screen = scrDepUpdate

	var cmd tea.Cmd
	m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
		return app.InstallAllDepsFor(m.locale, ch)
	}, false)
	return m, cmd
}

func (m Model) handleDlUpdate(u app.DlUpdate) (tea.Model, tea.Cmd) {
	if u.Type == app.EvClosed {
		return m, nil
	}

	if u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case app.EvStart:
			*s = slotState{title: trunc(u.Text, 50)}
		case app.EvDest:
			s.title = trunc(u.Text, 50)
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
			s.pct = 100
		}
	}

	if u.Type != app.EvDone {
		return m, listenDownloadCmd(m.dlCh)
	}

	if u.OK {
		m.dlDone++
	} else {
		m.dlFailed++
	}

	if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
		if m.dlTotal == 0 {
			m.singleOK = u.OK
		}
		label := m.downloadLabel()
		if m.dlTotal > 0 {
			label += m.sessionPlaylistSuffix(m.dlTotal)
		}
		m.session.Record(label, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
		m.screen = scrSummary
		m.timerActive = false
		m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
		m = m.syncMenu()
		return m, nil
	}

	return m, listenDownloadCmd(m.dlCh)
}

func (m Model) downloadLabel() string {
	profile := m.currentProfile()
	if profile.Label != "" {
		return profile.Label
	}
	switch profile.Mode {
	case app.ModeAudio:
		return m.u().ModeAudio
	case app.ModeThumbnail:
		return m.u().ModeThumbnail
	default:
		return m.u().ModeVideo
	}
}

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
		if k == "?" {
			return m.openSearchInput()
		}
		if k != "enter" {
			m.urlErr = ""
			return m, m.urlInput.Update(msg)
		}
		return m.submitURLInput()

	case scrSearchInput:
		if k != "enter" {
			m.searchErr = ""
			return m, m.searchInput.Update(msg)
		}
		return m.submitSearchInput()
	}

	return m, nil
}

func (m Model) handleTerminalPaste(content string) (tea.Model, tea.Cmd) {
	switch {
	case m.screen == scrURL:
		m.urlErr = ""
		var cmds []tea.Cmd
		if !m.urlInput.Focused() {
			cmds = append(cmds, m.urlInput.Focus())
		}
		cmds = append(cmds, m.urlInput.insertRunes([]rune(content)))
		return m, tea.Batch(cmds...)

	case m.screen == scrSearchInput:
		m.searchErr = ""
		var cmds []tea.Cmd
		if !m.searchInput.Focused() {
			cmds = append(cmds, m.searchInput.Focus())
		}
		cmds = append(cmds, m.searchInput.insertRunes([]rune(content)))
		return m, tea.Batch(cmds...)

	case m.screen == scrPlaylist && m.plInputMode:
		m.plInputErr = ""
		var cmds []tea.Cmd
		if !m.plInput.Focused() {
			cmds = append(cmds, m.plInput.Focus())
		}
		cmds = append(cmds, m.plInput.insertRunes([]rune(content)))
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m Model) handlePlaylistKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.plInfo == nil {
		return m, nil
	}

	k := msg.String()
	if m.plInputMode {
		switch k {
		case "enter":
			indices, err := app.ParseSelectionFor(m.plInput.Value(), len(m.plInfo.Entries), m.locale)
			if err != nil {
				m.plInputErr = err.Error()
				return m, nil
			}

			m.plSelected = map[int]bool{}
			for _, idx := range indices {
				m.plSelected[idx] = true
			}
			m.plInput.Blur()
			m.plInputMode = false
			m.plInputErr = ""
			return m, nil

		case "esc":
			m.plInput.Blur()
			m.plInputMode = false
			m.plInputErr = ""
			return m, nil

		default:
			return m, m.plInput.Update(msg)
		}
	}

	total := len(m.plInfo.Entries)
	switch k {
	case "up", "k":
		m = m.stepPlaylistCursor(-1)
	case "down", "j":
		m = m.stepPlaylistCursor(1)
	case "space":
		idx := m.plInfo.Entries[m.plCursor].Index
		if m.plSelected[idx] {
			delete(m.plSelected, idx)
		} else {
			m.plSelected[idx] = true
		}
		m.plInputErr = ""
	case "a", "а":
		if len(m.plSelected) == total {
			m.plSelected = map[int]bool{}
		} else {
			m.plSelected = make(map[int]bool, total)
			for _, entry := range m.plInfo.Entries {
				m.plSelected[entry.Index] = true
			}
		}
		m.plInputErr = ""
	case "/":
		m.plInputMode = true
		m.plInputErr = ""
		m.plInput.SetValue("")
		return m, m.plInput.Focus()
	case "enter":
		m.dlEntries = m.selectedPlaylistEntries()
		if len(m.dlEntries) == 0 {
			m.plInputErr = m.u().ErrPickOne
			return m, nil
		}
		return m.startModeSelection()
	default:
		return m, nil
	}

	return m, nil
}

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
			return m.startModeSelection()
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

func (m Model) isMenuScreen() bool {
	switch m.screen {
	case scrUpdateReady, scrFFmpegAsk, scrPlaylistAsk, scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrSearchResults:
		return true
	}
	return false
}

func (m Model) startModeSelection() (tea.Model, tea.Cmd) {
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = ""
	m.qualityChoices = nil
	m.audioProfiles = nil
	m.screen = scrMode
	m = m.syncMenu()
	return m, nil
}

func (m Model) startQualityScan() (tea.Model, tea.Cmd) {
	m.qualityChoices = nil
	m.profile = app.OutputProfile{}
	m.flowErr = ""
	m.screen = scrQualityFetch
	return m, loadQualityChoicesCmd(m.qualityScanURLs())
}

func (m Model) qualityScanURLs() []string {
	if m.forceSingle || m.plInfo == nil || len(m.dlEntries) == 0 {
		return []string{m.target.DownloadURL(m.forceSingle)}
	}

	urls := make([]string, 0, len(m.dlEntries))
	for _, entry := range m.dlEntries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func (m Model) currentProfile() app.OutputProfile {
	if m.profile.Mode != 0 {
		return m.profile
	}
	return app.DefaultProfileForMode(m.mode, m.locale)
}

func (m Model) startDownload() (tea.Model, tea.Cmd) {
	req := app.NormalizeDownloadRequest(app.DownloadRequest{
		Target:       m.target,
		Profile:      m.currentProfile(),
		ForceSingle:  m.forceSingle,
		PlaylistInfo: m.plInfo,
		Entries:      m.dlEntries,
		Workers:      max(m.numWorkers, 1),
		OutputDir:    app.DlDir,
		Locale:       m.locale,
	})
	if err := app.ValidateDownloadRequest(req); err != nil {
		m.flowErr = err.Error()
		switch req.Profile.Mode {
		case app.ModeAudio:
			if m.profile.Mode == 0 {
				m.screen = scrAudio
			}
		case app.ModeThumbnail:
			m.screen = scrMode
		default:
			if m.profile.Mode == 0 {
				m.screen = scrQuality
			}
		}
		m = m.syncMenu()
		return m, nil
	}

	workers := max(m.numWorkers, 1)
	if len(m.dlEntries) == 0 {
		workers = 1
	}

	m.slots = make([]slotState, workers)
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = len(m.dlEntries)
	m.singleOK = false
	m.dlStartedAt = time.Now()
	m.dlElapsed = 0
	m.timerActive = true

	ch := make(chan app.DlUpdate, 256)
	m.dlCh = ch
	m.screen = scrDownload

	req.Workers = workers
	app.StartDownloadRequest(req, ch)
	return m, tea.Batch(listenDownloadCmd(ch), timerTickCmd())
}

func (m Model) selectedPlaylistEntries() []app.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}

	selected := make([]app.PlaylistEntry, 0, len(m.plSelected))
	for _, entry := range m.plInfo.Entries {
		if m.plSelected[entry.Index] {
			selected = append(selected, entry)
		}
	}
	return selected
}

func (m Model) stepPlaylistCursor(delta int) Model {
	if m.plInfo == nil || len(m.plInfo.Entries) == 0 {
		return m
	}
	m.plCursor = max(0, min(m.plCursor+delta, len(m.plInfo.Entries)-1))
	m.ensurePlaylistCursorVisible()
	return m
}

func (m *Model) ensurePlaylistCursorVisible() {
	height := m.playlistViewportHeight()
	if height <= 0 {
		m.plTop = 0
		return
	}

	if m.plCursor < m.plTop {
		m.plTop = m.plCursor
	}
	if m.plCursor >= m.plTop+height {
		m.plTop = m.plCursor - height + 1
	}

	maxTop := max(0, len(m.playlistEntries())-height)
	m.plTop = max(0, min(m.plTop, maxTop))
}

func (m Model) playlistEntries() []app.PlaylistEntry {
	if m.plInfo == nil {
		return nil
	}
	return m.plInfo.Entries
}

func (m Model) playlistViewportHeight() int {
	lines := min(15, m.height-16)
	return max(3, lines)
}
