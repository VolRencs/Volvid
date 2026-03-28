package tui

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"
	"charm.land/lipgloss/v2"
)

func (m Model) viewDownload() string {
	u := m.u()
	var b strings.Builder

	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		pct := float64(done) / float64(m.dlTotal) * 100

		b.WriteString(sBold.Render(fmt.Sprintf(u.PlaylistBarFmt, m.dlTotal)) + "\n")
		b.WriteString("  " + renderProgressBar(barW, pct) + "\n")
		b.WriteString(
			"  " +
				sOk.Render(fmt.Sprintf("%.1f%%", pct)) +
				sOk.Render(fmt.Sprintf(" %d", m.dlDone)) +
				sDim.Render(fmt.Sprintf("/%d", m.dlTotal)) +
				"\n\n",
		)

		for i, slot := range m.slots {
			b.WriteString(m.viewSlot(i, slot, true))
		}

		b.WriteString(
			"\n  " +
				sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)) +
				"  " +
				sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)) +
				"  " +
				sDim.Render(fmt.Sprintf(u.QueueFmt, m.dlTotal-done)) +
				"  " +
				sDim.Render(formatElapsed(m.dlElapsed)) +
				"\n",
		)

		return b.String()
	}

	if len(m.slots) > 0 {
		b.WriteString(sTitle.Render(u.Downloading) + "\n\n")
		b.WriteString(m.viewSlot(0, m.slots[0], false))
	}
	return b.String()
}

func (m Model) viewSlot(index int, slot slotState, withBadge bool) string {
	u := m.u()
	var indicator string

	switch {
	case slot.done:
		indicator = sOk.Render("●")
	case slot.failed:
		indicator = sErr.Render("●")
	case slot.proc, slot.title != "":
		indicator = sTitle.Render("●")
	default:
		indicator = sTitle.Render(m.spinnerView())
	}

	prefix := "  " + indicator + "  "
	if withBadge {
		prefix += sDim.Render(fmt.Sprintf("[%d]", index+1)) + "  "
	}
	indent := strings.Repeat(" ", lipgloss.Width(prefix))

	if slot.title == "" && !slot.done && !slot.failed && !slot.proc && slot.pct <= 0 && slot.doneB <= 0 {
		return prefix + sDim.Render(u.Waiting) + "\n"
	}

	titleText := trunc(slot.title, 46)
	if strings.TrimSpace(titleText) == "" {
		titleText = u.Downloading
	}
	titleRaw := sSlotTitle.Render(titleText)
	title := sDim.Render(titleRaw)
	if slot.done {
		title = sNormal.Render(titleRaw)
	}

	row1 := prefix + title
	switch {
	case slot.done:
		return row1 + "  " + sOk.Render("✔") + "\n" + indent + renderProgressBar(barW, 100) + "\n"
	case slot.failed:
		message := u.ErrSlot
		if strings.TrimSpace(slot.label) != "" {
			message = slot.label
		}
		return row1 + "\n" + indent + sErr.Render(message) + "\n"
	case slot.proc:
		return row1 + "\n" + indent + sWarn.Render("⚙ ") + sDim.Render(slot.label) + "\n"
	default:
		return row1 + "\n" + indent + renderProgressBar(barW, slot.pct) + "  " + sOk.Render(fmt.Sprintf("%.1f%%", slot.pct)) + "  " + fmtStats(m.locale, slot.doneB, slot.totalB, slot.speed) + "\n"
	}
}

func (m Model) viewSummary() string {
	u := m.u()
	var b strings.Builder

	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render(u.SummaryOK) + "\n" + sDim.Render("     → "+app.DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render(u.SummaryFail) + "\n")
			if strings.TrimSpace(m.downloadErr) != "" {
				b.WriteString(sDim.Render("     "+m.downloadErr) + "\n")
			}
			b.WriteString("\n")
		}
	} else {
		icon := sOk.Render("✔")
		if m.dlFailed > 0 {
			icon = sWarn.Render("!")
		}
		b.WriteString(
			"  " +
				icon +
				"  " +
				sBold.Render(u.SummaryPlaylistTitle) +
				"  " +
				sOk.Render(fmt.Sprintf("%d", m.dlDone)) +
				sDim.Render(fmt.Sprintf(u.SuccessFmt, m.dlTotal)) +
				"\n\n",
		)
	}

	if len(m.session.Items) > 0 {
		b.WriteString(sDim.Render(u.SessionHist) + "\n")
		b.WriteString("  " + sep(54) + "\n")
		for _, item := range m.session.Items {
			icon := sOk.Render("✔")
			if !item.OK {
				icon = sErr.Render("✘")
			}
			b.WriteString(fmt.Sprintf(
				"  %s  %-26s  %s\n",
				icon,
				trunc(item.Label, 26),
				sDim.Render(trunc(item.URL, 30)),
			))
		}
		b.WriteString("  " + sep(54) + "\n")
		b.WriteString(fmt.Sprintf(
			"  %s  %s\n\n",
			sOk.Render(fmt.Sprintf("✔ %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("✘ %d", m.session.Failed)),
		))
	}

	b.WriteString(m.menuAndNav())
	return b.String()
}
