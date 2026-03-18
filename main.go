package main

import (
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	sTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	sOk    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	sWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	sGray  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sBold  = lipgloss.NewStyle().Bold(true)
	sDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	sBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 2)
	sInput = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).MarginLeft(2)
	sInputFocus = sInput.BorderForeground(lipgloss.Color("14"))

	titleCol     = lipgloss.NewStyle().Width(44)
	slotTitleW   = lipgloss.NewStyle().Width(48)
	wWorkerLabel = lipgloss.NewStyle().Width(20)
)

const (
	barW      = 28
	boardBarW = 20
)

var (
	sep        = sGray.Render(strings.Repeat("─", 56))
	spinFrames = [10]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func spin(t int) string    { return sTitle.Render(spinFrames[t%10]) }
func hint(s string) string { return sGray.Render("  "+s) + "\n" }

func menuRow(active bool, n int, label, note string) string {
	return arrow(active) + sTitle.Render(fmt.Sprintf("[%d]", n)) + "  " + label + note + "\n"
}

func renderBar(pct float64, w int) string {
	f := min(int(float64(w)*pct/100), w)
	return sOk.Render(strings.Repeat("█", f)) + sGray.Render(strings.Repeat("░", w-f))
}

func arrow(ok bool) string {
	if ok {
		return sTitle.Render("▶ ")
	}
	return "  "
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func fmtStats(done, total int64, speed string) string {
	s := "…"
	if total > 0 {
		s = FmtBytes(done) + sGray.Render("/") + FmtBytes(total)
	} else if done > 0 {
		s = FmtBytes(done)
	}
	if speed != "" {
		s += "  " + sGray.Render(speed)
	}
	return s
}

func slotInd(s slotState) string {
	switch {
	case s.done:              return sOk.Render("▌")
	case s.failed:            return sErr.Render("▌")
	case s.proc, s.title != "": return sTitle.Render("▌")
	default:                  return sGray.Render("▌")
	}
}

type (
	msgUpdateChecked   struct{ info *UpdateInfo }
	msgDepProgress     struct{ p FileProgress }
	msgDepDone         struct{ err error; isUpdate bool }
	msgPlaylistFetched struct{ info *PlaylistInfo; err error }
	msgDlUpdate        struct{ u DlUpdate }
)

type screen int

const (
	scrUpdateCheck screen = iota
	scrUpdateReady
	scrUpdateDl
	scrUpdateDone
	scrFFmpegAsk
	scrDepDl
	scrURL
	scrPlaylistAsk
	scrPlaylistFetch
	scrPlaylist
	scrWorkers
	scrQuality
	scrDownload
	scrSummary
)

type slotState struct {
	title  string
	pct    float64
	doneB  int64
	totalB int64
	speed  string
	label  string
	proc   bool
	done   bool
	failed bool
}

type model struct {
	scr    screen
	width  int
	height int
	tick   int

	updateInfo  *UpdateInfo
	depProgress FileProgress
	depLabel    string
	depErr      string
	depCh       <-chan FileProgress

	urlInput textinput.Model
	urlErr   string

	plInfo      *PlaylistInfo
	plPage      int
	plCursor    int
	plPageSize  int
	plSelected  map[int]bool
	plInputMode bool
	plInput     textinput.Model
	plInputErr  string

	cfg         QualityConfig
	url         string
	dlEntries   []PlaylistEntry
	forceSingle bool
	numWorkers  int
	dlCh        <-chan DlUpdate
	slots       []slotState
	dlDone      int
	dlFailed    int
	dlTotal     int
	singleOK    bool

	menuCursor int
	session    Session
}

func newModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://youtu.be/..."
	ti.CharLimit = 300
	ti.SetWidth(54)

	pi := textinput.New()
	pi.Placeholder = "1-5, 2,4,7 или а (все)"
	pi.CharLimit = 100
	pi.SetWidth(40)

	return model{
		scr:        scrUpdateCheck,
		urlInput:   ti,
		plInput:    pi,
		plPageSize: 12,
		plSelected: map[int]bool{},
		numWorkers: 1,
	}
}

func cmdCheckUpdate() tea.Cmd {
	return func() tea.Msg { return msgUpdateChecked{CheckUpdate()} }
}

func cmdStream(ch <-chan FileProgress, isUpdate bool) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{nil, isUpdate}
		}
		if p.Done {
			return msgDepDone{p.Err, isUpdate}
		}
		return msgDepProgress{p}
	}
}

func launch(fn func(chan<- FileProgress) error, isUpdate bool) (<-chan FileProgress, tea.Cmd) {
	ch := make(chan FileProgress, 16)
	go func() {
		if err := fn(ch); err != nil {
			ch <- FileProgress{Done: true, Err: err}
		}
		close(ch)
	}()
	return ch, cmdStream(ch, isUpdate)
}

func cmdFetchPlaylist(url string) tea.Cmd {
	return func() tea.Msg {
		info, err := FetchPlaylistInfo(url)
		return msgPlaylistFetched{info, err}
	}
}

func cmdListenDl(ch <-chan DlUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{DlUpdate{Type: EvClosed}}
		}
		return msgDlUpdate{u}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(cmdCheckUpdate(), tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.tick++
		if m.scr == scrUpdateCheck || m.scr == scrPlaylistFetch {
			return m, tickCmd()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo, m.scr, m.menuCursor = msg.info, scrUpdateReady, 1
		return m, nil
	case msgDepProgress:
		m.depProgress = msg.p
		return m, cmdStream(m.depCh, m.scr == scrUpdateDl)
	case msgDepDone:
		if msg.err != nil {
			m.depErr = msg.err.Error()
			return m, nil
		}
		if msg.isUpdate {
			m.scr = scrUpdateDone
			return m, nil
		}
		return m.gotoURL()
	case msgPlaylistFetched:
		if msg.err != nil || msg.info == nil {
			m.forceSingle, m.scr, m.menuCursor = true, scrQuality, 0
			return m, nil
		}
		m.plInfo, m.plPage, m.plCursor = msg.info, 0, 0
		m.plSelected = map[int]bool{}
		m.scr = scrPlaylist
		return m, nil
	case msgDlUpdate:
		return m.handleDlUpdate(msg.u)
	}
	switch {
	case m.scr == scrURL:
		ti, cmd := m.urlInput.Update(msg)
		m.urlInput = ti
		return m, cmd
	case m.scr == scrPlaylist && m.plInputMode:
		pi, cmd := m.plInput.Update(msg)
		m.plInput = pi
		return m, cmd
	}
	return m, nil
}

func (m model) gotoChecks() (tea.Model, tea.Cmd) {
	if DetectDeps().YtdlpVer == "" {
		m.depLabel, m.depProgress, m.scr = "yt-dlp", FileProgress{}, scrDepDl
		var cmd tea.Cmd
		m.depCh, cmd = launch(InstallYtDlp, false)
		return m, cmd
	}
	return m.gotoURL()
}

func (m model) gotoURL() (tea.Model, tea.Cmd) {
	if IsWindows && DetectDeps().FFmpegMissing {
		m.scr, m.menuCursor = scrFFmpegAsk, 0
		return m, nil
	}
	m.scr = scrURL
	return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
}

func (m model) handleDlUpdate(u DlUpdate) (tea.Model, tea.Cmd) {
	if u.Type == EvClosed {
		return m, nil
	}
	if u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case EvStart:
			*s = slotState{title: trunc(u.Text, 50)}
		case EvDest:
			s.title = trunc(u.Text, 50)
		case EvProgress:
			s.pct, s.doneB, s.totalB, s.speed, s.proc = u.Pct, u.DoneB, u.TotalB, u.Speed, false
		case EvProc, EvFallback:
			s.proc, s.label = true, u.Text
		case EvReset:
			*s = slotState{}
		case EvDone:
			s.done, s.failed, s.pct = u.OK, !u.OK, 100
		}
	}
	if u.Type == EvDone {
		if u.OK {
			m.dlDone++
		} else {
			m.dlFailed++
		}
		if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
			if m.dlTotal == 0 {
				m.singleOK = u.OK
			}
			label := m.cfg.Label
			if m.dlTotal > 0 {
				label += fmt.Sprintf(" [плейлист/%d]", m.dlTotal)
			}
			m.session.Record(label, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
			m.scr, m.menuCursor = scrSummary, 0
			return m, nil
		}
	}
	return m, cmdListenDl(m.dlCh)
}

func menuNav(cur, maxN int, k string) int {
	switch k {
	case "up", "k":   return max(cur-1, 0)
	case "down", "j": return min(cur+1, maxN-1)
	}
	return cur
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.scr {
	case scrUpdateReady:
		m.menuCursor = menuNav(m.menuCursor, 2, k)
		switch k {
		case "y", "д":
			m.menuCursor = 0
			fallthrough
		case "enter":
			if m.menuCursor == 0 {
				m.scr, m.depProgress = scrUpdateDl, FileProgress{}
				info := m.updateInfo
				var cmd tea.Cmd
				m.depCh, cmd = launch(func(ch chan<- FileProgress) error { return ApplyUpdate(info, ch) }, true)
				return m, cmd
			}
			return m.gotoChecks()
		case "n", "н", "esc", "q":
			return m.gotoChecks()
		}

	case scrUpdateDone:
		return m, tea.Quit

	case scrFFmpegAsk:
		m.menuCursor = menuNav(m.menuCursor, 2, k)
		switch k {
		case "y", "д":
			m.menuCursor = 0
			fallthrough
		case "enter":
			if m.menuCursor == 0 {
				m.depLabel, m.depProgress, m.scr = "ffmpeg", FileProgress{}, scrDepDl
				var cmd tea.Cmd
				m.depCh, cmd = launch(InstallFFmpeg, false)
				return m, cmd
			}
			return m.gotoURL()
		case "n", "н", "q":
			return m.gotoURL()
		}

	case scrURL:
		if k != "enter" {
			ti, cmd := m.urlInput.Update(msg)
			m.urlInput = ti
			return m, cmd
		}
		url := strings.TrimSpace(m.urlInput.Value())
		if url == "" {
			m.urlErr = "Ссылка не может быть пустой."
			return m, nil
		}
		if !YtRE.MatchString(url) {
			m.urlErr = "Не похоже на YouTube-ссылку."
			return m, nil
		}
		m.urlErr, m.url = "", url
		m.plInfo, m.dlEntries = nil, nil
		m.forceSingle, m.numWorkers = false, 1
		if IsPlaylistURL(url) {
			if VideoInPlaylistRE.MatchString(url) {
				m.scr, m.menuCursor = scrPlaylistAsk, 0
				return m, nil
			}
			m.scr = scrPlaylistFetch
			return m, tea.Batch(cmdFetchPlaylist(url), tickCmd())
		}
		m.scr, m.menuCursor = scrQuality, 0

	case scrPlaylistAsk:
		switch k {
		case "up", "k", "1":
			m.menuCursor = 0
		case "down", "j", "2":
			m.menuCursor = 1
		case "enter":
			if m.menuCursor == 0 {
				m.forceSingle, m.scr, m.menuCursor = true, scrQuality, 0
			} else {
				m.scr = scrPlaylistFetch
				return m, tea.Batch(cmdFetchPlaylist(m.url), tickCmd())
			}
		}

	case scrPlaylist:
		return m.handlePlaylistKey(msg)

	case scrWorkers:
		maxW := min(len(m.dlEntries), 5)
		m.menuCursor = menuNav(m.menuCursor, maxW, k)
		if len(k) == 1 && k[0] >= '1' && k[0] <= byte('0'+maxW) {
			m.numWorkers = int(k[0] - '0')
			m.scr, m.menuCursor = scrQuality, 0
		} else if k == "enter" {
			m.numWorkers = m.menuCursor + 1
			m.scr, m.menuCursor = scrQuality, 0
		}

	case scrQuality:
		m.menuCursor = menuNav(m.menuCursor, 3, k)
		switch k {
		case "1", "2", "3":
			m.menuCursor = int(k[0] - '1')
			return m.startDownload()
		case "enter":
			return m.startDownload()
		}

	case scrSummary:
		m.menuCursor = menuNav(m.menuCursor, 2, k)
		switch k {
		case "y", "д", "enter":
			if k == "enter" && m.menuCursor != 0 {
				return m, tea.Quit
			}
			if k != "enter" || m.menuCursor == 0 {
				return m.resetForNext()
			}
		case "n", "н", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) handlePlaylistKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if m.plInputMode {
		switch k {
		case "enter":
			indices, err := ParseSelection(m.plInput.Value(), len(m.plInfo.Entries))
			if err != nil {
				m.plInputErr = err.Error()
				return m, nil
			}
			m.plSelected = make(map[int]bool, len(indices))
			for _, i := range indices {
				m.plSelected[i] = true
			}
			m.plInput.Blur()
			m.plInputMode, m.plInputErr = false, ""
		case "esc":
			m.plInput.Blur()
			m.plInputMode, m.plInputErr = false, ""
		default:
			pi, cmd := m.plInput.Update(msg)
			m.plInput = pi
			return m, cmd
		}
		return m, nil
	}
	total := len(m.plInfo.Entries)
	switch k {
	case "up", "k":
		if m.plCursor > 0 {
			m.plCursor--
			if m.plCursor < m.plPage*m.plPageSize {
				m.plPage--
			}
		}
	case "down", "j":
		if m.plCursor < total-1 {
			m.plCursor++
			if m.plCursor >= (m.plPage+1)*m.plPageSize {
				m.plPage++
			}
		}
	case "space":
		idx := m.plInfo.Entries[m.plCursor].Index
		if m.plSelected[idx] {
			delete(m.plSelected, idx)
		} else {
			m.plSelected[idx] = true
		}
	case "a", "а":
		if len(m.plSelected) == total {
			m.plSelected = map[int]bool{}
		} else {
			m.plSelected = make(map[int]bool, total)
			for _, e := range m.plInfo.Entries {
				m.plSelected[e.Index] = true
			}
		}
	case "/":
		m.plInputMode = true
		m.plInput.SetValue("")
		return m, tea.Batch(m.plInput.Focus(), textinput.Blink)
	case "enter":
		if len(m.plSelected) == 0 {
			m.plInputErr = "Выбери хотя бы одно видео."
			return m, nil
		}
		sel := make([]PlaylistEntry, 0, len(m.plSelected))
		for _, e := range m.plInfo.Entries {
			if m.plSelected[e.Index] {
				sel = append(sel, e)
			}
		}
		m.dlEntries, m.menuCursor = sel, 0
		if len(sel) >= 2 {
			m.scr = scrWorkers
		} else {
			m.scr = scrQuality
		}
		return m, nil
	}
	return m, nil
}

func (m model) startDownload() (tea.Model, tea.Cmd) {
	m.cfg = Qualities[m.menuCursor]
	w := max(m.numWorkers, 1)
	if len(m.dlEntries) == 0 {
		w = 1
	}
	m.slots = make([]slotState, w)
	m.dlDone, m.dlFailed, m.dlTotal = 0, 0, len(m.dlEntries)
	ch := make(chan DlUpdate, 256)
	m.dlCh, m.scr = ch, scrDownload
	StartDownload(m.cfg, m.url, m.forceSingle, m.plInfo, m.dlEntries, w, ch)
	return m, cmdListenDl(ch)
}

func (m model) resetForNext() (tea.Model, tea.Cmd) {
	m.scr, m.url, m.urlErr = scrURL, "", ""
	m.urlInput.SetValue("")
	m.plInfo, m.dlEntries = nil, nil
	m.plSelected = map[int]bool{}
	m.plInput.Blur()
	m.plInputMode, m.plInputErr = false, ""
	m.forceSingle, m.numWorkers = false, 1
	m.slots = nil
	m.dlDone, m.dlFailed, m.dlTotal, m.menuCursor = 0, 0, 0, 0
	return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
}

func (m model) View() tea.View {
	var b strings.Builder
	b.WriteString(sBox.Render(
		sTitle.Render("  VolRen  ·  Video / Audio  Downloader  ")+"\n"+
			sDim.Render("  версия "+Version+"  •  powered by yt-dlp  "),
	) + "\n\n")

	switch m.scr {
	case scrUpdateCheck:
		fmt.Fprintf(&b, "  %s  %s\n", spin(m.tick), sGray.Render("Проверяю обновления…"))

	case scrUpdateReady:
		b.WriteString(sOk.Render("  ✔  Доступна новая версия: ") +
			sBold.Render(m.updateInfo.Latest) +
			sGray.Render("  (текущая: "+Version+")") + "\n\n")
		b.WriteString(viewMenu("Обновить сейчас?", []string{"Да", "Нет"}, m.menuCursor))
		b.WriteString(hint("[↑↓] выбрать  [Enter] / [y/n]"))

	case scrUpdateDl, scrDepDl:
		label := m.depLabel
		if m.scr == scrUpdateDl {
			label = "обновление " + m.updateInfo.Latest
		}
		b.WriteString(sTitle.Render("  Скачиваю "+label+"…") + "\n\n")
		b.WriteString(viewProgress(m.depProgress))
		if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.depErr) + "\n")
		}

	case scrUpdateDone:
		b.WriteString(sOk.Render("  ✔  Обновление "+m.updateInfo.Latest+" применено.") + "\n\n")
		if IsWindows {
			b.WriteString(sDim.Render("  Файл будет заменён после закрытия. Запустите вручную.") + "\n")
		} else {
			b.WriteString(sDim.Render("  Бинарник заменён. Перезапустите:") + "\n\n")
			b.WriteString(sDim.Render("    ./VolRenDownloader") + "\n")
		}
		b.WriteString("\n" + hint("[любая клавиша] выйти"))

	case scrFFmpegAsk:
		b.WriteString(sWarn.Render("  ⚠  ffmpeg не найден — нужен для HD и MP3") + "\n\n")
		b.WriteString(viewMenu("Скачать (~80 МБ)?", []string{"Да, скачать", "Нет, пропустить"}, m.menuCursor))
		b.WriteString(hint("[↑↓] выбрать  [Enter] / [y/n]"))

	case scrURL:
		b.WriteString(sBold.Render("  Вставь ссылку:") + "\n\n")
		style := sInput
		if m.urlInput.Focused() {
			style = sInputFocus
		}
		b.WriteString(style.Render(m.urlInput.View()) + "\n")
		if m.urlErr != "" {
			b.WriteString("\n" + sErr.Render("  ✘  "+m.urlErr) + "\n")
		}
		b.WriteString("\n" + hint("[Enter] подтвердить  [Ctrl+C] выход"))

	case scrPlaylistAsk:
		b.WriteString(sWarn.Render("  ⚠  Ссылка содержит и видео, и плейлист") + "\n\n")
		b.WriteString(viewMenu("", []string{"Только это видео", "Открыть плейлист"}, m.menuCursor))
		b.WriteString(hint("[↑↓] / [1/2]  [Enter] выбрать"))

	case scrPlaylistFetch:
		fmt.Fprintf(&b, "  %s  %s\n", spin(m.tick), sGray.Render("Загружаю плейлист…"))

	case scrPlaylist:  b.WriteString(viewPlaylist(m))
	case scrWorkers:   b.WriteString(viewWorkers(m))
	case scrQuality:   b.WriteString(viewQuality(m))
	case scrDownload:  b.WriteString(viewDownload(m))
	case scrSummary:   b.WriteString(viewSummary(m))
	}

	v := tea.NewView(m.center(b.String()))
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader"
	v.Cursor = nil
	return v
}

func (m model) center(s string) string {
	if m.width == 0 || m.height == 0 {
		return s
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
}

func viewMenu(title string, opts []string, sel int) string {
	var b strings.Builder
	if title != "" {
		b.WriteString("  " + sBold.Render(title) + "\n\n")
	}
	for i, opt := range opts {
		b.WriteString(arrow(i == sel) + opt + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

func viewProgress(p FileProgress) string {
	s := fmt.Sprintf("  [%s]  %s\n", renderBar(p.Pct, barW), sOk.Render(fmt.Sprintf("%.1f%%", p.Pct)))
	if p.DoneB > 0 {
		s += "  " + fmtStats(p.DoneB, p.TotalB, p.Speed) + "\n"
	}
	return s
}

func viewPlaylist(m model) string {
	var b strings.Builder
	info, total := m.plInfo, len(m.plInfo.Entries)
	fmt.Fprintf(&b, "%s%s\n%s\n\n",
		    sBold.Render(fmt.Sprintf("  Плейлист: «%s»", trunc(info.Title, 40))),
		    sGray.Render(fmt.Sprintf("  (%d видео)", total)),
		    sep)

	start := m.plPage * m.plPageSize
	for i, e := range info.Entries[start:min(start+m.plPageSize, total)] {
		chk := sGray.Render("[ ]")
		if m.plSelected[e.Index] {
			chk = sOk.Render("[✔]")
		}
		colLeft  := lipgloss.NewStyle().Width(7).Render(arrow(start+i == m.plCursor) + chk)
		colNum   := lipgloss.NewStyle().Width(7).Render(sTitle.Render(fmt.Sprintf("%4d.", e.Index)))
		colTitle := titleCol.Copy().Inline(true).Render(trunc(e.Title, 40))
		colDur   := sDim.Render(FmtDuration(e.Duration))

		line := lipgloss.JoinHorizontal(lipgloss.Left, colLeft, colNum, colTitle, colDur)
		b.WriteString(line + "\n")
	}

	if pages := (total + m.plPageSize - 1) / m.plPageSize; pages > 1 {
		fmt.Fprintf(&b, "\n%s\n", sGray.Render(fmt.Sprintf("  стр. %d / %d", m.plPage+1, pages)))
	}
	b.WriteString("\n")
	if m.plInputMode {
		b.WriteString("  " + sTitle.Render("Введи номера:") + "\n" + sInputFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
	} else {
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
		fmt.Fprintf(&b, "%s\n\n%s",
			    sGray.Render(fmt.Sprintf("  Выбрано: %d / %d", len(m.plSelected), total)),
			    hint("[↑↓] навигация  [Пробел] выбрать  [a] все  [/] ввод  [Enter] далее"))
	}
	return b.String()
}

func viewWorkers(m model) string {
	maxW := min(len(m.dlEntries), 5)
	labels := [5]string{"Последовательно", "2 потока", "3 потока", "4 потока", "5 потоков"}

	var b strings.Builder
	fmt.Fprintf(&b, "%s%s\n\n",
		    sBold.Render("  Параллельная загрузка:"),
		    sGray.Render(fmt.Sprintf("  (%d видео)", len(m.dlEntries))))

	rowStyle := lipgloss.NewStyle().Width(48)

	for i := range maxW {
		note := ""
		if i == 2 {
			note = " " + sDim.Render("← рекомендуется")
		}
		active := i == m.menuCursor

		opt := arrow(active) + sTitle.Render(fmt.Sprintf("[%d]", i+1)) + "  " +
		wWorkerLabel.Render(labels[i]) + note

		b.WriteString(rowStyle.Render(opt) + "\n")
	}
	fmt.Fprintf(&b, "\n%s", hint(fmt.Sprintf("[↑↓] / [1-%d]  [Enter] выбрать", maxW)))
	return b.String()
}

func viewQuality(m model) string {
	noFF := ""
	if FFmpegResolved == "" {
		noFF = sDim.Render("  [нужен ffmpeg]")
	}
	return sBold.Render("  Выбери качество:") + "\n\n" +
		menuRow(m.menuCursor == 0, 1, "Лучшее качество  (HD / 4K)", noFF) +
		menuRow(m.menuCursor == 1, 2, "Экономичное  (360p)", "") +
		menuRow(m.menuCursor == 2, 3, "Только аудио  (MP3)", noFF) +
		"\n" + hint("[↑↓] / [1/2/3]  [Enter] начать")
}

func viewSlot(i int, s slotState, badge bool) string {
	ind := slotInd(s)
	var pre, ind2 string
	if badge {
		pre  = "  " + ind + "  " + sTitle.Render(fmt.Sprintf("[%d]", i+1)) + "  "
		ind2 = "          "
	} else {
		pre  = "  " + ind + "  "
		ind2 = "     "
	}
	if s.title == "" && !s.done && !s.failed {
		return pre + sDim.Render("ожидание…") + "\n\n"
	}
	row1 := pre + slotTitleW.Render(trunc(s.title, 48))
	switch {
	case s.done:
		return row1 + "  " + sOk.Render("✔ готово") + "\n" +
			ind2 + sOk.Render(strings.Repeat("█", boardBarW)) + "\n"
	case s.failed:
		return row1 + "\n" + ind2 + sErr.Render("✘ ошибка загрузки") + "\n"
	case s.proc:
		return row1 + "\n" + ind2 + sWarn.Render("⚙ ") + sGray.Render(s.label) + "\n"
	default:
		return row1 + "\n" + ind2 + fmt.Sprintf("%s [%s]  %s  %s\n",
			sTitle.Render("↓"), renderBar(s.pct, boardBarW),
			sBold.Render(fmt.Sprintf("%5.1f%%", s.pct)),
			fmtStats(s.doneB, s.totalB, s.speed))
	}
}

func viewDownload(m model) string {
	var b strings.Builder
	if m.dlTotal > 0 {
		fmt.Fprintf(&b, "%s\n%s\n\n", sBold.Render(fmt.Sprintf("  Плейлист  ·  %d видео", m.dlTotal)), sep)
		for i, s := range m.slots {
			b.WriteString(viewSlot(i, s, true) + "\n")
		}
		pending := m.dlTotal - m.dlDone - m.dlFailed
		fmt.Fprintf(&b, "%s\n  %s   %s   %s\n", sep,
			sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)),
			sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)),
			sGray.Render(fmt.Sprintf("◷ %d в очереди  ·  %d / %d", pending, m.dlDone+m.dlFailed, m.dlTotal)))
	} else if len(m.slots) > 0 {
		b.WriteString(sTitle.Render("  Загружаю…") + "\n\n" + viewSlot(0, m.slots[0], false))
	}
	return b.String()
}

func viewSummary(m model) string {
	var b strings.Builder
	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render("  ✔  Готово!") + "\n" + sDim.Render("     → "+DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render("  ✘  Не удалось скачать.") + "\n" +
				sDim.Render("     Попробуй: VolRenDownloader --update") + "\n\n")
		}
	} else {
		fmt.Fprintf(&b, "%s\n\n",
			sOk.Render("  ✔  ")+sBold.Render(fmt.Sprintf("Плейлист завершён  ·  успешно: %d / %d", m.dlDone, m.dlTotal)))
	}
	if len(m.session.Items) > 0 {
		b.WriteString(sBold.Render("  Итоги сессии:") + "\n" + sep + "\n")
		for _, item := range m.session.Items {
			badge := sOk.Render("OK  ")
			if !item.OK {
				badge = sErr.Render("FAIL")
			}
			fmt.Fprintf(&b, "  [%s]  %-22s  %s\n", badge, trunc(item.Label, 22), sDim.Render(trunc(item.URL, 42)))
		}
		fmt.Fprintf(&b, "%s\n  %s  ·  %s  ·  %s\n\n", sep,
			sBold.Render(fmt.Sprintf("Всего: %d", len(m.session.Items))),
			sOk.Render(fmt.Sprintf("✔ %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("✘ %d", m.session.Failed)))
	}
	b.WriteString(viewMenu("Скачать ещё?", []string{"Да", "Нет, выйти"}, m.menuCursor))
	b.WriteString(hint("[↑↓] выбрать  [Enter] / [y/n]"))
	return b.String()
}

func main() {
	if slices.Contains(os.Args[1:], "--update") {
		fmt.Println("Обновляю yt-dlp…")
		if err := InstallYtDlp(nil); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка yt-dlp:", err)
			os.Exit(1)
		}
		if IsWindows {
			fmt.Println("Обновляю ffmpeg…")
			if err := InstallFFmpeg(nil); err != nil {
				fmt.Fprintln(os.Stderr, "Ошибка ffmpeg:", err)
				os.Exit(1)
			}
		}
		fmt.Println("Готово.")
		return
	}
	p := tea.NewProgram(newModel())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() { <-sigs; p.Quit() }()
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	signal.Stop(sigs)
}
