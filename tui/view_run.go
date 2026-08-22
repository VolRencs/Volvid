package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) viewDownload() string {
	u := m.u()
	var parts []string
	barWidth := m.progressBarWidth()

	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		pct := float64(done) / float64(m.dlTotal) * 100
		statsLine := sOk.Render(fmt.Sprintf("%.1f%%", pct)) + "  " +
			sOk.Render(fmt.Sprintf("%s %d", iconDotOn, m.dlDone)) + "  " +
			sErr.Render(fmt.Sprintf("%s %d", iconDotOn, m.dlFailed)) + "  " +
			sMeta.Render(fmt.Sprintf(u.QueueFmt, m.dlTotal-done)) + "  " +
			sMeta.Render(formatElapsed(m.dlElapsed))
		parts = append(parts, m.renderSectionBlock("", strings.Join([]string{
			renderProgressBar(barWidth, pct),
			statsLine,
		}, "\n")))
		for i, slot := range m.slots {
			if body := m.viewSlot(i, slot, true); body != "" {
				parts = append(parts, m.renderSectionBlock("", body))
			}
		}
		return strings.Join(parts, "\n\n")
	}

	if len(m.slots) > 0 {
		return m.renderSectionBlock("", m.viewSlot(0, m.slots[0], false))
	}
	return ""
}

func (m Model) viewSlot(index int, slot slotState, withBadge bool) string {
	u := m.u()

	indicator := sDim.Render(iconDotOff)
	switch {
	case slot.done:
		indicator = sOk.Render(iconDotOn)
	case slot.failed:
		indicator = sErr.Render(iconDotOn)
	case slot.title == "" && !slot.proc:
		return "" // idle slot: hidden entirely
	}

	prefix := indicator + "  "
	if withBadge {
		prefix += sLabel.Render(fmt.Sprintf("#%d", index+1)) + "  "
	}

	titleWidth := max(1, m.cardBodyWidth()-lipgloss.Width(prefix))
	title := trunc(slot.title, titleWidth)
	if strings.TrimSpace(title) == "" {
		title = u.Downloading
	}

	lines := []string{prefix + sValue.Render(sSlotTitle.Width(titleWidth).Render(title))}
	switch {
	case slot.done:
		lines = append(lines, renderProgressBar(m.progressBarWidth(), 100)+"  "+sOk.Render("100%"))
	case slot.failed:
		message := u.ErrSlot
		if strings.TrimSpace(slot.label) != "" {
			message = slot.label
		}
		lines = append(lines, sErr.Render(message))
	case slot.proc:
		lines = append(lines, sWarn.Render(slot.label))
	default:
		lines = append(lines,
			renderProgressBar(m.progressBarWidth(), slot.pct)+"  "+
				sOk.Render(fmt.Sprintf("%.1f%%", slot.pct))+"  "+
				fmtStats(m.locale, slot.doneB, slot.totalB, slot.speed),
		)
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewSummary() string {
	var parts []string

	if m.singleOK || m.dlDone > 0 {
		parts = append(parts, m.renderSectionBlock(m.u().SummaryLocation, renderFileLink(m.env.DownloadsDir())))
	}
	if m.dlTotal > 0 {
		counts := renderStatusChip(m.u().SummaryPlaylistTitle,
			fmt.Sprintf("%d/%d", m.dlDone, m.dlTotal), m.dlFailed == 0)
		failedChip := renderBadge(m.u().HomeStatFailed, fmt.Sprintf("%d", m.dlFailed))
		parts = append(parts, m.renderSectionBlock("",
			counts+"  "+failedChip+"  "+sMeta.Render(formatElapsed(m.dlElapsed)),
		))
	}

	if len(m.menu.items) > 0 {
		parts = append(parts, m.menu.View(m.menuWidth()))
	}

	if len(m.session.Items) > 0 {
		rows := make([]string, 0, len(m.session.Items))
		labelWidth := fitWidth(m.cardBodyWidth()/3, 24, 8)
		urlWidth := max(1, m.cardBodyWidth()-labelWidth-10)
		for _, item := range m.session.Items {
			icon := statusIcon(item.OK)
			rows = append(rows,
				fmt.Sprintf("%s %-*s  %s",
					icon, labelWidth, trunc(item.Label, labelWidth),
					sMeta.Render(trunc(item.URL, urlWidth))),
			)
		}
		parts = append(parts, m.renderSectionBlock(m.u().SessionHist, strings.Join(rows, "\n")))
	}

	return strings.Join(parts, "\n\n")
}
