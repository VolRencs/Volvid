package tui

import (
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

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
	i.width = max(1, width)
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

func (i *inputField) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		if msg.target != i.target || msg.tag != i.blinkTag || !i.focused {
			return nil
		}
		i.cursorVisible = !i.cursorVisible
		return blinkInputCmd(i.target, i.blinkTag)
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

	if isClipboardPasteKey(msg) {
		return pasteClipboardCmd()
	}

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
	default:
		if msg.Text != "" {
			return i.insertRunes([]rune(msg.Text))
		}
	}

	if beforeCursor != i.cursor || beforeLen != len(i.value) {
		return i.touch()
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
	return i.touch()
}

func (i *inputField) ensureCursorVisible() {
	if i.width <= 1 {
		i.offset = 0
		return
	}

	maxVisible := i.width
	if i.cursor < i.offset {
		i.offset = i.cursor
	}
	if i.cursor >= i.offset+maxVisible {
		i.offset = i.cursor - maxVisible + 1
	}
	i.offset = max(0, min(i.offset, len(i.value)))
}

func (i *inputField) deleteAfterCursor() {
	if i.cursor < 0 || i.cursor > len(i.value) {
		return
	}
	i.value = i.value[:i.cursor]
}

func (i *inputField) wordBoundaryBackward() int {
	start := i.cursor
	for start > 0 && unicode.IsSpace(i.value[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(i.value[start-1]) {
		start--
	}
	return start
}

func (i *inputField) wordBoundaryForward() int {
	end := i.cursor
	for end < len(i.value) && unicode.IsSpace(i.value[end]) {
		end++
	}
	for end < len(i.value) && !unicode.IsSpace(i.value[end]) {
		end++
	}
	return end
}

func (i *inputField) deleteWordBackward() {
	start := i.wordBoundaryBackward()
	if start == i.cursor {
		return
	}
	i.value = append(i.value[:start], i.value[i.cursor:]...)
	i.cursor = start
}

func (i *inputField) deleteWordForward() {
	end := i.wordBoundaryForward()
	if end == i.cursor {
		return
	}
	i.value = append(i.value[:i.cursor], i.value[end:]...)
}

func (i *inputField) wordBackward() {
	i.cursor = i.wordBoundaryBackward()
}

func (i *inputField) wordForward() {
	i.cursor = i.wordBoundaryForward()
}

func (i inputField) View() string {
	start, end := i.visibleWindow()
	visible := i.value[start:end]
	cursor := i.cursor - start

	if len(i.value) == 0 {
		placeholder := i.placeholder
		if i.width > 0 {
			placeholder = trunc(placeholder, max(1, i.width-1))
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
	if i.focused && i.cursorVisible && cursor >= len(visible) && len(visible) < i.width {
		b.WriteString(i.renderCursor(" "))
	}
	return b.String()
}

const cursorBlinkInterval = 530 * time.Millisecond

func blinkInputCmd(target inputTarget, tag int) tea.Cmd {
	return tea.Tick(cursorBlinkInterval, func(time.Time) tea.Msg {
		return cursorBlinkMsg{target: target, tag: tag}
	})
}

func pasteClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.ReadClipboard()
	}
}

func (i inputField) visibleWindow() (int, int) {
	if i.width <= 0 {
		return 0, len(i.value)
	}
	start := min(i.offset, len(i.value))
	end := min(len(i.value), start+max(1, i.width))
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

func (i *inputField) touch() tea.Cmd {
	i.ensureCursorVisible()
	i.cursorVisible = true
	i.blinkTag++
	return blinkInputCmd(i.target, i.blinkTag)
}

func isClipboardPasteKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "ctrl+v", "ctrl+shift+v", "shift+ctrl+v", "shift+insert":
		return true
	}
	return msg.Text == string(rune(22))
}
