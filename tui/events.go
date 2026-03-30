package tui

import (
	"time"
	"unicode"

	app "YouTubeBuild/internal/app"

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

	case msgOpenDownloadsDirDone:
		if msg.err != nil && m.downloadErr == "" {
			m.downloadErr = msg.err.Error()
		}
		return m, nil
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

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if k == "ctrl+c" {
		return m, tea.Quit
	}

	if k == "tab" {
		m.locale = app.NextLocale(m.locale)
		_ = app.SaveLocale(m.locale)
		m.syncLocalizedInputs()
		m = m.syncMenu()
		return m, nil
	}

	if k == "ctrl+u" && m.canOpenDependencyScreen() {
		return m.startDepUpdate()
	}

	if m.screen == scrSummary && isOpenFolderKey(msg) && (m.singleOK || m.dlDone > 0) {
		return m, openDownloadsDirCmd(app.DlDir)
	}

	if k == "esc" {
		switch m.screen {
		case scrSearchInput, scrSearchResults:
			return m.exitSearch()
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
		case "esc":
			if m.screen == scrDepUpdate {
				return m.returnFromDependencyScreen()
			}
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

func isOpenFolderKey(msg tea.KeyPressMsg) bool {
	key := msg.Key()
	if key.BaseCode != 0 {
		return unicode.ToLower(key.BaseCode) == 'o'
	}
	return isOpenFolderRune(key.Code)
}

func isOpenFolderRune(r rune) bool {
	switch unicode.ToLower(r) {
	case 'o', 'щ':
		return true
	default:
		return false
	}
}

func (m Model) isMenuScreen() bool {
	switch m.screen {
	case scrUpdateReady, scrPlaylistAsk, scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrSearchResults, scrFragmentChoice, scrDepUpdate:
		return true
	}
	return false
}

func (m Model) handleURLKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+g":
		return m.openSearchInput()
	case "enter":
		return m.submitURLInput()
	default:
		return m.routeFocusedInputMessage(msg)
	}
}

func (m Model) handleSearchInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		return m.submitSearchInput()
	}
	return m.routeFocusedInputMessage(msg)
}

func (m Model) handleFragmentInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fragmentErr = ""
		m.fragmentIn.Blur()
		m.screen = scrFragmentChoice
		m = m.syncMenu()
		return m, nil
	case "enter":
		fragment, err := app.ParseBoundedFragmentRange(m.fragmentIn.Value(), m.mediaDuration)
		if err != nil {
			m.fragmentErr = app.FragmentInputErrorText(m.locale, err, m.mediaDuration)
			return m, nil
		}
		m.fragmentErr = ""
		m.fragment = &fragment
		m.fragmentIn.Blur()
		return m.startModeSelection()
	default:
		return m.routeFocusedInputMessage(msg)
	}
}

func (m Model) handlePlaylistKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.plInfo == nil {
		return m, nil
	}

	if m.plInputMode {
		return m.handlePlaylistInputKey(msg)
	}

	switch msg.String() {
	case "up":
		m = m.stepPlaylistCursor(-1)
	case "down":
		m = m.stepPlaylistCursor(1)
	case "space":
		m.toggleCurrentPlaylistEntry()
		m.plInputErr = ""
	case "a", "а":
		m.toggleAllPlaylistEntries()
		m.plInputErr = ""
	case "/":
		return m, m.openPlaylistInput()
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

func (m Model) handlePlaylistInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		indices, err := app.ParseSelectionFor(m.plInput.Value(), len(m.plInfo.Entries), m.locale)
		if err != nil {
			m.plInputErr = err.Error()
			return m, nil
		}

		m.applyPlaylistSelectionIndices(indices)
		m.closePlaylistInput()
		return m, nil
	case "esc":
		m.closePlaylistInput()
		return m, nil
	default:
		return m.routeFocusedInputMessage(msg)
	}
}

func (m *Model) activeInputState() (activeInputState, bool) {
	switch {
	case m.screen == scrURL:
		return activeInputState{field: &m.urlInput, err: &m.urlErr}, true
	case m.screen == scrSearchInput:
		return activeInputState{field: &m.searchInput, err: &m.searchErr}, true
	case m.screen == scrPlaylist && m.plInputMode:
		return activeInputState{field: &m.plInput, err: &m.plInputErr}, true
	case m.screen == scrFragmentInput:
		return activeInputState{field: &m.fragmentIn, err: &m.fragmentErr}, true
	}
	return activeInputState{}, false
}

type activeInputState struct {
	field *inputField
	err   *string
}

func (m *Model) pasteIntoActiveInput(content string) tea.Cmd {
	input, ok := m.activeInputState()
	if !ok {
		return nil
	}
	return m.pasteIntoInput(input, content)
}

func (m *Model) pasteIntoInput(input activeInputState, content string) tea.Cmd {
	var cmds []tea.Cmd
	if !input.field.Focused() {
		cmds = append(cmds, input.field.Focus())
	}

	before := input.field.Value()
	cmd := input.field.insertRunes([]rune(content))
	if input.field.Value() != before && input.err != nil {
		*input.err = ""
	}
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *Model) updateActiveInput(msg tea.Msg) tea.Cmd {
	input, ok := m.activeInputState()
	if !ok {
		return nil
	}

	before := input.field.Value()
	cmd := input.field.Update(msg)
	if input.field.Value() != before && input.err != nil {
		*input.err = ""
	}
	return cmd
}

func (m Model) handleTerminalPaste(content string) (tea.Model, tea.Cmd) {
	return m, (&m).pasteIntoActiveInput(content)
}

func (m Model) routeFocusedInputMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := (&m).updateActiveInput(msg); cmd != nil {
		return m, cmd
	}
	return m, nil
}

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

func streamFileProgressCmd(ch <-chan app.FileProgress, isUpdate bool) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{isUpdate: isUpdate}
		}
		if p.Done {
			return msgDepDone{err: p.Err, isUpdate: isUpdate}
		}
		return msgDepProgress{progress: p}
	}
}

func launchProgress(fn func(chan<- app.FileProgress) error, isUpdate bool) (<-chan app.FileProgress, tea.Cmd) {
	ch := make(chan app.FileProgress, 16)
	go func() {
		defer close(ch)

		progressCh := make(chan app.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			doneCh <- fn(progressCh)
		}()

		for progress := range progressCh {
			progress.Done = false
			progress.Err = nil
			ch <- progress
		}

		if err := <-doneCh; err != nil {
			ch <- app.FileProgress{Done: true, Err: err}
			return
		}

		ch <- app.FileProgress{Done: true}
	}()
	return ch, streamFileProgressCmd(ch, isUpdate)
}

func refreshDepsCmd(token int) tea.Cmd {
	return func() tea.Msg {
		return msgDepsRefreshed{deps: app.RefreshDeps(), token: token}
	}
}

func fetchPlaylistCmd(url string, l app.Locale) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(nil, url, l)
		return msgPlaylistFetched{info: info, err: err}
	}
}

func searchYouTubeCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := app.SearchYouTube(query)
		return msgSearchResults{results: results, err: err}
	}
}

func loadQualityChoicesCmd(urls []string) tea.Cmd {
	return func() tea.Msg {
		choices, err := app.ResolveQualityChoices(urls)
		return msgQualityScanned{choices: choices, err: err}
	}
}

func probeFragmentDurationCmd(target app.ParsedTarget) tea.Cmd {
	return func() tea.Msg {
		duration, err := app.ProbeMediaDuration(target)
		return msgFragmentDuration{duration: duration, err: err}
	}
}

func listenDownloadCmd(ch <-chan app.DlUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{update: app.DlUpdate{Type: app.EvClosed}}
		}
		return msgDlUpdate{update: u}
	}
}
