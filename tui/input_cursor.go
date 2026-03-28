package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

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
