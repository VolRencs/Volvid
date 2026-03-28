package tui

import tea "charm.land/bubbletea/v2"

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
	default:
		return activeInputState{}, false
	}
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
