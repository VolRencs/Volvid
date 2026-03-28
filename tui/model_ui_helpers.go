package tui

import (
	"fmt"

	app "YouTubeBuild/internal/app"
)

func (m Model) qualityOptions() []string {
	return app.QualityChoiceLabels(m.qualityChoices, m.locale)
}

func (m Model) audioOptions() []string {
	return app.OutputProfileLabels(m.audioProfiles)
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
