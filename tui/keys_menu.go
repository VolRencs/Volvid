package tui

import (
	"context"
	"strconv"
	"volvid/internal/core"

	"volvid/internal/i18n"

	tea "charm.land/bubbletea/v2"
)

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
func (m Model) activateMenu() (tea.Model, tea.Cmd) {
	if len(m.menu.items) == 0 {
		return m, nil
	}
	if handler, ok := menuActions[m.screen]; ok {
		return handler(m, m.menu.Index())
	}
	return m, nil
}

// unifyProfileChoice applies a profile chosen from audio/video-output menus.
func (m Model) unifyProfileChoice(profile core.OutputProfile) (tea.Model, tea.Cmd) {
	m.profile = profile
	m.flowErr = ""
	return m.continueAfterProfileSelection()
}

var menuActions = map[screen]func(Model, int) (tea.Model, tea.Cmd){
	scrUpdateReady: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx == 0 {
			info := m.updateInfo
			return m.startDependencyDownload(scrUpdateDl, "", true, func(ctx context.Context, ch chan<- core.FileProgress) error {
				return m.api.ApplyUpdate(ctx, m.locale, info, ch)
			})
		}
		return m.gotoChecks()
	},
	scrDepUpdate: func(m Model, idx int) (tea.Model, tea.Cmd) {
		return m.activateDependencyAction(idx)
	},
	scrPlaylistAsk: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx == 0 {
			m.forceSingle = true
			return m.startFragmentFlow()
		}
		var ctx context.Context
		m, ctx = m.nextOpCtx()
		m.screen = scrPlaylistFetch
		return m, tea.Batch(fetchPlaylistCmd(m.api, ctx, m.url, m.locale, m.opGen), spinnerTickCmd())
	},
	scrSummary: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx == 0 {
			return m.resetForNext()
		}
		return m, tea.Quit
	},
	scrSearchResults: func(m Model, idx int) (tea.Model, tea.Cmd) {
		return m.activateSearchResult(idx)
	},
	scrFragmentChoice: func(m Model, idx int) (tea.Model, tea.Cmd) {
		return m.activateFragmentChoice(idx)
	},
	scrMode: func(m Model, idx int) (tea.Model, tea.Cmd) {
		return m.activateModeChoice(idx)
	},
	scrAudio: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx < 0 || idx >= len(m.audioProfiles) {
			return m, nil
		}
		return m.unifyProfileChoice(m.audioProfiles[idx])
	},
	scrQuality: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx < 0 || idx >= len(m.qualityChoices) {
			return m, nil
		}
		m.profile = i18n.QualityProfile(m.qualityChoices[idx], m.locale)
		m.videoProfiles = i18n.VideoOutputProfiles(m.profile, m.locale)
		m.flowErr = ""
		m.screen = scrVideoOutput
		m = m.syncMenu()
		return m, nil
	},
	scrVideoOutput: func(m Model, idx int) (tea.Model, tea.Cmd) {
		if idx < 0 || idx >= len(m.videoProfiles) {
			return m, nil
		}
		m.profile = m.videoProfiles[idx]
		m.flowErr = ""
		return m.startAudioTrackStep()
	},
	scrWorkers: func(m Model, idx int) (tea.Model, tea.Cmd) {
		m.numWorkers = idx + 1
		return m.startDownload()
	},
}

func (m Model) activateDependencyAction(idx int) (tea.Model, tea.Cmd) {
	actions := m.depActions()
	if idx < 0 || idx >= len(actions) {
		return m, nil
	}
	action := actions[idx]
	switch action.Kind {
	case depActionInstall:
		key := action.Key
		return m.startDependencyDownload(scrDepDl, key, false, func(ctx context.Context, ch chan<- core.FileProgress) error {
			return m.api.InstallDependency(ctx, key, m.locale, ch)
		})
	case depActionRefresh:
		return m.startDepsRefresh()
	case depActionContinue:
		return m.gotoURLWithDeps(m.api.DetectDeps())
	case depActionBack:
		return m.returnFromDependencyScreen()
	case depActionExit:
		return m, tea.Quit
	}
	return m, nil
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
	m.profile = core.OutputProfile{}
	m.qualityChoices = nil
	m.audioProfiles = nil
	m.subTracks = nil
	m.subsOffered = false
	m.audioTracks = nil
	m.audioOffered = false
	m.audioCursor = 0
	m.audioTop = 0
	m.audioSelected = nil
	m.subCursor = 0
	m.subTop = 0
	m.subSelected = nil
	m.flowErr = ""

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
		if err := m.api.ValidateFragment(fragment, m.mediaDuration); err != nil {
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
