package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

type screen int

const (
	scrUpdateCheck screen = iota
	scrUpdateReady
	scrUpdateDl
	scrUpdateDone
	scrDepDl
	scrDepUpdate
	scrURL
	scrSearchInput
	scrSearchFetch
	scrSearchResults
	scrPlaylistAsk
	scrPlaylistFetch
	scrPlaylist
	scrFragmentProbe
	scrFragmentChoice
	scrFragmentInput
	scrMode
	scrAudio
	scrQualityFetch
	scrQuality
	scrVideoOutput
	scrWorkers
	scrDownload
	scrSummary
)

type inputTarget uint8

const (
	inputURL inputTarget = iota
	inputSearch
	inputPlaylist
	inputFragment
)

type slotState struct {
	title  string
	pct    float64
	doneB  int64
	totalB int64
	speed  string
	label  string
	proc   bool
	done   bool
	failed bool
}

type depScreenMode uint8

const (
	depModeStartup depScreenMode = iota + 1
	depModeManage
)

type depActionKind uint8

const (
	depActionInstall depActionKind = iota + 1
	depActionContinue
	depActionRefresh
	depActionBack
	depActionExit
)

type depAction struct {
	Kind  depActionKind
	Key   string
	Label string
}

type (
	msgUpdateChecked struct{ info *app.UpdateInfo }
	msgDepProgress   struct{ progress app.FileProgress }
	msgDepDone       struct {
		err      error
		isUpdate bool
	}
	msgDepsRefreshed struct {
		deps  app.CheckDepsResult
		token int
	}
	msgPlaylistFetched struct {
		info *app.PlaylistInfo
		err  error
	}
	msgSearchResults struct {
		results []app.SearchResult
		err     error
	}
	msgQualityScanned struct {
		choices []app.QualityChoice
		err     error
	}
	msgFragmentDuration struct {
		duration int
		err      error
	}
	msgDlUpdate             struct{ update app.DlUpdate }
	msgOpenDownloadsDirDone struct{ err error }
	msgPickDownloadsDirDone struct {
		path string
		err  error
	}

	spinnerTickMsg struct{}
	timerTickMsg   time.Time
	cursorBlinkMsg struct {
		target inputTarget
		tag    int
	}
)

type Model struct {
	screen screen

	width  int
	height int

	locale app.Locale

	spinnerFrame int

	deps    app.CheckDepsResult
	depMode depScreenMode

	updateInfo  *app.UpdateInfo
	depProgress app.FileProgress
	depLabel    string
	depErr      string
	depCh       <-chan app.FileProgress

	urlInput      inputField
	urlErr        string
	target        app.ParsedTarget
	searchInput   inputField
	searchQuery   string
	searchErr     string
	searchResults []app.SearchResult

	plInfo        *app.PlaylistInfo
	plCursor      int
	plTop         int
	plSelected    map[int]bool
	plInputMode   bool
	plInput       inputField
	plInputErr    string
	mediaDuration int
	fragment      *app.DownloadFragment
	fragmentErr   string
	fragmentIn    inputField

	menu menu

	mode           app.DownloadMode
	profile        app.OutputProfile
	qualityChoices []app.QualityChoice
	videoProfiles  []app.OutputProfile
	audioProfiles  []app.OutputProfile
	flowErr        string
	url            string
	dlEntries      []app.PlaylistEntry
	forceSingle    bool
	numWorkers     int
	dlCh           <-chan app.DlUpdate
	slots          []slotState
	dlDone         int
	dlFailed       int
	dlTotal        int
	singleOK       bool
	downloadErr    string
	dlStartedAt    time.Time
	dlElapsed      time.Duration
	timerActive    bool

	session         app.Session
	depReturnScreen screen
	depRefreshing   bool
	depRefreshToken int
	depUpdateDone   bool

	dlCancel    context.CancelFunc
	dlCancelled bool
}

func New() tea.Model {
	return newModel()
}

func newModel() Model {
	loc := app.LoadLocale()

	m := Model{
		screen:      scrUpdateCheck,
		locale:      loc,
		urlInput:    newInput(inputURL, "https://youtu.be/...", inputW, 300),
		searchInput: newInput(inputSearch, app.StringsFor(loc).SearchPlaceholder, inputW, 120),
		plInput:     newInput(inputPlaylist, app.StringsFor(loc).PlInputPlaceholder, 38, 100),
		fragmentIn:  newInput(inputFragment, "1:00-2:30", 28, 32),
		mode:        app.DefaultDownloadMode(),
		profile:     app.DefaultVideoProfile(loc),
		numWorkers:  1,
		plSelected:  map[int]bool{},
	}
	m.syncLayout()
	return m
}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(ts time.Time) tea.Msg { return timerTickMsg(ts) })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		func() tea.Msg { return msgUpdateChecked{info: app.CheckUpdate()} },
	)
}

func (m Model) u() *app.UIStrings {
	return app.StringsFor(m.locale)
}

func fitWidth(available, preferred, minWidth int) int {
	if available <= 0 {
		return 1
	}
	if available >= preferred {
		return preferred
	}
	if available >= minWidth {
		return available
	}
	return max(available, 1)
}

func (m Model) cardWidth() int {
	if m.width <= 0 {
		return cardW
	}
	return fitWidth(m.width-4, cardW, 38)
}

func (m Model) cardPadding() (int, int) {
	switch {
	case m.height > 0 && m.height < 28:
		return 0, 1
	case m.height > 0 && m.height < 34:
		return 0, 2
	default:
		return 1, 2
	}
}

func (m Model) cardBodyWidth() int {
	_, px := m.cardPadding()
	if m.width <= 0 {
		return max(1, cardW-(px*2)-2)
	}
	return max(1, m.cardWidth()-(px*2)-2)
}

func (m Model) menuWidth() int {
	return fitWidth(m.cardBodyWidth(), menuW, 24)
}

func (m Model) primaryInputWidth() int {
	return fitWidth(m.cardBodyWidth()-4, inputW, 18)
}

func (m Model) playlistInputWidth() int {
	return fitWidth(m.cardBodyWidth()-12, 38, 14)
}

func (m Model) fragmentInputWidth() int {
	return fitWidth(m.cardBodyWidth()-20, 28, 12)
}

func (m Model) progressBarWidth() int {
	return fitWidth(m.cardBodyWidth()-18, barW, 12)
}

func (m Model) playlistTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-18, 40, 16)
}

func (m Model) slotTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-14, 46, 18)
}

func (m *Model) syncLayout() {
	m.urlInput.SetWidth(m.primaryInputWidth())
	m.searchInput.SetWidth(m.primaryInputWidth())
	m.plInput.SetWidth(m.playlistInputWidth())
	m.fragmentIn.SetWidth(m.fragmentInputWidth())
}

func (m Model) qualityOptions() []string {
	return app.QualityChoiceLabels(m.qualityChoices, m.locale)
}

func (m Model) audioOptions() []string {
	return app.OutputProfileLabels(m.audioProfiles)
}

func (m Model) videoOutputOptions() []string {
	return app.OutputProfileLabels(m.videoProfiles)
}

func (m Model) modeOptions() []string {
	u := m.u()
	return []string{u.ModeVideo, u.ModeAudio, u.ModeThumbnail}
}

func (m Model) uiBusy() bool {
	switch m.screen {
	case scrUpdateDl, scrDepDl, scrDepUpdate, scrDownload, scrPlaylistFetch, scrFragmentProbe, scrQualityFetch, scrSearchFetch:
		return true
	}
	return false
}

func (m Model) isAppUpdateScreen() bool {
	switch m.screen {
	case scrUpdateCheck, scrUpdateReady, scrUpdateDl, scrUpdateDone:
		return true
	}
	return false
}

func (m Model) canOpenDependencyScreen() bool {
	return !m.uiBusy() && !m.isAppUpdateScreen()
}

func (m Model) canOpenDownloadsFolder() bool {
	return m.screen == scrURL || (m.screen == scrSummary && (m.singleOK || m.dlDone > 0))
}

func (m Model) canPickDownloadsFolder() bool {
	return m.screen == scrURL
}

func (m Model) startOpenDownloadsDir() (tea.Model, tea.Cmd) {
	if m.screen == scrURL {
		m.urlErr = ""
	}
	return m, openDownloadsDirCmd(app.DlDir)
}

func (m Model) startPickDownloadsDir() (tea.Model, tea.Cmd) {
	m.urlErr = ""
	return m, pickDownloadsDirCmd(app.DlDir, m.locale)
}

func (m Model) restoreActiveScreen() (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrMode, scrAudio, scrSummary, scrWorkers, scrQuality, scrVideoOutput, scrSearchResults, scrFragmentChoice, scrPlaylistAsk:
		m = m.syncMenu()
		return m, nil
	case scrURL:
		return m, m.urlInput.Focus()
	case scrSearchInput:
		return m, m.searchInput.Focus()
	case scrFragmentInput:
		return m, m.fragmentIn.Focus()
	case scrPlaylist:
		if m.plInputMode {
			return m, m.plInput.Focus()
		}
	}
	return m, nil
}

func (m Model) workerMenuOptions(n int) []string {
	u := m.u()
	opts := make([]string, n)
	opts[0] = u.WorkerSeq
	for i := 1; i < n; i++ {
		opts[i] = fmt.Sprintf(u.WorkerNFmt, i+1)
	}
	return opts
}

func (m Model) syncMenu() Model {
	m.menu.SetItems(m.menuItems())
	return m
}

func (m Model) menuItems() []string {
	u := m.u()
	switch m.screen {
	case scrUpdateReady:
		return []string{u.MenuUpdateY, u.MenuUpdateN}
	case scrPlaylistAsk:
		return []string{u.MenuVidOnly, u.MenuOpenPl}
	case scrDepUpdate:
		actions := m.depActions()
		items := make([]string, 0, len(actions))
		for _, action := range actions {
			items = append(items, action.Label)
		}
		return items
	case scrMode:
		return m.modeOptions()
	case scrFragmentChoice:
		return m.fragmentChoiceOptions()
	case scrAudio:
		return m.audioOptions()
	case scrSearchResults:
		return m.searchResultOptions()
	case scrSummary:
		return []string{u.MenuAgainY, u.MenuAgainN}
	case scrQuality:
		return m.qualityOptions()
	case scrVideoOutput:
		return m.videoOutputOptions()
	case scrWorkers:
		return m.workerMenuOptions(min(len(m.dlEntries), 5))
	default:
		return nil
	}
}

func (m *Model) syncLocalizedInputs() {
	m.searchInput.SetPlaceholder(m.u().SearchPlaceholder)
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
	m.menu.SetItems(m.menuItems())
	m.syncLayout()
}

func (m Model) fragmentChoiceOptions() []string {
	u := m.u()
	options := []string{u.MenuFullVideo}
	if m.canUseURLStartFragment() {
		options = append(options, fmt.Sprintf("%s (%s)", u.MenuFromURLStart, app.FormatClockTimestamp(m.target.URLStartAt)))
	}
	return append(options, u.MenuManualRange)
}

func (m Model) canUseURLStartFragment() bool {
	return m.target.HasURLStart && m.target.URLStartAt > 0 && m.mediaDuration > 0 && m.target.URLStartAt < m.mediaDuration
}

func (m Model) searchResultOptions() []string {
	options := make([]string, 0, len(m.searchResults))
	for i, result := range m.searchResults {
		label := strings.TrimSpace(result.Title)
		if label == "" {
			label = fmt.Sprintf(m.u().VideoTitleFmt, i+1)
		}
		if result.Duration > 0 {
			label += "  ·  " + app.FmtDuration(result.Duration)
		}
		options = append(options, label)
	}
	return options
}

func (m Model) selectedPlaylistCount() int {
	return len(m.plSelected)
}

func (m *Model) clearPlaylistSelection() {
	m.plSelected = map[int]bool{}
}

func (m *Model) applyPlaylistSelectionIndices(indices []int) {
	if len(indices) == 0 {
		m.clearPlaylistSelection()
		return
	}

	selected := make(map[int]bool, len(indices))
	for _, idx := range indices {
		selected[idx] = true
	}
	m.plSelected = selected
}

func (m *Model) toggleCurrentPlaylistEntry() {
	if m.plInfo == nil || m.plCursor < 0 || m.plCursor >= len(m.plInfo.Entries) {
		return
	}

	idx := m.plInfo.Entries[m.plCursor].Index
	if m.plSelected[idx] {
		delete(m.plSelected, idx)
		return
	}
	m.plSelected[idx] = true
}

func (m *Model) toggleAllPlaylistEntries() {
	total := len(m.playlistEntries())
	if total == 0 {
		m.clearPlaylistSelection()
		return
	}
	if len(m.plSelected) == total {
		m.clearPlaylistSelection()
		return
	}

	selected := make(map[int]bool, total)
	for _, entry := range m.playlistEntries() {
		selected[entry.Index] = true
	}
	m.plSelected = selected
}

func (m *Model) openPlaylistInput() tea.Cmd {
	m.plInputMode = true
	m.plInputErr = ""
	m.plInput.SetValue("")
	return m.plInput.Focus()
}

func (m *Model) closePlaylistInput() {
	m.plInput.Blur()
	m.plInputMode = false
	m.plInputErr = ""
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
	lines := min(14, m.height-18)
	return max(4, lines)
}

func (m *Model) resetSearchState() {
	m.searchQuery = ""
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.SetValue("")
	m.searchInput.Blur()
}

func (m *Model) resetPlaylistState() {
	m.plInfo = nil
	m.plCursor = 0
	m.plTop = 0
	m.clearPlaylistSelection()
	m.plInput.SetValue("")
	m.closePlaylistInput()
}

func (m *Model) resetProfileState() {
	m.forceSingle = false
	m.numWorkers = 1
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = ""
	m.dlEntries = nil
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.audioProfiles = nil
}

func (m *Model) resetFragmentState() {
	m.mediaDuration = 0
	m.fragment = nil
	m.fragmentErr = ""
	m.fragmentIn.SetValue("")
	m.fragmentIn.Blur()
}

func (m *Model) resetDownloadProgressState() {
	m.slots = nil
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = 0
	m.singleOK = false
	m.downloadErr = ""
	m.dlStartedAt = time.Time{}
	m.dlElapsed = 0
	m.timerActive = false
	m.dlCancel = nil
	m.dlCancelled = false
}

func (m *Model) resetDownloadState() {
	m.resetProfileState()
	m.resetFragmentState()
	m.resetDownloadProgressState()
}

func (m *Model) resetTargetFlowState() {
	m.resetPlaylistState()
	m.resetProfileState()
	m.resetFragmentState()
	m.searchResults = nil
	m.searchErr = ""
}

func (m Model) resetForNext() (tea.Model, tea.Cmd) {
	m.screen = scrURL
	m.url = ""
	m.urlErr = ""
	m.urlInput.SetValue("")
	m.target = app.ParsedTarget{}
	m.resetSearchState()
	m.resetPlaylistState()
	m.resetDownloadState()
	return m, m.urlInput.Focus()
}

func (m Model) handleSearchResultsMsg(msg msgSearchResults) (tea.Model, tea.Cmd) {
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

func (m Model) openSearchInput() (tea.Model, tea.Cmd) {
	m.screen = scrSearchInput
	m.searchErr = ""
	m.searchResults = nil
	if strings.TrimSpace(m.searchInput.Value()) == "" {
		m.searchInput.SetValue("")
	}
	return m, m.searchInput.Focus()
}

func (m Model) submitSearchInput() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		m.searchErr = m.u().SearchErrEmpty
		return m, nil
	}

	m.searchQuery = query
	m.searchErr = ""
	m.searchResults = nil
	m.screen = scrSearchFetch
	return m, searchYouTubeCmd(query)
}

func (m Model) activateSearchResult(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.searchResults) {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed
		return m, m.searchInput.Focus()
	}

	result := m.searchResults[idx]
	if strings.TrimSpace(result.URL) == "" {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed
		return m, m.searchInput.Focus()
	}

	target, err := app.ParseTarget(result.URL)
	if err != nil {
		m.screen = scrSearchInput
		m.searchErr = m.u().SearchErrFailed + ": " + err.Error()
		return m, m.searchInput.Focus()
	}
	return m.startTargetFlow(result.URL, target)
}

func (m Model) exitSearch() (tea.Model, tea.Cmd) {
	m.screen = scrURL
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.Blur()
	return m, m.urlInput.Focus()
}

func (m Model) handlePlaylistFetched(msg msgPlaylistFetched) (tea.Model, tea.Cmd) {
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

func (m Model) submitURLInput() (tea.Model, tea.Cmd) {
	rawURL := strings.TrimSpace(m.urlInput.Value())
	if rawURL == "" {
		m.urlErr = m.u().URLErrEmpty
		return m, nil
	}

	target, err := app.ParseTarget(rawURL)
	if err != nil {
		m.urlErr = m.u().URLErrBad
		return m, nil
	}

	m.urlErr = ""
	return m.startTargetFlow(rawURL, target)
}

func (m Model) startTargetFlow(rawURL string, target app.ParsedTarget) (tea.Model, tea.Cmd) {
	m.url = rawURL
	m.urlInput.SetValue(rawURL)
	m.target = target
	m.resetTargetFlowState()

	if target.IsPlaylist() {
		if target.Kind == app.TargetMixed {
			m.screen = scrPlaylistAsk
			m = m.syncMenu()
			return m, nil
		}
		m.screen = scrPlaylistFetch
		return m, fetchPlaylistCmd(rawURL, m.locale)
	}

	return m.startFragmentFlow()
}

func (m Model) startFragmentFlow() (tea.Model, tea.Cmd) {
	m.resetFragmentState()
	m.screen = scrFragmentProbe
	return m, probeFragmentDurationCmd(m.target)
}

func (m Model) handleFragmentDurationMsg(msg msgFragmentDuration) (tea.Model, tea.Cmd) {
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

func (m Model) gotoChecks() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps()
	m.deps = deps
	if deps.MissingRequired() {
		return m.openDependencyScreen(depModeStartup)
	}
	return m.gotoURLWithDeps(deps)
}

func (m Model) gotoURL() (tea.Model, tea.Cmd) {
	return m.gotoURLWithDeps(app.DetectDeps())
}

func (m Model) gotoURLWithDeps(deps app.CheckDepsResult) (tea.Model, tea.Cmd) {
	m.deps = deps
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) withDeps(deps app.CheckDepsResult) Model {
	m.deps = deps
	return m
}

func (m Model) openDependencyScreen(mode depScreenMode) (tea.Model, tea.Cmd) {
	return m.openDependencyScreenWithError(mode, "")
}

func (m Model) openDependencyScreenWithError(mode depScreenMode, errText string) (tea.Model, tea.Cmd) {
	if mode == depModeStartup {
		m.depReturnScreen = scrURL
	}
	m.depMode = mode
	m.depErr = errText
	m.screen = scrDepUpdate
	m = m.syncMenu()
	if mode == depModeManage {
		return m.startDepsRefresh()
	}
	return m, nil
}

func (m Model) startDepUpdate() (tea.Model, tea.Cmd) {
	m.depReturnScreen = m.screen
	m.depUpdateDone = false
	return m.openDependencyScreen(depModeManage)
}

func (m Model) startDepsRefresh() (tea.Model, tea.Cmd) {
	m.depRefreshToken++
	m.depRefreshing = true
	if m.screen == scrDepUpdate {
		m = m.syncMenu()
	}
	return m, refreshDepsCmd(m.depRefreshToken)
}

func (m Model) returnFromDependencyScreen() (tea.Model, tea.Cmd) {
	if m.depMode == depModeStartup {
		m.depErr = ""
		if !m.deps.MissingRequired() {
			return m.gotoURLWithDeps(app.DetectDeps())
		}
		return m, tea.Quit
	}

	target := m.depReturnScreen
	if target == scrUpdateCheck {
		target = scrURL
	}

	m.depErr = ""
	m.screen = target
	return m.restoreActiveScreen()
}

func (m Model) depActions() []depAction {
	actions := make([]depAction, 0, 5)
	for _, dep := range m.deps.ActionableDependencies() {
		label := m.depActionLabel(depActionInstall, dep.Name, dep.Available && dep.Source == app.DepManaged)
		actions = append(actions, depAction{Kind: depActionInstall, Key: dep.Key, Label: label})
	}

	if !m.depRefreshing {
		actions = append(actions, depAction{Kind: depActionRefresh, Label: m.depActionLabel(depActionRefresh, "", false)})
	}
	if m.depMode == depModeStartup && !m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionContinue, Label: m.depActionLabel(depActionContinue, "", false)})
	}
	if m.depMode == depModeManage {
		actions = append(actions, depAction{Kind: depActionBack, Label: m.depActionLabel(depActionBack, "", false)})
	} else if m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionExit, Label: m.depActionLabel(depActionExit, "", false)})
	}
	return actions
}

func (m Model) startDependencyDownload(
	screen screen,
	label string,
	isUpdate bool,
	fn func(chan<- app.FileProgress) error,
) (tea.Model, tea.Cmd) {
	m.depLabel = label
	m.depProgress = app.FileProgress{}
	m.depErr = ""
	m.screen = screen

	var cmd tea.Cmd
	m.depCh, cmd = launchProgress(fn, isUpdate)
	return m, cmd
}

func (m Model) depActionLabel(kind depActionKind, name string, isUpdate bool) string {
	u := m.u()
	switch kind {
	case depActionInstall:
		if isUpdate {
			return fmt.Sprintf(u.DepActionUpdateFmt, name)
		}
		return fmt.Sprintf(u.DepActionDownloadFmt, name)
	case depActionRefresh:
		return u.DepActionRefresh
	case depActionContinue:
		return u.DepActionContinue
	case depActionBack:
		return u.DepActionBack
	case depActionExit:
		return u.DepActionExit
	}
	return name
}

func (m Model) depRequirementText(name string) string {
	return fmt.Sprintf(m.u().DepRequirementFmt, name)
}

func (m Model) handleDlUpdate(u app.DlUpdate) (tea.Model, tea.Cmd) {
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
		return m, listenDownloadCmd(m.dlCh)
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

	return m, listenDownloadCmd(m.dlCh)
}

func (m Model) downloadLabel() string {
	profile := m.currentProfile()
	label := profile.Label
	if label == "" {
		switch profile.Mode {
		case app.ModeAudio:
			label = m.u().ModeAudio
		case app.ModeThumbnail:
			label = m.u().ModeThumbnail
		default:
			label = m.u().ModeVideo
		}
	}
	if profile.Mode == app.ModeThumbnail {
		return label
	}
	if fragment := app.FormatFragmentLabel(m.fragment); fragment != "" {
		return label + " [" + fragment + "]"
	}
	return label
}

func (m Model) exitToURL() (tea.Model, tea.Cmd) {
	m.resetTargetFlowState()
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) gotoModeSelection() (tea.Model, tea.Cmd) {
	return m.startModeSelection()
}

func (m Model) gotoQualitySelection() (tea.Model, tea.Cmd) {
	m.screen = scrQuality
	m = m.syncMenu()
	return m, nil
}

func (m Model) gotoWorkersBack() (tea.Model, tea.Cmd) {
	if m.mode == app.ModeAudio {
		m.screen = scrAudio
		m = m.syncMenu()
		return m, nil
	}
	m.screen = scrVideoOutput
	m = m.syncMenu()
	return m, nil
}

func (m Model) startModeSelection() (tea.Model, tea.Cmd) {
	return m.startModeSelectionWithNotice("")
}

func (m Model) startModeSelectionWithNotice(notice string) (tea.Model, tea.Cmd) {
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = notice
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.audioProfiles = nil
	m.screen = scrMode
	m = m.syncMenu()
	return m, nil
}

func (m Model) startQualityScan() (tea.Model, tea.Cmd) {
	m.qualityChoices = nil
	m.videoProfiles = nil
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
	deps := app.DetectDeps()
	m.deps = deps

	switch {
	case !deps.YTDLP.Available:
		m.depReturnScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.YTDLP.Name))
	case (m.currentProfile().Mode == app.ModeAudio || m.fragment != nil || m.currentProfile().RequiresVideoPostprocessing()) && !deps.FFmpeg.Available:
		m.depReturnScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.FFmpeg.Name))
	}

	req, err := app.PrepareDownloadRequest(app.DownloadRequest{
		Target:        m.target,
		Profile:       m.currentProfile(),
		Fragment:      m.fragment,
		MediaDuration: m.mediaDuration,
		ForceSingle:   m.forceSingle,
		PlaylistInfo:  m.plInfo,
		Entries:       m.dlEntries,
		Workers:       max(m.numWorkers, 1),
		OutputDir:     app.DlDir,
		Locale:        m.locale,
	})
	if err != nil {
		m.flowErr = err.Error()
		m.restoreDownloadConfigScreen()
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
	m.downloadErr = ""
	m.dlStartedAt = time.Now()
	m.dlElapsed = 0
	m.timerActive = true

	ch := make(chan app.DlUpdate, 256)
	m.dlCh = ch
	m.screen = scrDownload

	dlCtx, dlCancel := context.WithCancel(context.Background())
	m.dlCancel = dlCancel

	req.Workers = workers
	app.StartDownloadRequestContext(dlCtx, req, ch)
	return m, tea.Batch(listenDownloadCmd(ch), timerTickCmd())
}

func (m Model) cancelDownload() (tea.Model, tea.Cmd) {
	if m.dlCancel != nil {
		m.dlCancel()
		m.dlCancel = nil
	}
	m.timerActive = false
	m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
	m.resetDownloadState()
	m.dlCancelled = true
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m *Model) restoreDownloadConfigScreen() {
	switch m.currentProfile().Mode {
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
}
