package tui

import (
	"fmt"
	"strconv"
	"strings"
)

type menu struct {
	items  []string
	cursor int
}

func (m *menu) SetItems(items []string) {
	changed := !sameMenuItems(m.items, items)
	m.items = append(m.items[:0], items...)
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	if changed {
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

func (m menu) View(width int) string {
	rowWidth := fitWidth(width, menuW, 24)
	indexWidth := max(2, len(strconv.Itoa(len(m.items))))
	labelWidth := max(10, rowWidth-indexWidth-8)
	lines := make([]string, len(m.items))
	for i, item := range m.items {
		label := trunc(item, labelWidth)
		prefix := fmt.Sprintf("%*d", indexWidth, i+1)
		lead := renderMenuLead(false)
		indexStyle := sMenuIndex.Width(indexWidth)
		textStyle := sMenuText
		rowStyle := sMenuRow
		if i == m.cursor {
			lead = renderMenuLead(true)
			indexStyle = sMenuIndexAct.Width(indexWidth)
			textStyle = sMenuTextAct
			rowStyle = sMenuActive
		}
		row := lead + indexStyle.Render(prefix) + "  " + textStyle.Render(label)
		lines[i] = rowStyle.Render(row)
	}
	return strings.Join(lines, "\n")
}

func renderMenuLead(active bool) string {
	if active {
		return sMenuLeadAct.Render("› ")
	}
	return sMenuLead.Render("  ")
}

func sameMenuItems(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
