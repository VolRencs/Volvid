package tui

import (
	"strconv"
	"strings"
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
	body := renderFileLink(trunc(m.env.DownloadsDir(), pathWidth))
	return m.renderSectionBlock(m.u().HomeOutputTitle, body)
}

func (m Model) viewHomeSession() string {
	if m.compactHomeLayout() && len(m.session.Items) == 0 {
		return ""
	}

	stats := renderStatusChip(m.u().HomeStatSuccess, strconv.Itoa(m.session.Success), true) + "  " +
		renderStatusChip(m.u().HomeStatFailed, strconv.Itoa(m.session.Failed), false)
	if len(m.session.Items) == 0 {
		return m.renderSectionBlock(m.u().HomeSessionTitle, stats+"\n"+sMeta.Render(m.u().HomeSessionEmpty))
	}

	items := m.session.Items
	if len(items) > 3 {
		items = items[len(items)-3:]
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
