package tui

import (
	"fmt"
	"strconv"
	"strings"

	app "volvid/internal/app"

	"charm.land/lipgloss/v2"
)

func (m Model) viewPlaylist() string {
	if m.plInfo == nil {
		return ""
	}

	var parts []string
	parts = append(parts, m.renderSectionBlock("", m.renderPlaylistItems()))
	if m.plInputMode {
		parts = append(parts,
			m.renderSectionBlock(m.u().PlEnterNums, renderInputField(m.plInput)),
		)
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) renderPlaylistItems() string {
	entries := m.playlistEntries()
	start := m.plTop
	end := min(len(entries), start+m.playlistViewportHeight())
	indexWidth := max(2, len(strconv.Itoa(len(entries))))
	rowWidth := m.cardBodyWidth()
	staticWidth := lipgloss.Width(sListLead.Render("  ")) + lipgloss.Width(iconDotOn) + indexWidth + 14
	titleWidth := max(1, min(m.playlistTitleWidth(), rowWidth-staticWidth))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := entries[i]
		duration := fmt.Sprintf("%8s", app.FormatDuration(entry.Duration))
		data := listRowData{
			index:    fmt.Sprintf("%*d", indexWidth, entry.Index),
			hasCheck: true,
			checked:  m.plSelected[entry.Index],
			active:   i == m.plCursor,
		}
		labelWidth := listLabelWidth(rowWidth, data) - lipgloss.Width(duration) - 2
		title := sPlTitle.Width(max(1, titleWidth)).Render(trunc(entry.Title, max(1, min(titleWidth, labelWidth))))
		data.label = title + "  " + sTableMeta.Render(duration)

		lines = append(lines, renderListRow(rowWidth, data))
	}
	return strings.Join(lines, "\n")
}
