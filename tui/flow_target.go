package tui

import (
	"strings"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handlePlaylistFetched(msg msgPlaylistFetched) (tea.Model, tea.Cmd) {
	if msg.err != nil || msg.info == nil {
		m.forceSingle = true
		return m.startModeSelection()
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
