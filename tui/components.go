package tui

import (
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type menu struct {
	items  []string
	cursor int
}

func (m *menu) SetItems(items []string) {
	m.items = append(m.items[:0], items...)
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = max(0, min(m.cursor, len(m.items)-1))
}

func (m *menu) Move(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor = max(0, min(m.cursor+delta, len(m.items)-1))
}

func (m menu) Index() int {
	if len(m.items) == 0 {
		return 0
	}
	return m.cursor
}

func (m menu) View() string {
	lines := make([]string, len(m.items))
	maxWidth := 0
	for i, item := range m.items {
		if i == m.cursor {
			lines[i] = sTitle.Render(" > ") + sBold.Render(item)
		} else {
			lines[i] = sDim.Render("   " + item)
		}
		maxWidth = max(maxWidth, lipgloss.Width(lines[i]))
	}

	rowStyle := lipgloss.NewStyle().Width(maxWidth).Align(lipgloss.Left)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(rowStyle.Render(line))
	}
	return b.String()
}

type inputField struct {
	target        inputTarget
	placeholder   string
	value         []rune
	cursor        int
	offset        int
	width         int
	charLimit     int
	focused       bool
	cursorVisible bool
	blinkTag      int
}

func newInput(target inputTarget, placeholder string, width, charLimit int) inputField {
	return inputField{
		target:      target,
		placeholder: placeholder,
		width:       width,
		charLimit:   charLimit,
	}
}

func (i *inputField) SetPlaceholder(s string) {
	i.placeholder = s
}

func (i *inputField) SetWidth(width int) {
	i.width = width
	i.ensureCursorVisible()
}

func (i *inputField) SetValue(s string) {
	i.value = sanitizeRunes([]rune(s))
	if i.charLimit > 0 && len(i.value) > i.charLimit {
		i.value = i.value[:i.charLimit]
	}
	i.cursor = len(i.value)
	i.offset = 0
	i.ensureCursorVisible()
}

func (i inputField) Value() string {
	return string(i.value)
}

func (i inputField) Focused() bool {
	return i.focused
}

func (i *inputField) Focus() tea.Cmd {
	i.focused = true
	i.cursorVisible = true
	i.blinkTag++
	return blinkInputCmd(i.target, i.blinkTag)
}

func (i *inputField) Blur() {
	i.focused = false
	i.cursorVisible = false
	i.blinkTag++
}

func blinkInputCmd(target inputTarget, tag int) tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg {
		return cursorBlinkMsg{target: target, tag: tag}
	})
}

func pasteClipboardCmd(_ inputTarget) tea.Cmd {
	return func() tea.Msg {
		return tea.ReadClipboard()
	}
}

func (i *inputField) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		if msg.target != i.target || msg.tag != i.blinkTag || !i.focused {
			return nil
		}
		i.cursorVisible = !i.cursorVisible
		return blinkInputCmd(i.target, i.blinkTag)
	case tea.PasteMsg:
		if !i.focused {
			return nil
		}
		return i.insertRunes([]rune(msg.Content))
	case tea.ClipboardMsg:
		if !i.focused {
			return nil
		}
		return i.insertRunes([]rune(msg.Content))
	case tea.KeyPressMsg:
		if !i.focused {
			return nil
		}
		return i.handleKey(msg)
	default:
		return nil
	}
}

func (i *inputField) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	beforeCursor := i.cursor
	beforeLen := len(i.value)

	switch msg.String() {
	case "left", "ctrl+b":
		if i.cursor > 0 {
			i.cursor--
		}
	case "alt+left", "ctrl+left", "alt+b":
		i.wordBackward()
	case "right", "ctrl+f":
		if i.cursor < len(i.value) {
			i.cursor++
		}
	case "alt+right", "ctrl+right", "alt+f":
		i.wordForward()
	case "home", "ctrl+a":
		i.cursor = 0
	case "end", "ctrl+e":
		i.cursor = len(i.value)
	case "backspace", "ctrl+h":
		if i.cursor > 0 {
			i.value = append(i.value[:i.cursor-1], i.value[i.cursor:]...)
			i.cursor--
		}
	case "alt+backspace", "ctrl+w":
		i.deleteWordBackward()
	case "delete", "ctrl+d":
		if i.cursor < len(i.value) {
			i.value = append(i.value[:i.cursor], i.value[i.cursor+1:]...)
		}
	case "alt+delete", "alt+d":
		i.deleteWordForward()
	case "ctrl+k":
		i.deleteAfterCursor()
	case "ctrl+v", "shift+insert":
		return pasteClipboardCmd(i.target)
	default:
		if msg.Text != "" {
			return i.insertRunes([]rune(msg.Text))
		}
	}

	if beforeCursor != i.cursor || beforeLen != len(i.value) {
		i.ensureCursorVisible()
		i.cursorVisible = true
		i.blinkTag++
		return blinkInputCmd(i.target, i.blinkTag)
	}
	return nil
}

func (i *inputField) insertRunes(runes []rune) tea.Cmd {
	runes = sanitizeRunes(runes)
	if len(runes) == 0 {
		return nil
	}

	if i.charLimit > 0 {
		room := i.charLimit - len(i.value)
		if room <= 0 {
			return nil
		}
		if len(runes) > room {
			runes = runes[:room]
		}
	}

	i.value = append(i.value[:i.cursor], append(runes, i.value[i.cursor:]...)...)
	i.cursor += len(runes)
	i.ensureCursorVisible()
	i.cursorVisible = true
	i.blinkTag++
	return blinkInputCmd(i.target, i.blinkTag)
}

func (i *inputField) ensureCursorVisible() {
	if i.width <= 1 {
		i.offset = 0
		return
	}

	maxVisible := i.width - 1
	if i.cursor < i.offset {
		i.offset = i.cursor
	}
	if i.cursor > i.offset+maxVisible {
		i.offset = i.cursor - maxVisible
	}
	i.offset = max(0, min(i.offset, len(i.value)))
}

func (i *inputField) deleteAfterCursor() {
	if i.cursor < 0 || i.cursor > len(i.value) {
		return
	}
	i.value = i.value[:i.cursor]
}

func (i *inputField) deleteWordBackward() {
	if i.cursor == 0 || len(i.value) == 0 {
		return
	}

	start := i.cursor
	for start > 0 && unicode.IsSpace(i.value[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(i.value[start-1]) {
		start--
	}

	i.value = append(i.value[:start], i.value[i.cursor:]...)
	i.cursor = start
}

func (i *inputField) deleteWordForward() {
	if i.cursor >= len(i.value) || len(i.value) == 0 {
		return
	}

	end := i.cursor
	for end < len(i.value) && unicode.IsSpace(i.value[end]) {
		end++
	}
	for end < len(i.value) && !unicode.IsSpace(i.value[end]) {
		end++
	}

	i.value = append(i.value[:i.cursor], i.value[end:]...)
}

func (i *inputField) wordBackward() {
	if i.cursor == 0 || len(i.value) == 0 {
		return
	}

	pos := i.cursor
	for pos > 0 && unicode.IsSpace(i.value[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(i.value[pos-1]) {
		pos--
	}
	i.cursor = pos
}

func (i *inputField) wordForward() {
	if i.cursor >= len(i.value) || len(i.value) == 0 {
		return
	}

	pos := i.cursor
	for pos < len(i.value) && unicode.IsSpace(i.value[pos]) {
		pos++
	}
	for pos < len(i.value) && !unicode.IsSpace(i.value[pos]) {
		pos++
	}
	i.cursor = pos
}

func (i inputField) View() string {
	start, end := i.visibleWindow()
	visible := i.value[start:end]
	cursor := i.cursor - start

	if len(i.value) == 0 {
		placeholder := i.placeholder
		if i.width > 0 {
			placeholder = trunc(placeholder, i.width-1)
		}
		if i.focused {
			return i.renderCursor(" ") + sDim.Render(placeholder)
		}
		return sDim.Render(placeholder)
	}

	var b strings.Builder
	for pos, r := range visible {
		cell := string(r)
		if i.focused && cursor == pos && i.cursorVisible {
			b.WriteString(i.renderCursor(cell))
			continue
		}
		b.WriteString(cell)
	}
	if i.focused && cursor == len(visible) {
		b.WriteString(i.renderCursor(" "))
	}
	return b.String()
}

func (i inputField) visibleWindow() (int, int) {
	if i.width <= 0 {
		return 0, len(i.value)
	}
	start := min(i.offset, len(i.value))
	end := min(len(i.value), start+i.width-1)
	return start, max(start, end)
}

func (i inputField) renderCursor(cell string) string {
	if cell == "" {
		cell = " "
	}
	return sCursor.Render(cell)
}

func sanitizeRunes(runes []rune) []rune {
	clean := make([]rune, 0, len(runes))
	for _, r := range runes {
		switch r {
		case '\n', '\r', '\t':
			clean = append(clean, ' ')
		default:
			if r >= 32 {
				clean = append(clean, r)
			}
		}
	}
	return clean
}
