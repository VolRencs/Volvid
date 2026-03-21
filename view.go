package main

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	barW   = 40
	inputW = 50
)

var (
	cPrimary = lipgloss.Color("12")
	cRed     = lipgloss.Color("9")
	cYellow  = lipgloss.Color("11")
	cGray    = lipgloss.Color("8")
	cDim     = lipgloss.Color("240")
	cWhite   = lipgloss.Color("15")

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(cPrimary)
	sOk     = lipgloss.NewStyle().Foreground(cPrimary)
	sErr    = lipgloss.NewStyle().Foreground(cRed)
	sWarn   = lipgloss.NewStyle().Foreground(cYellow)
	sGray   = lipgloss.NewStyle().Foreground(cGray)
	sBold   = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sNormal = sBold.Bold(false)

	sHeader = lipgloss.NewStyle().Bold(true).Foreground(cPrimary).
		Border(lipgloss.RoundedBorder()).BorderForeground(cPrimary).Padding(0, 3)

	sInputBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cGray).Padding(0, 1)
	sInputBoxFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cPrimary).Padding(0, 1)

	sPlTitle   = lipgloss.NewStyle().Width(44).Inline(true)
	sSlotTitle = lipgloss.NewStyle().Width(46).Inline(true)
)

type menuItem string

func (menuItem) FilterValue() string { return "" }

type simpleMenuDelegate struct{}

func (simpleMenuDelegate) Height() int                             { return 1 }
func (simpleMenuDelegate) Spacing() int                            { return 0 }
func (simpleMenuDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (simpleMenuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s := string(item.(menuItem))
	if index == m.Index() {
		fmt.Fprint(w, sTitle.Render(" > ")+sBold.Render(s))
	} else {
		fmt.Fprint(w, sDim.Render("   "+s))
	}
}

type inlineKM []key.Binding

func (k inlineKM) ShortHelp() []key.Binding  { return k }
func (k inlineKM) FullHelp() [][]key.Binding { return [][]key.Binding{k} }

func createMenuList(opts []string) list.Model {
	items := make([]list.Item, len(opts))
	for i, o := range opts {
		items[i] = menuItem(o)
	}
	l := list.New(items, simpleMenuDelegate{}, 60, 8)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	return l
}

func (m model) kbUp() key.Binding {
	return key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", m.u().HelpUp))
}
func (m model) kbDown() key.Binding {
	return key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", m.u().HelpDown))
}
func (m model) kbEnter() key.Binding {
	return key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", m.u().HelpEnter))
}
func (m model) kbQuit() key.Binding {
	return key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", m.u().HelpQuit))
}
func (m model) kbUpdDeps() key.Binding {
	return key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", m.u().HelpDeps))
}
func (m model) kbSpace() key.Binding {
	return key.NewBinding(key.WithKeys("space"), key.WithHelp("space", m.u().HelpSpace))
}
func (m model) kbAll() key.Binding {
	return key.NewBinding(key.WithKeys("a", "а"), key.WithHelp("a", m.u().HelpAll))
}
func (m model) kbSlash() key.Binding {
	return key.NewBinding(key.WithKeys("/"), key.WithHelp("/", m.u().HelpSlash))
}
func (m model) kbAny() key.Binding {
	return key.NewBinding(key.WithKeys("any"), key.WithHelp(m.u().HelpAnyKey, m.u().HelpExit))
}

func (m model) hint(bindings ...key.Binding) string {
	return "\n" + sDim.Render(m.hlp.View(inlineKM(bindings)))
}

func (m model) menuAndNav() string { return m.menuList.View() + m.hint(m.kbUp(), m.kbDown(), m.kbEnter()) }

func verOrDash(v string) string {
	if v != "" {
		return sOk.Render(v)
	}
	return sDim.Render("—")
}

func sep(n int) string { return sDim.Render(strings.Repeat("─", n)) }

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func fmtStats(done, total int64, speed string) string {
	var s string
	switch {
	case total > 0:
		s = sBold.Render(FmtBytes(done)) + sDim.Render("/"+FmtBytes(total))
	case done > 0:
		s = sBold.Render(FmtBytes(done))
	default:
		s = sDim.Render("…")
	}
	if speed != "" {
		s += "  " + sTitle.Render(speed)
	}
	return s
}

func (m model) plVpHeight() int {
	lines := min(15, m.height-16)
	return max(3, lines)
}

func (m model) plRepaint() model {
	m.plVp.SetContent(m.renderPlItems())
	return m
}

func (m model) plStepCursor(delta int) model {
	n := len(m.plInfo.Entries)
	if n == 0 {
		return m
	}
	m.plCursor = max(0, min(m.plCursor+delta, n-1))
	m.plVp.SetContent(m.renderPlItems())
	m.plVp.EnsureVisible(m.plCursor, 0, 0)
	return m
}

func (m model) renderPlItems() string {
	if m.plInfo == nil {
		return ""
	}
	var b strings.Builder
	for i, e := range m.plInfo.Entries {
		cur := i == m.plCursor
		ar := "  "
		if cur {
			ar = sTitle.Render("> ")
		}
		chk := sGray.Render("[ ]")
		if m.plSelected[e.Index] {
			chk = sOk.Render("[✔]")
		}
		num := sDim.Render(fmt.Sprintf("%4d", e.Index))
		ttl := sPlTitle.Render(trunc(e.Title, 40))
		if cur {
			ttl = sNormal.Render(ttl)
		} else {
			ttl = sDim.Render(ttl)
		}
		dur := sDim.Render(FmtDuration(e.Duration))
		b.WriteString(ar + chk + "  " + num + "  " + ttl + "  " + dur + "\n")
	}
	return b.String()
}

func (m model) View() tea.View {
	body := m.renderBody()
	screen := m.buildScreen(body)
	v := tea.NewView(screen)
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader · v" + Version
	v.Cursor = nil
	return v
}

func (m model) renderLocaleFooter() string {
	hint := m.u().LangTab + "  " + strings.ToUpper(m.locale.String())
	if m.width == 0 {
		return sDim.Render(hint)
	}
	return lipgloss.Place(m.width, 1, lipgloss.Right, lipgloss.Top, sDim.Render(hint))
}

func (m model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	footer := m.renderLocaleFooter()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n" + body + "\n" + footer
	}
	mainH := max(1, m.height-2)
	return topBar + "\n" + lipgloss.Place(m.width, mainH, lipgloss.Center, lipgloss.Center, body) + "\n" + footer
}

func (m model) renderTopBar() string {
	u := m.u()
	badge := " " + sGray.Render("yt-dlp ") + verOrDash(m.ytdlpVer) + sDim.Render("  ·  ") + sGray.Render("ffmpeg ") + verOrDash(m.ffmpegVer) + " "
	var btnText string
	if m.scr == scrDepUpdate {
		btnText = " " + sWarn.Render(m.sp.View()+u.TopBarDepsBusy) + " "
	} else {
		btnText = " " + sGray.Render(u.TopBarDeps) + "  " + sDim.Render("Ctrl+U") + " "
	}
	if m.width == 0 {
		return badge + btnText
	}
	gap := m.width - lipgloss.Width(badge) - lipgloss.Width(btnText)
	if gap < 1 {
		gap = 1
	}
	return badge + strings.Repeat(" ", gap) + btnText
}

func (m model) renderBody() string {
	u := m.u()
	var b strings.Builder
	header := sHeader.Render(sTitle.Render("VolRen") + sDim.Render(u.AppSubtitle) + "\n" + sDim.Render(fmt.Sprintf(u.HeaderPowered, Version)))
	b.WriteString(header + "\n\n")

	switch m.scr {
	case scrUpdateCheck, scrPlaylistFetch:
		msg := u.SpinnerUpdate
		if m.scr == scrPlaylistFetch {
			msg = u.SpinnerPlaylist
		}
		b.WriteString("  " + m.sp.View() + sDim.Render(msg))
	case scrUpdateReady, scrFFmpegAsk, scrPlaylistAsk:
		var head string
		switch m.scr {
		case scrUpdateReady:
			head = sOk.Render(u.UpdateAvail) + sBold.Render(m.updateInfo.Latest) + sDim.Render(fmt.Sprintf(u.CurrentVerShort, Version)) + "\n\n"
		case scrFFmpegAsk:
			head = sWarn.Render(u.FFmpegWarn) + "\n" + sDim.Render(u.FFmpegHint) + "\n\n"
		default:
			head = sWarn.Render(u.PlaylistMixWarn) + "\n\n"
		}
		b.WriteString(head + m.menuAndNav())
	case scrUpdateDl, scrDepDl:
		lbl := m.depLabel
		if m.scr == scrUpdateDl {
			lbl = fmt.Sprintf(u.DepLabelFmt, m.updateInfo.Latest)
		}
		b.WriteString(viewDlProgress(m, lbl))
	case scrUpdateDone:
		b.WriteString(sOk.Render(u.UpdateDonePrefix) + sBold.Render(m.updateInfo.Latest) + "\n\n")
		if IsWindows {
			b.WriteString(sDim.Render(u.UpdateAppliedWin) + "\n")
		} else {
			b.WriteString(sDim.Render(u.UpdateAppliedUnix) + "\n")
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
			b.WriteString(viewDlProgress(m, u.DepsUpdating))
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
		b.WriteString(viewPlaylist(m))
	case scrWorkers, scrQuality:
		if m.scr == scrWorkers {
			b.WriteString(sBold.Render(u.ParallelFmt) + sDim.Render(fmt.Sprintf(u.WorkersQueuedFmt, len(m.dlEntries))) + "\n\n")
		} else {
			b.WriteString(sBold.Render(u.QualityTitle) + "\n\n")
		}
		b.WriteString(m.menuAndNav())
	case scrDownload:
		b.WriteString(viewDownload(m))
	case scrSummary:
		b.WriteString(viewSummary(m))
	}
	return b.String()
}

func viewDlProgress(m model, label string) string {
	u := m.u()
	var b strings.Builder
	b.WriteString(sDim.Render("  "+label) + "\n\n")
	b.WriteString("  " + m.progDep.View() + "\n")
	b.WriteString("  " + sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		b.WriteString("  " + sBold.Render(FmtBytes(m.depProgress.DoneB)))
		if m.depProgress.TotalB > 0 {
			b.WriteString(sDim.Render(" / " + FmtBytes(m.depProgress.TotalB)))
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

func viewPlaylist(m model) string {
	u := m.u()
	var b strings.Builder
	info := m.plInfo
	total := len(info.Entries)
	b.WriteString(sBold.Render(trunc(info.Title, 46)) + sDim.Render(fmt.Sprintf(u.PlVideosFmt, total)) + "\n")
	b.WriteString("  " + sep(54) + "\n\n")
	b.WriteString(m.plVp.View() + "\n\n")
	if m.plInputMode {
		b.WriteString(sTitle.Render(u.PlEnterNums) + "\n")
		b.WriteString(sInputBoxFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
	} else {
		b.WriteString(sDim.Render(fmt.Sprintf(u.PlSelectedFmt, len(m.plSelected), total)) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
		b.WriteString(m.hint(m.kbUp(), m.kbDown(), m.kbSpace(), m.kbAll(), m.kbSlash(), m.kbEnter()))
	}
	return b.String()
}

func viewDownload(m model) string {
	u := m.u()
	var b strings.Builder
	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		pct := float64(done) / float64(m.dlTotal) * 100
		b.WriteString(sBold.Render(fmt.Sprintf(u.PlaylistBarFmt, m.dlTotal)) + "\n")
		b.WriteString("  " + m.overallProg.View() + "\n")
		b.WriteString("  " + sOk.Render(fmt.Sprintf("%.1f%%", pct)) + sOk.Render(fmt.Sprintf(" %d", m.dlDone)) + sDim.Render(fmt.Sprintf("/%d", m.dlTotal)) + "\n\n")
		for i, s := range m.slots {
			b.WriteString(viewSlot(m, i, s, m.progs[i].View(), true))
		}
		b.WriteString("\n  " + sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)) + "  " + sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)) + "  " + sDim.Render(fmt.Sprintf(u.QueueFmt, m.dlTotal-done)) + "  " + sDim.Render(m.dlSW.View()) + "\n")
	} else if len(m.slots) > 0 {
		b.WriteString(sTitle.Render(u.Downloading) + "\n\n")
		b.WriteString(viewSlot(m, 0, m.slots[0], m.progs[0].View(), false))
	}
	return b.String()
}

func viewSlot(m model, i int, s slotState, bar string, badge bool) string {
	u := m.u()
	var ind string
	switch {
	case s.done:
		ind = sOk.Render("●")
	case s.failed:
		ind = sErr.Render("●")
	case s.proc, s.title != "":
		ind = sTitle.Render("●")
	default:
		ind = sGray.Render("○")
	}
	pre := "  " + ind + "  "
	if badge {
		pre = "  " + ind + "  " + sDim.Render(fmt.Sprintf("[%d]", i+1)) + "  "
	}
	indent := strings.Repeat(" ", lipgloss.Width(pre))
	if s.title == "" && !s.done && !s.failed {
		return pre + sDim.Render(u.Waiting) + "\n"
	}
	ttlRaw := sSlotTitle.Render(trunc(s.title, 46))
	ttl := sDim.Render(ttlRaw)
	if s.done {
		ttl = sNormal.Render(ttlRaw)
	}
	row1 := pre + ttl
	switch {
	case s.done:
		return row1 + "  " + sOk.Render("✔") + "\n" + indent + bar + "\n"
	case s.failed:
		return row1 + "\n" + indent + sErr.Render(u.ErrSlot) + "\n"
	case s.proc:
		return row1 + "\n" + indent + sWarn.Render("⚙ ") + sDim.Render(s.label) + "\n"
	default:
		return row1 + "\n" + indent + bar + "  " + sOk.Render(fmt.Sprintf("%.1f%%", s.pct)) + "  " + fmtStats(s.doneB, s.totalB, s.speed) + "\n"
	}
}

func viewSummary(m model) string {
	u := m.u()
	var b strings.Builder
	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render(u.SummaryOK) + "\n" + sDim.Render("     → "+DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render(u.SummaryFail) + "\n\n")
		}
	} else {
		ico := sOk.Render("✔")
		if m.dlFailed > 0 {
			ico = sWarn.Render("!")
		}
		b.WriteString("  " + ico + "  " + sBold.Render(u.SummaryPlaylistTitle) + "  " + sOk.Render(fmt.Sprintf("%d", m.dlDone)) + sDim.Render(fmt.Sprintf(u.SuccessFmt, m.dlTotal)) + "\n\n")
	}
	if len(m.session.Items) > 0 {
		b.WriteString(sDim.Render(u.SessionHist) + "\n" + "  " + sep(54) + "\n")
		for _, item := range m.session.Items {
			ico := sOk.Render("✔")
			if !item.OK {
				ico = sErr.Render("✘")
			}
			b.WriteString(fmt.Sprintf("  %s  %-26s  %s\n",
				ico,
				trunc(item.Label, 26),
				sDim.Render(trunc(item.URL, 30)),
			))
		}
		b.WriteString("  " + sep(54) + "\n")
		b.WriteString(fmt.Sprintf("  %s  %s\n\n",
			sOk.Render(fmt.Sprintf("✔ %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("✘ %d", m.session.Failed)),
		))
	}
	b.WriteString(m.menuAndNav())
	return b.String()
}
