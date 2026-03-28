package tui

import (
	"strings"

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
