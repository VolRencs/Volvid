package tui

import (
	"volvid/internal/core"
	"volvid/internal/i18n"

	tea "charm.land/bubbletea/v2"
)

type inputTarget uint8

const (
	inputURL inputTarget = iota
	inputSearch
	inputPlaylist
	inputFragment
)

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
		fragment, err := core.ParseBoundedFragmentRange(m.fragmentIn.Value(), m.mediaDuration)
		if err != nil {
			m.fragmentErr = i18n.FragmentInputErrorText(m.locale, err, m.mediaDuration)
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
func (m Model) handleSubtitlesKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	return handleChecklistKey(&m, msg, &m.subList, m.subTracks, m.confirmSubtitleSelection)
}

// handleChecklistKey drives any language checklist: cursor moves, single
// toggle, all/none toggle and Enter to confirm.
func handleChecklistKey[T any](
	m *Model,
	msg tea.KeyPressMsg,
	list *checklist[T],
	tracks []T,
	confirm func() (tea.Model, tea.Cmd),
) (tea.Model, tea.Cmd) {
	if len(tracks) == 0 {
		return *m, nil
	}

	switch msg.String() {
	case "up":
		list.move(-1, tracks, m.playlistViewportHeight())
	case "down":
		list.move(1, tracks, m.playlistViewportHeight())
	case "space":
		list.toggle(tracks)
		m.flowErr = ""
	case "a", "а":
		list.toggleAll(tracks)
		m.flowErr = ""
	case "enter":
		return confirm()
	default:
		return *m, nil
	}

	return *m, nil
}

// confirmSubtitleSelection applies the checked languages: an empty checklist
// means no subtitles are added.
func (m Model) confirmSubtitleSelection() (tea.Model, tea.Cmd) {
	m.profile.SubMode = core.SubOff
	m.profile.SubLangs = nil
	if langs := m.subList.selectedKeys(m.subTracks); len(langs) > 0 {
		m.profile.SubMode = core.SubEmbed
		m.profile.SubLangs = langs
	}
	m.flowErr = ""
	return m.continueAfterProfileSelection()
}
func (m Model) handlePlaylistInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		indices, err := m.api.ParseSelection(m.plInput.Value(), len(m.plInfo.Entries), m.locale)
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

// withActiveInput resolves the input field focused on the current screen and
// applies fn to it and its error slot. It reports whether a field exists.
func (m *Model) withActiveInput(fn func(field *inputField, err *string)) bool {
	var field *inputField
	var err *string
	switch {
	case m.screen == scrURL:
		field, err = &m.urlInput, &m.urlErr
	case m.screen == scrSearchInput:
		field, err = &m.searchInput, &m.searchErr
	case m.screen == scrPlaylist && m.plInputMode:
		field, err = &m.plInput, &m.plInputErr
	case m.screen == scrFragmentInput:
		field, err = &m.fragmentIn, &m.fragmentErr
	default:
		return false
	}
	fn(field, err)
	return true
}
func (m *Model) pasteIntoActiveInput(content string) tea.Cmd {
	var cmds []tea.Cmd
	ok := m.withActiveInput(func(field *inputField, err *string) {
		if !field.Focused() {
			cmds = append(cmds, field.Focus())
		}
		before := field.Value()
		cmds = append(cmds, field.insertRunes([]rune(content)))
		if field.Value() != before && err != nil {
			*err = ""
		}
	})
	if !ok {
		return nil
	}
	return tea.Batch(cmds...)
}
func (m *Model) updateActiveInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	ok := m.withActiveInput(func(field *inputField, err *string) {
		before := field.Value()
		cmd = field.Update(msg)
		if field.Value() != before && err != nil {
			*err = ""
		}
	})
	if !ok {
		return nil
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

func (m Model) handleAudioTrackKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	return handleChecklistKey(&m, msg, &m.audioList, m.audioTracks, m.confirmAudioTrackSelection)
}

// confirmAudioTrackSelection applies the checked languages: an empty
// checklist means no override, i.e. the original audio only. Subtitles were
// resolved in the same batch, so this routes directly.
func (m Model) confirmAudioTrackSelection() (tea.Model, tea.Cmd) {
	m.profile.AudioLangs = nil
	if langs := m.audioList.selectedKeys(m.audioTracks); len(langs) > 0 {
		m.profile.AudioLangs = langs
	}
	m.flowErr = ""
	if m.subsOffered {
		m.screen = scrSubtitles
		m = m.syncMenu()
		return m, nil
	}
	return m.continueAfterProfileSelection()
}
