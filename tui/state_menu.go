package tui

import (
	"fmt"
	"strings"

	app "volvid/internal/app"

	tea "charm.land/bubbletea/v2"
)

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

func (m Model) workerMenuOptions(n int) []string {
	u := m.u()
	if n <= 0 {
		return nil
	}
	opts := make([]string, n)
	opts[0] = u.WorkerSeq
	for i := 1; i < n; i++ {
		opts[i] = fmt.Sprintf(u.WorkerNFmt, i+1)
	}
	return opts
}

func (m Model) fragmentChoiceOptions() []string {
	u := m.u()
	options := []string{u.MenuFullVideo}
	if m.canUseURLStartFragment() {
		options = append(options, fmt.Sprintf("%s (%s)", u.MenuFromURLStart, app.FormatClockTimestamp(m.target.URLStartAt)))
	}
	return append(options, u.MenuManualRange)
}

func (m Model) searchResultOptions() []string {
	options := make([]string, 0, len(m.searchResults))
	for i, result := range m.searchResults {
		label := strings.TrimSpace(result.Title)
		if label == "" {
			label = fmt.Sprintf(m.u().VideoTitleFmt, i+1)
		}
		if result.Duration > 0 {
			label += "  ·  " + app.FormatDuration(result.Duration)
		}
		options = append(options, label)
	}
	return options
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

func (m Model) syncMenu() Model {
	m.menuDigits = ""
	m.menu.SetItems(m.menuItems())
	return m
}

func (m Model) gotoScreen(s screen) Model {
	m.screen = s
	return m.syncMenu()
}

func (m *Model) syncLocalizedInputs() {
	m.searchInput.SetPlaceholder(m.u().SearchPlaceholder)
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
	m.menu.SetItems(m.menuItems())
	m.syncLayout()
}

func (m Model) depActions() []depAction {
	actions := make([]depAction, 0, 5)
	for _, dep := range m.deps.ActionableDependencies() {
		label := depActionText(depActionInstall, dep.Name, dep.Available && dep.Source == app.DepManaged, m.u())
		actions = append(actions, depAction{Kind: depActionInstall, Key: dep.Key, Label: label})
	}

	if !m.depRefreshing {
		actions = append(actions, depAction{Kind: depActionRefresh, Label: depActionText(depActionRefresh, "", false, m.u())})
	}
	if m.depMode == depModeStartup && !m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionContinue, Label: depActionText(depActionContinue, "", false, m.u())})
	}
	if m.depMode == depModeManage {
		actions = append(actions, depAction{Kind: depActionBack, Label: depActionText(depActionBack, "", false, m.u())})
	} else if m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionExit, Label: depActionText(depActionExit, "", false, m.u())})
	}
	return actions
}

func depActionText(kind depActionKind, name string, isUpdate bool, u *app.UIStrings) string {
	if kind == depActionInstall {
		if isUpdate {
			return fmt.Sprintf(u.DepActionUpdateFmt, name)
		}
		return fmt.Sprintf(u.DepActionDownloadFmt, name)
	}
	switch kind {
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

func (m Model) uiBusy() bool {
	return m.screen.props().busy
}

func (m Model) isAppUpdateScreen() bool {
	return m.screen.props().updating
}

func (m Model) canOpenDependencyScreen() bool {
	return !m.uiBusy() && !m.isAppUpdateScreen()
}

func (m Model) canOpenDownloadsFolder() bool {
	if m.screen == scrURL {
		return true
	}
	return m.screen == scrSummary && (m.singleOK || m.dlDone > 0)
}

func (m Model) canPickDownloadsFolder() bool {
	return m.screen == scrURL
}

func (m Model) isMenuScreen() bool {
	return m.screen.props().menu
}

func (m Model) canUseURLStartFragment() bool {
	return m.target.HasURLStart && m.target.URLStartAt > 0 && m.mediaDuration > 0 && m.target.URLStartAt < m.mediaDuration
}

func (m Model) selectedPlaylistCount() int {
	return len(m.plSelected)
}

func (m *Model) clearPlaylistSelection() {
	clear(m.plSelected)
	if m.plSelected == nil {
		m.plSelected = map[int]bool{}
	}
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

func (m Model) currentProfile() app.OutputProfile {
	if m.profile.Mode != 0 {
		return m.profile
	}
	return app.DefaultProfileForMode(m.mode, m.locale)
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
