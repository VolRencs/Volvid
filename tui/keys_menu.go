package tui

import (
	"context"
	"strconv"
	"volvid/internal/core"

	"volvid/internal/i18n"

	tea "charm.land/bubbletea/v2"
)

// handleMenuDigit jumps to a 1-based menu item and activates it. Menus never
// exceed nine entries, so a single digit is enough.
func (m Model) handleMenuDigit(digit string) (tea.Model, tea.Cmd) {
	n, err := strconv.Atoi(digit)
	if err != nil || n < 1 || n > len(m.menu.items) {
		return m, nil
	}
	m.menu.SetCursor(n - 1)
	return m.activateMenu()
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
			return m.startDependencyDownload(scrUpdateDl, "", true, func(ctx context.Context, ch chan<- core.FileProgress) error {
				return m.api.ApplyUpdate(ctx, m.locale, info, ch)
			})
		}
		return m.gotoChecks()
	case scrDepUpdate:
		return m.activateDependencyAction(idx)
	case scrPlaylistAsk:
		if idx == 0 {
			m.forceSingle = true
			return m.startFragmentFlow()
		}
		return m.startOpScreen(scrPlaylistFetch, func(ctx context.Context, gen int) tea.Cmd {
			return fetchPlaylistCmd(m.api, ctx, m.url, m.locale, gen)
		})
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
		return m.activateModeChoice(idx)
	case scrAudio:
		if idx < 0 || idx >= len(m.audioProfiles) {
			return m, nil
		}
		return m.unifyProfileChoice(m.audioProfiles[idx])
	case scrQuality:
		if idx < 0 || idx >= len(m.qualityChoices) {
			return m, nil
		}
		m.profile = i18n.QualityProfile(m.qualityChoices[idx], m.locale)
		m.videoProfiles = i18n.VideoOutputProfiles(m.profile, m.locale)
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
		return m.startTracksStep()
	case scrWorkers:
		m.numWorkers = idx + 1
		return m.startDownload()
	}
	return m, nil
}

// unifyProfileChoice applies a profile chosen from audio/video-output menus.
func (m Model) unifyProfileChoice(profile core.OutputProfile) (tea.Model, tea.Cmd) {
	m.profile = profile
	m.flowErr = ""
	return m.continueAfterProfileSelection()
}

func (m Model) activateDependencyAction(idx int) (tea.Model, tea.Cmd) {
	actions := m.depActions()
	if idx < 0 || idx >= len(actions) {
		return m, nil
	}
	return actions[idx].Run(m)
}
func (m Model) activateModeChoice(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 1:
		m.mode = core.ModeAudio
	case 2:
		m.mode = core.ModeThumbnail
	default:
		m.mode = core.ModeVideo
	}
	m.resetProfileSelection()

	switch m.mode {
	case core.ModeThumbnail:
		m.profile = i18n.ThumbnailOutputProfile(m.locale)
		return m.continueAfterProfileSelection()
	case core.ModeAudio:
		m.audioProfiles = i18n.AudioOutputProfiles(m.locale)
		m.screen = scrAudio
		m = m.syncMenu()
		return m, nil
	default:
		return m.startQualityScan()
	}
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
		fragment := core.DownloadFragment{StartAt: m.target.URLStartAt}
		if err := core.ValidateFragmentDuration(fragment, m.mediaDuration); err != nil {
			m.flowErr = i18n.FragmentURLStartOutOfBoundsText(m.locale, m.mediaDuration)
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
