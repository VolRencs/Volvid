package tui

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"
)

func (m Model) viewDependencyProgress(label string) string {
	u := m.u()
	var b strings.Builder

	b.WriteString(sDim.Render("  " + label))
	b.WriteString("\n\n")
	b.WriteString("  " + renderProgressBar(barW, m.depProgress.Pct) + "\n")
	b.WriteString("  " + sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		b.WriteString("  " + sBold.Render(app.FmtBytesFor(m.depProgress.DoneB, m.locale)))
		if m.depProgress.TotalB > 0 {
			b.WriteString(sDim.Render(" / " + app.FmtBytesFor(m.depProgress.TotalB, m.locale)))
		}
		if m.depProgress.Speed != "" {
			b.WriteString("  " + sTitle.Render(m.depProgress.Speed))
		}
	}
	b.WriteString("\n")

	if m.depErr != "" {
		b.WriteString("\n" + sErr.Render(u.ErrPrefix) + sDim.Render(m.depErr) + "\n")
	}
	return b.String()
}

func (m Model) viewPlaylist() string {
	u := m.u()
	info := m.plInfo
	total := len(info.Entries)

	var b strings.Builder
	b.WriteString(sBold.Render(trunc(info.Title, 46)) + sDim.Render(fmt.Sprintf(u.PlVideosFmt, total)) + "\n")
	b.WriteString("  " + sep(54) + "\n\n")
	b.WriteString(m.renderPlaylistItems() + "\n\n")

	if m.plInputMode {
		b.WriteString(sTitle.Render(u.PlEnterNums) + "\n")
		b.WriteString(renderInputField(m.plInput) + "\n")
		if m.plInputErr != "" {
			b.WriteString(renderErrorLine(m.plInputErr))
		}
		return b.String()
	}

	b.WriteString(sDim.Render(fmt.Sprintf(u.PlSelectedFmt, m.selectedPlaylistCount(), total)))
	if total > m.playlistViewportHeight() {
		start := m.plTop + 1
		end := min(total, m.plTop+m.playlistViewportHeight())
		b.WriteString(sDim.Render(fmt.Sprintf("  %d-%d/%d", start, end, total)))
	}
	b.WriteString("\n")
	if m.plInputErr != "" {
		b.WriteString(renderErrorLine(m.plInputErr) + "\n")
	}
	b.WriteString(m.hint(m.kbUp(), m.kbDown(), m.kbSpace(), m.kbAll(), m.kbSlash(), m.kbEnter()))
	return b.String()
}

func (m Model) renderPlaylistItems() string {
	if m.plInfo == nil {
		return ""
	}

	entries := m.playlistEntries()
	start := m.plTop
	end := min(len(entries), start+m.playlistViewportHeight())

	var b strings.Builder
	for i := start; i < end; i++ {
		entry := entries[i]
		current := i == m.plCursor

		prefix := "  "
		if current {
			prefix = sTitle.Render("> ")
		}

		check := sGray.Render("[ ]")
		if m.plSelected[entry.Index] {
			check = sOk.Render("[✔]")
		}

		number := sDim.Render(fmt.Sprintf("%4d", entry.Index))
		title := sPlTitle.Render(trunc(entry.Title, 40))
		if current {
			title = sNormal.Render(title)
		} else {
			title = sDim.Render(title)
		}

		duration := sDim.Render(app.FmtDuration(entry.Duration))
		b.WriteString(prefix + check + "  " + number + "  " + title + "  " + duration + "\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
