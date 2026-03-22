package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type binding struct {
	key  string
	help string
}

func (m Model) View() tea.View {
	body := m.renderBody()
	screen := m.buildScreen(body)

	v := tea.NewView(screen)
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader · v" + app.Version
	v.Cursor = nil
	return v
}

func (m Model) kbUp() binding       { return binding{key: "↑/k", help: m.u().HelpUp} }
func (m Model) kbDown() binding     { return binding{key: "↓/j", help: m.u().HelpDown} }
func (m Model) kbEnter() binding    { return binding{key: "enter", help: m.u().HelpEnter} }
func (m Model) kbQuit() binding     { return binding{key: "ctrl+c", help: m.u().HelpQuit} }
func (m Model) kbUpdDeps() binding  { return binding{key: "ctrl+u", help: m.u().HelpDeps} }
func (m Model) kbSpace() binding    { return binding{key: "space", help: m.u().HelpSpace} }
func (m Model) kbAll() binding      { return binding{key: "a", help: m.u().HelpAll} }
func (m Model) kbSlash() binding    { return binding{key: "/", help: m.u().HelpSlash} }
func (m Model) kbAny() binding      { return binding{key: m.u().HelpAnyKey, help: m.u().HelpExit} }
func (m Model) spinnerView() string { return spinnerFrames[m.spinnerFrame%len(spinnerFrames)] }

func (m Model) hint(bindings ...binding) string {
	parts := make([]string, 0, len(bindings))
	for _, item := range bindings {
		parts = append(parts, item.key+" "+item.help)
	}
	return "\n" + sDim.Render(strings.Join(parts, "   "))
}

func (m Model) menuAndNav() string {
	return m.menu.View() + "\n" + m.hint(m.kbUp(), m.kbDown(), m.kbEnter())
}

func (m Model) renderBody() string {
	u := m.u()
	var b strings.Builder

	header := sHeader.Render(
		sTitle.Render("VolRen") +
			sDim.Render(u.AppSubtitle) +
			"\n" +
			sDim.Render(fmt.Sprintf(u.HeaderPowered, app.Version)),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	switch m.screen {
	case scrUpdateCheck, scrPlaylistFetch:
		msg := u.SpinnerUpdate
		if m.screen == scrPlaylistFetch {
			msg = u.SpinnerPlaylist
		}
		b.WriteString("  " + sTitle.Render(m.spinnerView()) + sDim.Render(msg))

	case scrUpdateReady, scrFFmpegAsk, scrPlaylistAsk:
		b.WriteString(m.renderPromptMenu())

	case scrUpdateDl, scrDepDl:
		label := m.depLabel
		if m.screen == scrUpdateDl && m.updateInfo != nil {
			label = fmt.Sprintf(u.DepLabelFmt, m.updateInfo.Latest)
		}
		b.WriteString(m.viewDependencyProgress(label))

	case scrUpdateDone:
		b.WriteString(sOk.Render(u.UpdateDonePrefix))
		if m.updateInfo != nil {
			b.WriteString(sBold.Render(m.updateInfo.Latest))
		}
		b.WriteString("\n\n")
		if app.IsWindows {
			b.WriteString(sDim.Render(u.UpdateAppliedWin))
		} else {
			b.WriteString(sDim.Render(u.UpdateAppliedUnix))
		}
		b.WriteString(m.hint(m.kbAny()))

	case scrDepUpdate:
		if m.depUpdateDone {
			b.WriteString(sOk.Render(u.DepsOK) + "\n\n")
			b.WriteString("  " + sGray.Render("yt-dlp  ") + sOk.Render(m.ytdlpVer) + "\n")
			if m.ffmpegVer != "" {
				b.WriteString("  " + sGray.Render("ffmpeg  ") + sOk.Render(m.ffmpegVer) + "\n")
			}
			b.WriteString(m.hint(m.kbEnter()))
		} else if m.depErr != "" {
			b.WriteString(sErr.Render(u.ErrPrefix) + sDim.Render(m.depErr) + "\n")
			b.WriteString(m.hint(m.kbEnter()))
		} else {
			b.WriteString(m.viewDependencyProgress(u.DepsUpdating))
		}

	case scrURL:
		b.WriteString(sBold.Render(u.PasteURL) + "\n\n")
		inputStyle := sInputBox
		if m.urlInput.Focused() {
			inputStyle = sInputBoxFocus
		}
		b.WriteString(inputStyle.Render(m.urlInput.View()) + "\n")
		if m.urlErr != "" {
			b.WriteString("\n" + sErr.Render("  ✘  "+m.urlErr) + "\n")
		} else {
			b.WriteString(sDim.Render(u.URLHints) + "\n")
		}
		b.WriteString(m.hint(m.kbEnter(), m.kbQuit(), m.kbUpdDeps()))

	case scrPlaylist:
		b.WriteString(m.viewPlaylist())

	case scrWorkers, scrQuality:
		if m.screen == scrWorkers {
			b.WriteString(
				sBold.Render(u.ParallelFmt) +
					sDim.Render(fmt.Sprintf(u.WorkersQueuedFmt, len(m.dlEntries))) +
					"\n\n",
			)
		} else {
			b.WriteString(sBold.Render(u.QualityTitle) + "\n\n")
		}
		b.WriteString(m.menuAndNav())

	case scrDownload:
		b.WriteString(m.viewDownload())

	case scrSummary:
		b.WriteString(m.viewSummary())
	}

	return b.String()
}

func (m Model) renderPromptMenu() string {
	u := m.u()

	switch m.screen {
	case scrUpdateReady:
		return sOk.Render(u.UpdateAvail) +
			sBold.Render(m.updateInfo.Latest) +
			sDim.Render(fmt.Sprintf(u.CurrentVerShort, app.Version)) +
			"\n\n" +
			m.menuAndNav()

	case scrFFmpegAsk:
		return sWarn.Render(u.FFmpegWarn) +
			"\n" +
			sDim.Render(u.FFmpegHint) +
			"\n\n" +
			m.menuAndNav()

	default:
		return sWarn.Render(u.PlaylistMixWarn) + "\n\n" + m.menuAndNav()
	}
}

func (m Model) renderTopBar() string {
	u := m.u()
	badge := " " +
		sGray.Render("yt-dlp ") + verOrDash(m.ytdlpVer) +
		sDim.Render("  ·  ") +
		sGray.Render("ffmpeg ") + verOrDash(m.ffmpegVer) +
		" "

	var action string
	if m.screen == scrDepUpdate && !m.depUpdateDone && m.depErr == "" {
		action = " " + sWarn.Render(m.spinnerView()+u.TopBarDepsBusy) + " "
	} else {
		action = " " + sGray.Render(u.TopBarDeps) + "  " + sDim.Render("Ctrl+U") + " "
	}

	if m.width == 0 {
		return badge + action
	}

	gap := m.width - lipgloss.Width(badge) - lipgloss.Width(action)
	if gap < 1 {
		gap = 1
	}
	return badge + strings.Repeat(" ", gap) + action
}

func (m Model) renderLocaleFooter() string {
	hint := m.u().LangTab + "  " + strings.ToUpper(m.locale.String())
	if m.width == 0 {
		return sDim.Render(hint)
	}
	return lipgloss.Place(m.width, 1, lipgloss.Right, lipgloss.Top, sDim.Render(hint))
}

func (m Model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	footer := m.renderLocaleFooter()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n" + body + "\n" + footer
	}

	mainH := max(1, m.height-2)
	return topBar + "\n" + lipgloss.Place(m.width, mainH, lipgloss.Center, lipgloss.Center, body) + "\n" + footer
}

func (m Model) viewDependencyProgress(label string) string {
	u := m.u()
	var b strings.Builder

	b.WriteString(sDim.Render("  " + label))
	b.WriteString("\n\n")
	b.WriteString("  " + renderProgressBar(barW, m.depProgress.Pct) + "\n")
	b.WriteString("  " + sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		b.WriteString("  " + sBold.Render(app.FmtBytes(m.depProgress.DoneB)))
		if m.depProgress.TotalB > 0 {
			b.WriteString(sDim.Render(" / " + app.FmtBytes(m.depProgress.TotalB)))
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
		b.WriteString(sInputBoxFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  " + m.plInputErr))
		}
		return b.String()
	}

	b.WriteString(sDim.Render(fmt.Sprintf(u.PlSelectedFmt, len(m.plSelected), total)))
	if total > m.playlistViewportHeight() {
		start := m.plTop + 1
		end := min(total, m.plTop+m.playlistViewportHeight())
		b.WriteString(sDim.Render(fmt.Sprintf("  %d-%d/%d", start, end, total)))
	}
	b.WriteString("\n")
	if m.plInputErr != "" {
		b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
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

	if slot.title == "" && !slot.done && !slot.failed {
		return prefix + sDim.Render(u.Waiting) + "\n"
	}

	titleRaw := sSlotTitle.Render(trunc(slot.title, 46))
	title := sDim.Render(titleRaw)
	if slot.done {
		title = sNormal.Render(titleRaw)
	}

	row1 := prefix + title
	switch {
	case slot.done:
		return row1 + "  " + sOk.Render("✔") + "\n" + indent + renderProgressBar(barW, 100) + "\n"
	case slot.failed:
		return row1 + "\n" + indent + sErr.Render(u.ErrSlot) + "\n"
	case slot.proc:
		return row1 + "\n" + indent + sWarn.Render("⚙ ") + sDim.Render(slot.label) + "\n"
	default:
		return row1 + "\n" + indent + renderProgressBar(barW, slot.pct) + "  " + sOk.Render(fmt.Sprintf("%.1f%%", slot.pct)) + "  " + fmtStats(slot.doneB, slot.totalB, slot.speed) + "\n"
	}
}

func (m Model) viewSummary() string {
	u := m.u()
	var b strings.Builder

	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render(u.SummaryOK) + "\n" + sDim.Render("     → "+app.DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render(u.SummaryFail) + "\n\n")
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

func renderProgressBar(width int, pct float64) string {
	if width <= 0 {
		return ""
	}

	totalWidth := width
	percent := math.Max(0, math.Min(1, pct/100))
	filledWidth := int(math.Round(float64(totalWidth) * percent))
	filledWidth = max(0, min(totalWidth, filledWidth))

	var b strings.Builder
	if filledWidth > 0 {
		blend := lipgloss.Blend1D(totalWidth*2, progressBlendStops...)
		blendIndex := 0
		for range filledWidth {
			b.WriteString(lipgloss.NewStyle().
				Foreground(blend[blendIndex]).
				Background(blend[blendIndex+1]).
				Render(progressFullChar))
			blendIndex += 2
		}
	}

	if filledWidth < totalWidth {
		b.WriteString(sBarRest.Render(strings.Repeat(progressEmptyChar, totalWidth-filledWidth)))
	}
	return b.String()
}

func verOrDash(value string) string {
	if value != "" {
		return sOk.Render(value)
	}
	return sDim.Render("—")
}

func sep(width int) string {
	return sDim.Render(strings.Repeat("─", width))
}

func trunc(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func fmtStats(done, total int64, speed string) string {
	var stat string
	switch {
	case total > 0:
		stat = sBold.Render(app.FmtBytes(done)) + sDim.Render("/"+app.FmtBytes(total))
	case done > 0:
		stat = sBold.Render(app.FmtBytes(done))
	default:
		stat = sDim.Render("…")
	}
	if speed != "" {
		stat += "  " + sTitle.Render(speed)
	}
	return stat
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
