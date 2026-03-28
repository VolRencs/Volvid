package tui

import (
	"fmt"

	app "YouTubeBuild/internal/app"
)

func (m Model) qualityOptions() []string {
	return app.QualityChoiceLabels(m.qualityChoices, m.locale)
}

func (m Model) qualityProfileAt(idx int) app.OutputProfile {
	if idx < 0 || idx >= len(m.qualityChoices) {
		return app.OutputProfile{}
	}
	return m.qualityChoices[idx].Profile(m.locale)
}

func (m Model) audioOptions() []string {
	return app.OutputProfileLabels(m.audioProfiles)
}

func (m Model) audioProfileAt(idx int) app.OutputProfile {
	if idx < 0 || idx >= len(m.audioProfiles) {
		return app.OutputProfile{}
	}
	return m.audioProfiles[idx]
}

func (m Model) modeAt(idx int) app.DownloadMode {
	switch idx {
	case 1:
		return app.ModeAudio
	case 2:
		return app.ModeThumbnail
	default:
		return app.ModeVideo
	}
}

func (m Model) searchResultAt(idx int) app.SearchResult {
	if idx < 0 || idx >= len(m.searchResults) {
		return app.SearchResult{}
	}
	return m.searchResults[idx]
}

func (m Model) shouldAskWorkers() bool {
	return len(m.dlEntries) > 1
}

func (m Model) modeOptions() []string {
	u := m.u()
	return []string{u.ModeVideo, u.ModeAudio, u.ModeThumbnail}
}

func (m Model) sessionPlaylistSuffix(n int) string {
	return app.PlaylistSuffix(m.locale, n)
}

func (m Model) uiBusy() bool {
	switch m.screen {
	case scrUpdateDl, scrDepDl, scrDepUpdate, scrDownload, scrPlaylistFetch, scrFragmentProbe, scrQualityFetch, scrSearchFetch:
		return true
	}
	return false
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
	case scrFFmpegAsk:
		return []string{u.MenuFFmpegY, u.MenuFFmpegN}
	case scrPlaylistAsk:
		return []string{u.MenuVidOnly, u.MenuOpenPl}
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
	case scrWorkers:
		return m.workerMenuOptions(min(len(m.dlEntries), 5))
	default:
		return nil
	}
}

func (m *Model) syncLocalizedInputs() {
	m.searchInput.SetPlaceholder(m.u().SearchPlaceholder)
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
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
	for _, result := range m.searchResults {
		label := trunc(result.Title, 42)
		if result.Duration > 0 {
			label += "  " + app.FmtDuration(result.Duration)
		}
		options = append(options, label)
	}
	return options
}
