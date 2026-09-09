package tui

import (
	"context"
	"errors"
	"strings"
	"time"
	"volvid/internal/core"

	"volvid/internal/services"

	tea "charm.land/bubbletea/v2"
)

func (m Model) gotoQualitySelection() (tea.Model, tea.Cmd) {
	m = m.gotoScreen(scrQuality)
	return m, nil
}
func (m Model) gotoWorkersBack() (tea.Model, tea.Cmd) {
	switch m.mode {
	case core.ModeAudio:
		m.screen = scrAudio
	case core.ModeThumbnail:
		m.screen = scrMode
	default:
		m.screen = scrVideoOutput
	}
	m = m.syncMenu()
	return m, nil
}
func (m Model) startModeSelectionWithNotice(notice string) (tea.Model, tea.Cmd) {
	m.mode = core.ModeVideo
	m.profile = m.defaultVideoProfile()
	m.flowErr = notice
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.audioProfiles = nil
	m = m.gotoScreen(scrMode)
	return m, nil
}
func (m Model) startOpenDownloadsDir() (tea.Model, tea.Cmd) {
	if m.screen == scrURL {
		m.urlErr = ""
	}
	return m, openDownloadsDirCmd(m.api, m.api.DownloadsDir())
}
func (m Model) startPickDownloadsDir() (tea.Model, tea.Cmd) {
	m.urlErr = ""
	if m.pickCancel != nil {
		m.pickCancel()
	}
	m.pickGen++
	ctx, cancel := context.WithCancel(m.baseCtx)
	m.pickCancel = cancel
	return m, pickDownloadsDirCmd(ctx, m.api, m.api.DownloadsDir(), m.locale, m.pickGen)
}
func (m Model) submitURLInput() (tea.Model, tea.Cmd) {
	rawURL := strings.TrimSpace(m.urlInput.Value())
	if rawURL == "" {
		m.urlErr = m.u().URLErrEmpty
		return m, nil
	}

	target, err := m.api.ParseTarget(rawURL)
	if err != nil {
		m.urlErr = m.u().URLErrBad + ": " + err.Error()
		return m, nil
	}

	m.urlErr = ""
	return m.startTargetFlow(rawURL, target)
}
func (m Model) startOpScreen(s screen, cmd func(ctx context.Context, gen int) tea.Cmd) (tea.Model, tea.Cmd) {
	var ctx context.Context
	m, ctx = m.nextOpCtx()
	m.screen = s
	return m, tea.Batch(cmd(ctx, m.opGen), spinnerTickCmd())
}
func (m Model) startTargetFlow(rawURL string, target core.ParsedTarget) (tea.Model, tea.Cmd) {
	m.url = rawURL
	m.urlInput.SetValue(rawURL)
	m.target = target
	m.resetTargetFlowState()

	if target.IsPlaylist() {
		if target.Kind == core.TargetMixed {
			m = m.gotoScreen(scrPlaylistAsk)
			return m, nil
		}
		return m.startOpScreen(scrPlaylistFetch, func(ctx context.Context, gen int) tea.Cmd {
			return fetchPlaylistCmd(m.api, ctx, rawURL, m.locale, gen)
		})
	}

	return m.startFragmentFlow()
}
func (m Model) startFragmentFlow() (tea.Model, tea.Cmd) {
	m.resetFragmentState()
	return m.startOpScreen(scrFragmentProbe, func(ctx context.Context, gen int) tea.Cmd {
		return probeFragmentDurationCmd(m.api, ctx, m.target, gen)
	})
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
	return m.startOpScreen(scrSearchFetch, func(ctx context.Context, gen int) tea.Cmd {
		return searchYouTubeCmd(m.api, ctx, query, gen)
	})
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

	target, err := m.api.ParseTarget(result.URL)
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
	deps := m.api.DetectDeps()
	m.deps = deps
	if deps.MissingRequired() {
		return m.openDependencyScreen(depModeStartup)
	}
	return m.gotoURLWithDeps(deps)
}
func (m Model) gotoURLWithDeps(deps core.CheckDepsResult) (tea.Model, tea.Cmd) {
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
	return m, refreshDepsCmd(m.api, m.depRefreshToken)
}
func (m Model) returnFromDependencyScreen() (tea.Model, tea.Cmd) {
	if m.depMode == depModeStartup {
		m.depErr = ""
		if !m.deps.MissingRequired() {
			return m.gotoURLWithDeps(m.api.DetectDeps())
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
	fn func(context.Context, chan<- core.FileProgress) error,
) (tea.Model, tea.Cmd) {
	m.depLabel = label
	m.depProgress = core.FileProgress{}
	m.depErr = ""
	m.screen = screen

	var cmd tea.Cmd
	if m.depCancel != nil {
		m.depCancel()
	}
	m.depGen++
	m.depCh, cmd, m.depCancel = launchProgress(m.baseCtx, fn, isUpdate, m.depGen)
	return m, cmd
}
func (m Model) startQualityScan() (tea.Model, tea.Cmd) {
	m.qualityChoices = nil
	m.videoProfiles = nil
	m.profile = core.OutputProfile{}
	m.flowErr = ""
	urls := m.qualityScanURLs()
	return m.startOpScreen(scrQualityFetch, func(ctx context.Context, gen int) tea.Cmd {
		return loadQualityChoicesCmd(m.api, ctx, urls, gen)
	})
}
func (m Model) continueAfterProfileSelection() (tea.Model, tea.Cmd) {
	if len(m.dlEntries) > 1 {
		m = m.gotoScreen(scrWorkers)
		return m, nil
	}
	return m.startDownload()
}

// startDownload now delegates validation/planning to services.PlanDownload.
// UI only maps the plan onto slots/channels and opens the dep screen on
// MissingDependencyError.
func (m Model) startDownload() (tea.Model, tea.Cmd) {
	deps := m.api.DetectDeps()
	m.deps = deps

	plan, err := services.PlanDownload(
		deps,
		m.target,
		m.currentProfile(),
		m.fragment,
		m.mediaDuration,
		m.forceSingle,
		m.plInfo,
		m.dlEntries,
		m.numWorkers,
		m.api.DownloadsDir(),
		m.locale,
		m.api.PrepareDownload,
	)
	if err != nil {
		var missing *services.MissingDependencyError
		if errors.As(err, &missing) {
			m.depReturnScreen = m.screen
			return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(missing.Name))
		}
		m.flowErr = err.Error()
		m.restoreDownloadConfigScreen()
		m = m.syncMenu()
		return m, nil
	}

	m.slots = make([]slotState, plan.Workers)
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = plan.Total
	m.singleOK = false
	m.downloadErr = ""
	m.dlStartedAt = time.Now()
	m.dlElapsed = 0
	m.timerActive = true
	m.dlCancelled = false

	ch := make(chan core.DlUpdate, 256)
	m.dlCh = ch
	m.dlGen++
	m.screen = scrDownload

	dlCtx, dlCancel := context.WithCancel(m.baseCtx)
	m.dlCancel = dlCancel

	m.api.StartDownload(dlCtx, plan.Request, plan.Deps, ch)
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
	case core.ModeAudio:
		if m.profile.Mode == 0 {
			m.screen = scrAudio
		}
	case core.ModeThumbnail:
		m.screen = scrMode
	default:
		if m.profile.Mode == 0 {
			m.screen = scrQuality
		}
	}
}
