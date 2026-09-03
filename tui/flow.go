package tui

import (
	"context"
	"strings"
	"time"

	app "volvid/internal/app"

	tea "charm.land/bubbletea/v2"
)

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
	m.mode = app.ModeVideo
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

func (m Model) exitToURL() (tea.Model, tea.Cmd) {
	m = m.cancelOps()
	m.resetTargetFlowState()
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) gotoQualitySelection() (tea.Model, tea.Cmd) {
	m.screen = scrQuality
	m = m.syncMenu()
	return m, nil
}

func (m Model) gotoWorkersBack() (tea.Model, tea.Cmd) {
	switch m.mode {
	case app.ModeAudio:
		m.screen = scrAudio
	case app.ModeThumbnail:
		m.screen = scrMode
	default:
		m.screen = scrVideoOutput
	}
	m = m.syncMenu()
	return m, nil
}

func (m Model) startModeSelectionWithNotice(notice string) (tea.Model, tea.Cmd) {
	m.mode = app.ModeVideo
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = notice
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.audioProfiles = nil
	m.screen = scrMode
	m = m.syncMenu()
	return m, nil
}

func (m Model) startOpenDownloadsDir() (tea.Model, tea.Cmd) {
	if m.screen == scrURL {
		m.urlErr = ""
	}
	return m, openDownloadsDirCmd(m.env.DownloadsDir())
}

func (m Model) startPickDownloadsDir() (tea.Model, tea.Cmd) {
	m.urlErr = ""
	return m, pickDownloadsDirCmd(m.env, m.env.DownloadsDir(), m.locale)
}

func (m Model) submitURLInput() (tea.Model, tea.Cmd) {
	rawURL := strings.TrimSpace(m.urlInput.Value())
	if rawURL == "" {
		m.urlErr = m.u().URLErrEmpty
		return m, nil
	}

	target, err := app.ParseTarget(rawURL)
	if err != nil {
		m.urlErr = m.u().URLErrBad + ": " + err.Error()
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
		var ctx context.Context
		m, ctx = m.nextOpCtx()
		m.screen = scrPlaylistFetch
		return m, tea.Batch(fetchPlaylistCmd(m.env, ctx, rawURL, m.locale, m.opGen), spinnerTickCmd())
	}

	return m.startFragmentFlow()
}

func (m Model) startFragmentFlow() (tea.Model, tea.Cmd) {
	m.resetFragmentState()
	var ctx context.Context
	m, ctx = m.nextOpCtx()
	m.screen = scrFragmentProbe
	return m, tea.Batch(probeFragmentDurationCmd(m.env, ctx, m.target, m.opGen), spinnerTickCmd())
}

func (m Model) openSearchInput() (tea.Model, tea.Cmd) {
	m.screen = scrSearchInput
	m.searchErr = ""
	m.searchResults = nil
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
	var ctx context.Context
	m, ctx = m.nextOpCtx()
	m.screen = scrSearchFetch
	return m, tea.Batch(searchYouTubeCmd(m.env, ctx, query, m.opGen), spinnerTickCmd())
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
	m = m.cancelOps()
	m.screen = scrURL
	m.searchErr = ""
	m.searchResults = nil
	m.searchInput.Blur()
	return m, m.urlInput.Focus()
}

func (m Model) gotoChecks() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps(m.env)
	m.deps = deps
	if deps.MissingRequired() {
		return m.openDependencyScreen(depModeStartup)
	}
	return m.gotoURLWithDeps(deps)
}

func (m Model) gotoURLWithDeps(deps app.CheckDepsResult) (tea.Model, tea.Cmd) {
	m.deps = deps
	m.screen = scrURL
	return m, m.urlInput.Focus()
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
	return m, refreshDepsCmd(m.env, m.depRefreshToken)
}

func (m Model) returnFromDependencyScreen() (tea.Model, tea.Cmd) {
	if m.depMode == depModeStartup {
		m.depErr = ""
		if !m.deps.MissingRequired() {
			return m.gotoURLWithDeps(app.DetectDeps(m.env))
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

func (m Model) startDependencyDownload(
	screen screen,
	label string,
	isUpdate bool,
	fn func(context.Context, chan<- app.FileProgress) error,
) (tea.Model, tea.Cmd) {
	m.depLabel = label
	m.depProgress = app.FileProgress{}
	m.depErr = ""
	m.screen = screen

	var cmd tea.Cmd
	m.depGen++
	m.depCh, cmd, m.depCancel = launchProgress(m.baseCtx, fn, isUpdate, m.depGen)
	return m, cmd
}

func (m Model) startQualityScan() (tea.Model, tea.Cmd) {
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.profile = app.OutputProfile{}
	m.flowErr = ""
	var ctx context.Context
	m, ctx = m.nextOpCtx()
	m.screen = scrQualityFetch
	return m, tea.Batch(loadQualityChoicesCmd(m.env, ctx, m.qualityScanURLs(), m.opGen), spinnerTickCmd())
}

func (m Model) continueAfterProfileSelection() (tea.Model, tea.Cmd) {
	if len(m.dlEntries) > 1 {
		m.screen = scrWorkers
		m = m.syncMenu()
		return m, nil
	}
	return m.startDownload()
}

func (m Model) startDownload() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps(m.env)
	m.deps = deps

	switch {
	case !deps.YTDLP.Available:
		m.depReturnScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.YTDLP.Name))
	case app.ProfileRequiresFFmpeg(m.currentProfile(), m.fragment) && !deps.FFmpeg.Available:
		m.depReturnScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.FFmpeg.Name))
	}

	req := app.DownloadRequest{
		Target:        m.target,
		Profile:       m.currentProfile(),
		Fragment:      m.fragment,
		MediaDuration: m.mediaDuration,
		ForceSingle:   m.forceSingle,
		PlaylistInfo:  m.plInfo,
		Entries:       m.dlEntries,
		Workers:       max(m.numWorkers, 1),
		OutputDir:     m.env.DownloadsDir(),
		Locale:        m.locale,
	}
	if _, err := app.PrepareDownloadRequestWithDeps(m.env, req, deps); err != nil {
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
	m.dlCancelled = false

	ch := make(chan app.DlUpdate, 256)
	m.dlCh = ch
	m.dlGen++
	m.screen = scrDownload

	dlCtx, dlCancel := context.WithCancel(m.baseCtx)
	m.dlCancel = dlCancel

	req.Workers = workers
	app.StartDownloadRequestContext(m.env, dlCtx, req, deps, ch)
	return m, tea.Batch(listenDownloadCmd(ch, m.dlGen), timerTickCmd())
}

func (m Model) cancelDownload() (tea.Model, tea.Cmd) {
	if m.dlCancel != nil {
		m.dlCancel()
		m.dlCancel = nil
	}
	m.timerActive = false
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
