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
		fragment, err := m.api.ParseFragment(m.fragmentIn.Value(), m.mediaDuration)
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
	if len(m.subTracks) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "up":
		m = m.stepSubtitleCursor(-1)
	case "down":
		m = m.stepSubtitleCursor(1)
	case "space":
		m.toggleCurrentSubtitle()
		m.flowErr = ""
	case "a", "а":
		m.toggleAllSubtitles()
		m.flowErr = ""
	case "enter":
		return m.confirmSubtitleSelection()
	default:
		return m, nil
	}

	return m, nil
}

// confirmSubtitleSelection applies the checked languages: cursor on the
// "no subtitles" row or an empty checklist means off, otherwise the checked
// tracks (in listed order) are embedded.
func (m Model) confirmSubtitleSelection() (tea.Model, tea.Cmd) {
	m.profile.SubMode = core.SubOff
	m.profile.SubLangs = nil
	if m.subCursor != 0 {
		if langs := m.selectedSubtitleLangs(); len(langs) > 0 {
			m.profile.SubMode = core.SubEmbed
			m.profile.SubLangs = langs
		}
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

type activeInputState struct {
	field *inputField
	err   *string
}

func (m *Model) activeInputState() (activeInputState, bool) {
	switch {
	case m.screen == scrURL:
		return activeInputState{field: &m.urlInput, err: &m.urlErr}, true
	case m.screen == scrSearchInput:
		return activeInputState{field: &m.searchInput, err: &m.searchErr}, true
	case m.screen == scrPlaylist && m.plInputMode:
		return activeInputState{field: &m.plInput, err: &m.plInputErr}, true
	case m.screen == scrFragmentInput:
		return activeInputState{field: &m.fragmentIn, err: &m.fragmentErr}, true
	}
	return activeInputState{}, false
}
func (m *Model) pasteIntoActiveInput(content string) tea.Cmd {
	input, ok := m.activeInputState()
	if !ok {
		return nil
	}
	return m.pasteIntoInput(input, content)
}
func (m *Model) pasteIntoInput(input activeInputState, content string) tea.Cmd {
	var cmds []tea.Cmd
	if !input.field.Focused() {
		cmds = append(cmds, input.field.Focus())
	}

	before := input.field.Value()
	cmd := input.field.insertRunes([]rune(content))
	if input.field.Value() != before && input.err != nil {
		*input.err = ""
	}
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}
func (m *Model) updateActiveInput(msg tea.Msg) tea.Cmd {
	input, ok := m.activeInputState()
	if !ok {
		return nil
	}

	before := input.field.Value()
	cmd := input.field.Update(msg)
	if input.field.Value() != before && input.err != nil {
		*input.err = ""
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
	if len(m.audioTracks) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "up":
		m = m.stepAudioCursor(-1)
	case "down":
		m = m.stepAudioCursor(1)
	case "space":
		m.toggleCurrentAudioTrack()
		m.flowErr = ""
	case "a", "а":
		m.toggleAllAudioTracks()
		m.flowErr = ""
	case "enter":
		return m.confirmAudioTrackSelection()
	default:
		return m, nil
	}

	return m, nil
}

// confirmAudioTrackSelection applies the checked languages: cursor on the
// "original" row or an empty checklist means no override, otherwise the
// checked tracks (in listed order) are embedded alongside the default audio.
// Subtitles were resolved in the same batch, so this routes directly.
func (m Model) confirmAudioTrackSelection() (tea.Model, tea.Cmd) {
	m.profile.AudioLangs = nil
	if m.audioCursor != 0 {
		m.profile.AudioLangs = m.selectedAudioLangs()
	}
	m.flowErr = ""
	if m.subsOffered {
		m.screen = scrSubtitles
		m = m.syncMenu()
		return m, nil
	}
	return m.continueAfterProfileSelection()
}
