package tui

import (
	"context"
	"errors"
	"strconv"
	"time"

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
		m.depProgress = msg.progress
		return m, streamFileProgressCmd(m.depCh, m.screen == scrUpdateDl)

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
		return m.handleDlUpdate(msg.update)

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

	return m.routeFocusedInputMessage(msg)
}

func (m *Model) updateElapsed() {
	m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
}

func (m Model) handleDepDone(msg msgDepDone) (tea.Model, tea.Cmd) {
	if errors.Is(msg.err, context.Canceled) {
		m.depErr = ""
		if m.screen == scrDepDl {
			m.screen = scrDepUpdate
			m = m.syncMenu()
		} else if m.screen == scrUpdateDl {
			m.screen = scrUpdateReady
			m = m.syncMenu()
		}
		return m, nil
	}
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

func (m Model) handleQualityScanned(msg msgQualityScanned) (tea.Model, tea.Cmd) {
	if msg.gen != m.opGen {
		return m, nil
	}
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

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if !m.isMenuScreen() || !isDigitKey(k) {
		m.menuDigits = ""
	}

	if k == "tab" {
		m.locale = app.NextLocale(m.locale)
		_ = app.SaveLocale(m.env, m.locale)
		m.syncLocalizedInputs()
		m = m.syncMenu()
		return m, nil
	}

	if k == "ctrl+u" && m.canOpenDependencyScreen() {
		return m.startDepUpdate()
	}

	if isPickFolderKey(msg) && m.canPickDownloadsFolder() {
		if app.DownloadsDirLocked() {
			m.urlErr = m.u().DownloadsDirLocked
			return m, nil
		}
		return m.startPickDownloadsDir()
	}

	if isOpenFolderKey(msg) && m.canOpenDownloadsFolder() &&
		(m.screen != scrURL || isUppercaseOpenFolderKey(msg)) {
		return m.startOpenDownloadsDir()
	}

	if k == "esc" {
		switch m.screen {
		case scrSearchInput, scrSearchResults, scrSearchFetch:
			return m.exitSearch()
		case scrPlaylistAsk, scrPlaylist:
			return m.exitToURL()
		case scrPlaylistFetch:
			m = m.cancelOps()
			return m.exitToURL()
		case scrFragmentChoice:
			return m.exitToURL()
		case scrFragmentProbe:
			m = m.cancelOps()
			return m.exitToURL()
		case scrQualityFetch:
			m = m.cancelOps()
			return m.startModeSelectionWithNotice("")
		case scrMode:
			return m.exitToURL()
		case scrFragmentInput:
			return m.handleFragmentInputKey(msg)
		case scrAudio:
			return m.startModeSelectionWithNotice("")
		case scrQuality:
			return m.startModeSelectionWithNotice("")
		case scrVideoOutput:
			return m.gotoQualitySelection()
		case scrWorkers:
			return m.gotoWorkersBack()
		case scrDownload:
			return m.cancelDownload()
		case scrSummary:
			return m.resetForNext()
		case scrDepUpdate:
			return m.returnFromDependencyScreen()
		case scrUpdateDl, scrDepDl:
			if m.depCancel != nil {
				m.depCancel()
			}
			return m, nil
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

func isOpenFolderKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "o", "O", "щ", "Щ":
		return true
	default:
		return false
	}
}

func isUppercaseOpenFolderKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "O", "Щ":
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

func digitTimeoutCmd() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg { return menuDigitTickMsg{} })
}

func (m Model) handleMenuDigit(digit string) (tea.Model, tea.Cmd) {
	if !m.isMenuScreen() || len(m.menu.items) == 0 {
		m.menuDigits = ""
		return m, nil
	}
	buf := m.menuDigits + digit
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 || n > len(m.menu.items) {
		m.menuDigits = ""
		return m, nil
	}
	m.menuDigits = buf
	m.menuDigitsScreen = m.screen
	if n*10 > len(m.menu.items) {
		m.menuDigits = ""
		m.menu.SetCursor(n - 1)
		return m.activateMenu()
	}
	return m, digitTimeoutCmd()
}

func (m Model) activatePendingDigits() (tea.Model, tea.Cmd) {
	buf := m.menuDigits
	m.menuDigits = ""
	if buf == "" || !m.isMenuScreen() || m.screen != m.menuDigitsScreen {
		return m, nil
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 || n > len(m.menu.items) {
		return m, nil
	}
	m.menu.SetCursor(n - 1)
	return m.activateMenu()
}

func (m Model) isMenuScreen() bool {
	switch m.screen {
	case scrUpdateReady, scrPlaylistAsk, scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrVideoOutput, scrSearchResults, scrFragmentChoice, scrDepUpdate:
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
		return m.startModeSelectionWithNotice("")
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
		return m.startModeSelectionWithNotice("")
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
	if len(m.menu.items) == 0 {
		return m, nil
	}
	idx := m.menu.Index()

	switch m.screen {
	case scrUpdateReady:
		if idx == 0 {
			info := m.updateInfo
			return m.startDependencyDownload(scrUpdateDl, "", true, func(ctx context.Context, ch chan<- app.FileProgress) error {
				return app.ApplyUpdateFor(m.env, ctx, m.locale, info, ch)
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
			return m.startDependencyDownload(scrDepDl, action.Key, false, func(ctx context.Context, ch chan<- app.FileProgress) error {
				return app.InstallDependencyFor(m.env, ctx, action.Key, m.locale, ch)
			})
		case depActionRefresh:
			return m.startDepsRefresh()
		case depActionContinue:
			return m.gotoURLWithDeps(app.DetectDeps(m.env))
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
		var ctx context.Context
		m, ctx = m.nextOpCtx()
		m.screen = scrPlaylistFetch
		return m, fetchPlaylistCmd(m.env, ctx, m.url, m.locale, m.opGen)

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
		m.videoProfiles = app.VideoOutputProfiles(m.profile, m.locale)
		m.flowErr = ""
		m.screen = scrVideoOutput
		m = m.syncMenu()
		return m, nil

	case scrVideoOutput:
		if idx < 0 || idx >= len(m.videoProfiles) {
			return m, nil
		}
		m.profile = m.videoProfiles[idx]
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
		return m.startModeSelectionWithNotice("")
	case m.canUseURLStartFragment() && idx == 1:
		fragment := app.DownloadFragment{StartAt: m.target.URLStartAt}
		if err := app.ValidateFragmentDuration(fragment, m.mediaDuration); err != nil {
			m.flowErr = app.FragmentURLStartOutOfBoundsText(m.locale, m.mediaDuration)
			m = m.syncMenu()
			return m, nil
		}
		m.fragment = &fragment
		return m.startModeSelectionWithNotice("")
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

func launchProgress(
	base context.Context,
	fn func(context.Context, chan<- app.FileProgress) error,
	isUpdate bool,
) (<-chan app.FileProgress, tea.Cmd, context.CancelFunc) {
	ch := make(chan app.FileProgress, 16)
	ctx, cancel := context.WithCancel(base)

	go func() {
		defer close(ch)

		progressCh := make(chan app.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			doneCh <- fn(ctx, progressCh)
		}()

		for progress := range progressCh {
			select {
			case <-ctx.Done():
				return
			default:
			}
			progress.Done = false
			progress.Err = nil
			ch <- progress
		}

		if err := <-doneCh; err != nil {
			if ctx.Err() != nil {
				err = context.Canceled
			}
			ch <- app.FileProgress{Done: true, Err: err}
			return
		}

		ch <- app.FileProgress{Done: true}
	}()

	return ch, streamFileProgressCmd(ch, isUpdate), cancel
}

func refreshDepsCmd(env *app.Env, token int) tea.Cmd {
	return func() tea.Msg {
		return msgDepsRefreshed{deps: app.RefreshDeps(env), token: token}
	}
}

func fetchPlaylistCmd(env *app.Env, ctx context.Context, url string, l app.Locale, gen int) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(env, ctx, url, l)
		return msgPlaylistFetched{info: info, err: err, gen: gen}
	}
}

func searchYouTubeCmd(env *app.Env, ctx context.Context, query string, gen int) tea.Cmd {
	return func() tea.Msg {
		results, err := app.SearchYouTubeContext(env, ctx, query)
		return msgSearchResults{results: results, err: err, gen: gen}
	}
}

func loadQualityChoicesCmd(env *app.Env, ctx context.Context, urls []string, gen int) tea.Cmd {
	return func() tea.Msg {
		choices, err := app.ResolveQualityChoicesContext(env, ctx, urls)
		return msgQualityScanned{choices: choices, err: err, gen: gen}
	}
}

func probeFragmentDurationCmd(env *app.Env, ctx context.Context, target app.ParsedTarget, gen int) tea.Cmd {
	return func() tea.Msg {
		duration, err := app.ProbeMediaDurationContext(env, ctx, target)
		return msgFragmentDuration{duration: duration, err: err, gen: gen}
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
