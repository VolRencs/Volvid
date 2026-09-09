package tui

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"strconv"
	"strings"
	"volvid/internal/core"
)

func (m Model) viewHome() string {
	parts := []string{
		m.viewHomeInput(),
		m.viewDownloadsLocation(),
	}
	if section := m.viewHomeSession(); section != "" {
		parts = append(parts, section)
	}
	return strings.Join(compactSections(parts...), m.sectionGap())
}

func (m Model) viewHomeInput() string {
	title := sSectionTitle.Render(m.u().HomeInputTitle)
	return title + "\n" + renderInputField(m.urlInput)
}

func (m Model) viewDownloadsLocation() string {
	pathWidth := max(18, m.cardBodyWidth()-10)
	body := renderFileLink(trunc(m.api.DownloadsDir(), pathWidth))
	return m.renderSectionBlock(m.u().HomeOutputTitle, body)
}

func (m Model) viewHomeSession() string {
	if m.compactHomeLayout() && len(m.session.Items) == 0 {
		return ""
	}
	return m.sessionBlock(3)
}

// sessionBlock renders the shared session history section (limit<=0 = all).
func (m Model) sessionBlock(limit int) string {
	stats := renderStatusChip(m.u().HomeStatSuccess, strconv.Itoa(m.session.Success), true) + "  " +
		renderStatusChip(m.u().HomeStatFailed, strconv.Itoa(m.session.Failed), false)
	if len(m.session.Items) == 0 {
		return m.renderSectionBlock(m.u().HomeSessionTitle, stats+"\n"+sMeta.Render(m.u().HomeSessionEmpty))
	}

	items := m.session.Items
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}

	width := max(18, m.cardBodyWidth()/2)
	rows := make([]string, 0, len(items)+1)
	rows = append(rows, stats)
	for _, item := range items {
		icon := statusIcon(item.OK)
		rows = append(rows,
			icon+"  "+sValue.Render(trunc(item.Label, width))+"\n"+
				sMeta.Render(trunc(item.URL, width+14)),
		)
	}
	return m.renderSectionBlock(m.u().HomeSessionTitle, strings.Join(rows, "\n\n"))
}

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
	staticWidth := lipgloss.Width("  ") + lipgloss.Width(iconDotOn) + indexWidth + 14
	titleWidth := max(1, min(m.playlistTitleWidth(), rowWidth-staticWidth))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := entries[i]
		duration := fmt.Sprintf("%8s", core.FormatDuration(entry.Duration))
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
