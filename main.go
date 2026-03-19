package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	cCyan   = lipgloss.Color("14")
	cGreen  = lipgloss.Color("10")
	cRed    = lipgloss.Color("9")
	cYellow = lipgloss.Color("11")
	cGray   = lipgloss.Color("8")
	cDim    = lipgloss.Color("240")
	cWhite  = lipgloss.Color("15")

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(cCyan)
	sOk     = lipgloss.NewStyle().Foreground(cGreen)
	sErr    = lipgloss.NewStyle().Foreground(cRed)
	sWarn   = lipgloss.NewStyle().Foreground(cYellow)
	sGray   = lipgloss.NewStyle().Foreground(cGray)
	sBold   = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sNormal = lipgloss.NewStyle().Foreground(cWhite)

	sHeader = lipgloss.NewStyle().
		Bold(true).Foreground(cCyan).
		Border(lipgloss.RoundedBorder()).BorderForeground(cCyan).
		Padding(0, 3)

	sInputBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cGray).Padding(0, 1)
	sInputBoxFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cCyan).Padding(0, 1)
)

const (
	barW      = 30
	boardBarW = 24
	inputW    = 50
)

type screen int

const (
	scrUpdateCheck screen = iota
	scrUpdateReady
	scrUpdateDl
	scrUpdateDone
	scrFFmpegAsk
	scrDepDl
	scrDepUpdate
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

type (
	msgUpdateChecked   struct{ info *UpdateInfo }
	msgDepProgress     struct{ p FileProgress }
	msgDepDone         struct{ err error; isUpdate bool }
	msgPlaylistFetched struct{ info *PlaylistInfo; err error }
	msgDlUpdate        struct{ u DlUpdate }
)

type model struct {
	scr    screen
	width  int
	height int

	sp      spinner.Model
	progDep progress.Model

	ytdlpVer  string
	ffmpegVer string

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

	menuCursor    int
	session       Session
	prevScr       screen
	depUpdateDone bool
}

func newModel() model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = sTitle

	pg := progress.New(
		progress.WithDefaultBlend(),
		progress.WithoutPercentage(),
		progress.WithWidth(barW),
	)

	ti := textinput.New()
	ti.Placeholder = "https://youtu.be/..."
	ti.CharLimit = 300
	ti.SetWidth(inputW)

	pi := textinput.New()
	pi.Placeholder = "1-5, 2,4,7 или а (все)"
	pi.CharLimit = 100
	pi.SetWidth(38)

	return model{
		scr:        scrUpdateCheck,
		sp:         sp,
		progDep:    pg,
		urlInput:   ti,
		plInput:    pi,
		plPageSize: 12,
		plSelected: map[int]bool{},
		numWorkers: 1,
	}
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
	return tea.Batch(
		m.sp.Tick,
		func() tea.Msg { return msgUpdateChecked{CheckUpdate()} },
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progDep.SetWidth(barW)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progDep, cmd = m.progDep.Update(msg)
		return m, cmd

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
		cmd := m.progDep.SetPercent(msg.p.Pct / 100)
		return m, tea.Batch(cmd, cmdStream(m.depCh, m.scr == scrUpdateDl))

	case msgDepDone:
		if msg.err != nil {
			m.depErr = msg.err.Error()
			return m, nil
		}
		if msg.isUpdate {
			m.scr = scrUpdateDone
			return m, nil
		}
		if m.scr == scrDepUpdate {
			deps := DetectDeps()
			m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
			m.depUpdateDone = true
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
	deps := DetectDeps()
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if deps.YtdlpVer == "" {
		m.depLabel, m.depProgress, m.scr = "yt-dlp", FileProgress{}, scrDepDl
		var cmd tea.Cmd
		m.depCh, cmd = launch(InstallYtDlp, false)
		return m, cmd
	}
	return m.gotoURL()
}

func (m model) gotoURL() (tea.Model, tea.Cmd) {
	deps := DetectDeps()
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if IsWindows && deps.FFmpegMissing {
		m.scr, m.menuCursor = scrFFmpegAsk, 0
		return m, nil
	}
	m.scr = scrURL
	return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
}

func (m model) startDepUpdate() (tea.Model, tea.Cmd) {
	m.prevScr = m.scr
	m.depProgress, m.depErr, m.depUpdateDone = FileProgress{}, "", false
	m.scr = scrDepUpdate
	var cmd tea.Cmd
	m.depCh, cmd = launch(InstallAllDeps, false)
	return m, cmd
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
			lbl := m.cfg.Label
			if m.dlTotal > 0 {
				lbl += fmt.Sprintf(" [плейлист/%d]", m.dlTotal)
			}
			m.session.Record(lbl, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
			m.scr, m.menuCursor = scrSummary, 0
			return m, nil
		}
	}
	return m, cmdListenDl(m.dlCh)
}

func menuNav(cur, n int, k string) int {
	switch k {
	case "up", "k":
		return max(cur-1, 0)
	case "down", "j":
		return min(cur+1, n-1)
	}
	return cur
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	busy := m.scr == scrUpdateCheck || m.scr == scrUpdateDl ||
		m.scr == scrDepDl || m.scr == scrDepUpdate ||
		m.scr == scrDownload || m.scr == scrPlaylistFetch
	if k == "ctrl+u" && !busy {
		return m.startDepUpdate()
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

	case scrDepUpdate:
		if m.depUpdateDone || m.depErr != "" {
			switch k {
			case "enter", "esc", "q":
				m.scr = m.prevScr
				if m.scr == scrURL {
					return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
				}
				return m, nil
			}
		}

	case scrURL:
		if k != "enter" {
			ti, cmd := m.urlInput.Update(msg)
			m.urlInput = ti
			return m, cmd
		}
		url := strings.TrimSpace(m.urlInput.Value())
		if url == "" {
			m.urlErr = "Ссылка не может быть пустой"
			return m, nil
		}
		if !YtRE.MatchString(url) {
			m.urlErr = "Не похоже на YouTube-ссылку"
			return m, nil
		}
		m.urlErr, m.url = "", url
		m.plInfo, m.dlEntries, m.forceSingle, m.numWorkers = nil, nil, false, 1
		if IsPlaylistURL(url) {
			if VideoInPlaylistRE.MatchString(url) {
				m.scr, m.menuCursor = scrPlaylistAsk, 0
				return m, nil
			}
			m.scr = scrPlaylistFetch
			return m, cmdFetchPlaylist(url)
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
				return m, cmdFetchPlaylist(m.url)
			}
		}

	case scrPlaylist:
		return m.handlePlaylistKey(msg)

	case scrWorkers:
		maxW := min(len(m.dlEntries), 5)
		m.menuCursor = menuNav(m.menuCursor, maxW, k)
		if len(k) == 1 && k[0] >= '1' && k[0] <= byte('0'+maxW) {
			m.numWorkers, m.scr, m.menuCursor = int(k[0]-'0'), scrQuality, 0
		} else if k == "enter" {
			m.numWorkers, m.scr, m.menuCursor = m.menuCursor+1, scrQuality, 0
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
			m.plInputErr = "Выбери хотя бы одно видео"
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

func renderBar(pct float64, w int) string {
	f := min(int(float64(w)*pct/100), w)
	return sOk.Render(strings.Repeat("█", f)) + sGray.Render(strings.Repeat("░", w-f))
}

func choice(opts []string, sel int) string {
	var b strings.Builder
	for i, o := range opts {
		if i == sel {
			b.WriteString(sTitle.Render(" > ") + sBold.Render(o) + "\n")
		} else {
			b.WriteString(sDim.Render("   "+o) + "\n")
		}
	}
	return b.String()
}

func hintLine(s string) string {
	return "\n" + sDim.Render(s)
}

func (m model) View() tea.View {
	body := m.renderBody()
	screen := m.buildScreen(body)
	v := tea.NewView(screen)
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader"
	v.Cursor = nil
	return v
}

func (m model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n" + body
	}
	mainH := m.height - 1
	if mainH < 1 {
		mainH = 1
	}
	return topBar + "\n" + lipgloss.Place(m.width, mainH, lipgloss.Center, lipgloss.Center, body)
}

func (m model) renderTopBar() string {
	ytVal := sDim.Render("—")
	if m.ytdlpVer != "" {
		ytVal = sOk.Render(m.ytdlpVer)
	}
	ffVal := sDim.Render("—")
	if m.ffmpegVer != "" {
		ffVal = sOk.Render(m.ffmpegVer)
	}
	badge := " " + sGray.Render("yt-dlp ") + ytVal +
		sDim.Render("  ·  ") +
		sGray.Render("ffmpeg ") + ffVal + " "

	var btnText string
	if m.scr == scrDepUpdate {
		btnText = " " + sWarn.Render(m.sp.View()+" обновление…") + " "
	} else {
		btnText = " " + sGray.Render("↻ обновить зависимости") + "  " + sDim.Render("Ctrl+U") + " "
	}

	if m.width == 0 {
		return badge + btnText
	}

	badgeW := lipgloss.Width(badge)
	btnW := lipgloss.Width(btnText)
	gap := m.width - badgeW - btnW
	if gap < 1 {
		gap = 1
	}
	return badge + strings.Repeat(" ", gap) + btnText
}

func (m model) renderBody() string {
	var b strings.Builder

	header := sHeader.Render(
		sTitle.Render("VolRen") + sDim.Render("  ·  Video / Audio Downloader") + "\n" +
			sDim.Render("v"+Version+"   powered by yt-dlp"),
	)
	b.WriteString(header + "\n\n")

	switch m.scr {
	case scrUpdateCheck:
		b.WriteString("  " + m.sp.View() + sDim.Render("  Проверяю обновления…"))

	case scrUpdateReady:
		b.WriteString(
			sOk.Render("  ✔  Доступна версия ") + sBold.Render(m.updateInfo.Latest) +
				sDim.Render("  (сейчас "+Version+")") + "\n\n",
		)
		b.WriteString(choice([]string{"Да, обновить", "Пропустить"}, m.menuCursor))
		b.WriteString(hintLine("↑↓ — выбор   Enter — подтвердить   y/n"))

	case scrUpdateDl, scrDepDl:
		lbl := m.depLabel
		if m.scr == scrUpdateDl {
			lbl = "обновление " + m.updateInfo.Latest
		}
		b.WriteString(viewDlProgress(m, lbl))

	case scrUpdateDone:
		b.WriteString(sOk.Render("  ✔  Обновление применено — ")+sBold.Render(m.updateInfo.Latest)+"\n\n")
		if IsWindows {
			b.WriteString(sDim.Render("  Файл заменится после закрытия. Запустите вручную.") + "\n")
		} else {
			b.WriteString(sDim.Render("  Бинарник заменён. Перезапустите программу.") + "\n")
		}
		b.WriteString(hintLine("Любая клавиша — выйти"))

	case scrFFmpegAsk:
		b.WriteString(sWarn.Render("  ⚠  ffmpeg не найден") + "\n")
		b.WriteString(sDim.Render("     Необходим для HD, 4K и MP3") + "\n\n")
		b.WriteString(choice([]string{"Да, скачать (~80 МБ)", "Пропустить"}, m.menuCursor))
		b.WriteString(hintLine("↑↓ — выбор   Enter — подтвердить   y/n"))

	case scrDepUpdate:
		if m.depUpdateDone {
			b.WriteString(sOk.Render("  ✔  Зависимости обновлены") + "\n\n")
			b.WriteString("  " + sGray.Render("yt-dlp  ") + sOk.Render(m.ytdlpVer) + "\n")
			if m.ffmpegVer != "" {
				b.WriteString("  " + sGray.Render("ffmpeg  ") + sOk.Render(m.ffmpegVer) + "\n")
			}
			b.WriteString(hintLine("Enter / Esc — назад"))
		} else if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  Ошибка: ")+sDim.Render(m.depErr)+"\n")
			b.WriteString(hintLine("Enter / Esc — назад"))
		} else {
			b.WriteString(viewDlProgress(m, "Обновление зависимостей…"))
		}

	case scrURL:
		b.WriteString(sBold.Render("  Вставь ссылку на видео или плейлист") + "\n\n")
		inputStyle := sInputBox
		if m.urlInput.Focused() {
			inputStyle = sInputBoxFocus
		}
		b.WriteString(inputStyle.Render(m.urlInput.View()) + "\n")
		if m.urlErr != "" {
			b.WriteString("\n" + sErr.Render("  ✘  "+m.urlErr) + "\n")
		} else {
			b.WriteString(sDim.Render("\nyoutube.com/watch   youtube.com/playlist   youtu.be/") + "\n")
		}
		b.WriteString(hintLine("Enter — подтвердить   Ctrl+C — выход"))

	case scrPlaylistAsk:
		b.WriteString(sWarn.Render("  ⚠  Ссылка содержит и видео, и плейлист") + "\n\n")
		b.WriteString(choice([]string{"Только это видео", "Открыть плейлист"}, m.menuCursor))
		b.WriteString(hintLine("↑↓ / 1·2 — выбор   Enter — подтвердить"))

	case scrPlaylistFetch:
		b.WriteString("  " + m.sp.View() + sDim.Render("  Загружаю информацию о плейлисте…"))

	case scrPlaylist:
		b.WriteString(viewPlaylist(m))
	case scrWorkers:
		b.WriteString(viewWorkers(m))
	case scrQuality:
		b.WriteString(viewQuality(m))
	case scrDownload:
		b.WriteString(viewDownload(m))
	case scrSummary:
		b.WriteString(viewSummary(m))
	}

	return b.String()
}

func viewDlProgress(m model, label string) string {
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
		b.WriteString("\n" + sErr.Render("  ✘  "+m.depErr) + "\n")
	}
	return b.String()
}

func viewPlaylist(m model) string {
	var b strings.Builder
	info, total := m.plInfo, len(m.plInfo.Entries)

	b.WriteString(sBold.Render(trunc(info.Title, 46)) +
		sDim.Render(fmt.Sprintf("  %d видео", total)) + "\n")
	b.WriteString(sDim.Render(strings.Repeat("─", 56)) + "\n\n")

	start, end := m.plPage*m.plPageSize, min(m.plPage*m.plPageSize+m.plPageSize, total)
	for i, e := range info.Entries[start:end] {
		cur := start+i == m.plCursor
		ar := "  "
		if cur {
			ar = sTitle.Render("> ")
		}
		chk := sGray.Render("[ ]")
		if m.plSelected[e.Index] {
			chk = sOk.Render("[✔]")
		}
		num := sDim.Render(fmt.Sprintf("%4d", e.Index))
		ttl := lipgloss.NewStyle().Width(44).Inline(true).Render(trunc(e.Title, 40))
		if cur {
			ttl = sNormal.Render(ttl)
		} else {
			ttl = sDim.Render(ttl)
		}
		dur := sDim.Render(FmtDuration(e.Duration))
		b.WriteString(ar + chk + "  " + num + "  " + ttl + "  " + dur + "\n")
	}

	if pages := (total + m.plPageSize - 1) / m.plPageSize; pages > 1 {
		b.WriteString("\n" + sDim.Render(fmt.Sprintf("  стр. %d / %d", m.plPage+1, pages)) + "\n")
	}
	b.WriteString("\n")

	if m.plInputMode {
		b.WriteString(sTitle.Render("  Введи номера:") + "\n")
		b.WriteString(sInputBoxFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
	} else {
		b.WriteString(sDim.Render("  Выбрано: ") + sOk.Render(fmt.Sprintf("%d", len(m.plSelected))) +
			sDim.Render(fmt.Sprintf(" / %d", total)) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
		b.WriteString(hintLine("↑↓ — навигация   Пробел — выбор   a — все   / — ввод номеров   Enter — далее"))
	}
	return b.String()
}

func viewWorkers(m model) string {
	maxW := min(len(m.dlEntries), 5)
	labels := [5]string{"Последовательно  (1 поток)", "2 потока", "3 потока", "4 потока", "5 потоков"}
	notes  := [5]string{"", "", "← рекомендуется", "", ""}

	var b strings.Builder
	b.WriteString(sBold.Render("  Параллельная загрузка") +
		sDim.Render(fmt.Sprintf("   %d видео в очереди", len(m.dlEntries))) + "\n\n")

	const rowW = 48
	for i := range maxW {
		active := i == m.menuCursor
		ar, num, lbl := sDim.Render("   "), sDim.Render(fmt.Sprintf("[%d]", i+1)), sDim.Render(labels[i])
		if active {
			ar, num, lbl = sTitle.Render(" > "), sTitle.Render(fmt.Sprintf("[%d]", i+1)), sBold.Render(labels[i])
		}
		note := ""
		if notes[i] != "" {
			note = "  " + sOk.Render(notes[i])
		}
		b.WriteString(lipgloss.NewStyle().Width(rowW).Render(ar+num+"  "+lbl+note) + "\n")
	}
	b.WriteString(hintLine(fmt.Sprintf("↑↓ / 1–%d — выбор   Enter — продолжить", maxW)))
	return b.String()
}

func viewQuality(m model) string {
	type qopt struct{ icon, label, sub, key string }
	opts := []qopt{
		{"▲", "Лучшее качество", "HD · 4K · максимальное разрешение", "1"},
		{"▼", "Экономичное", "360p · быстро · небольшой размер", "2"},
		{"♪", "Только аудио", "MP3 · 320 kbps", "3"},
	}
	noFF := FFmpegResolved == ""

	var b strings.Builder
	b.WriteString(sBold.Render("  Выбери качество") + "\n\n")

	const rowW = 52
	const subIndent = "      "
	for i, o := range opts {
		active := m.menuCursor == i
		needFF := (i == 0 || i == 2) && noFF

		ar := sDim.Render("   ")
		if active {
			ar = sTitle.Render(" > ")
		}
		num := sTitle.Render("[" + o.key + "]")
		icn, lbl := sDim.Render(o.icon), sDim.Render(o.label)
		if active {
			icn, lbl = sTitle.Render(o.icon), sBold.Render(o.label)
		}
		ff := ""
		if needFF {
			ff = "  " + sWarn.Render("⚠ ffmpeg")
		}
		line1 := lipgloss.NewStyle().Width(rowW).Render(ar + num + "  " + icn + "  " + lbl + ff)
		line2 := lipgloss.NewStyle().Width(rowW).Render(subIndent + sDim.Render(o.sub))
		b.WriteString(line1 + "\n" + line2 + "\n\n")
	}
	b.WriteString(hintLine("↑↓ / 1·2·3 — выбор   Enter — начать загрузку"))
	return b.String()
}

func viewSlotInd(s slotState) string {
	switch {
	case s.done:
		return sOk.Render("●")
	case s.failed:
		return sErr.Render("●")
	case s.proc, s.title != "":
		return sTitle.Render("●")
	default:
		return sGray.Render("○")
	}
}

func viewSlot(i int, s slotState, badge bool) string {
	ind := viewSlotInd(s)
	pre := "  " + ind + "  "
	if badge {
		pre = "  " + ind + "  " + sDim.Render(fmt.Sprintf("[%d]", i+1)) + "  "
	}
	indent := strings.Repeat(" ", lipgloss.Width(pre))

	if s.title == "" && !s.done && !s.failed {
		return pre + sDim.Render("ожидание…") + "\n"
	}
	const ttlW = 46
	ttlRaw := lipgloss.NewStyle().Width(ttlW).Inline(true).Render(trunc(s.title, ttlW))
	ttl := sDim.Render(ttlRaw)
	if s.done {
		ttl = sNormal.Render(ttlRaw)
	}
	row1 := pre + ttl

	switch {
	case s.done:
		return row1 + "  " + sOk.Render("✔") + "\n" +
			indent + sOk.Render(strings.Repeat("█", boardBarW)) + "\n"
	case s.failed:
		return row1 + "\n" + indent + sErr.Render("✘  ошибка загрузки") + "\n"
	case s.proc:
		return row1 + "\n" + indent + sWarn.Render("⚙ ") + sDim.Render(s.label) + "\n"
	default:
		return row1 + "\n" + indent +
			renderBar(s.pct, boardBarW) + "  " +
			sTitle.Render(fmt.Sprintf("%.0f%%", s.pct)) + "  " +
			fmtStats(s.doneB, s.totalB, s.speed) + "\n"
	}
}

func viewDownload(m model) string {
	var b strings.Builder
	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		b.WriteString(sBold.Render(fmt.Sprintf("  Плейлист  ·  %d видео", m.dlTotal)) + "\n")
		b.WriteString("  " + renderBar(float64(done)/float64(m.dlTotal)*100, 40) + "  " +
			sOk.Render(fmt.Sprintf("%d", m.dlDone)) +
			sDim.Render(fmt.Sprintf("/%d", m.dlTotal)) + "\n\n")
		for i, s := range m.slots {
			b.WriteString(viewSlot(i, s, true))
		}
		b.WriteString("\n  " +
			sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)) + "  " +
			sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)) + "  " +
			sDim.Render(fmt.Sprintf("◷ %d в очереди", m.dlTotal-done)) + "\n")
	} else if len(m.slots) > 0 {
		b.WriteString(sTitle.Render("  Загружаю…") + "\n\n")
		b.WriteString(viewSlot(0, m.slots[0], false))
	}
	return b.String()
}

func viewSummary(m model) string {
	var b strings.Builder
	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render("  ✔  Готово!") + "\n" + sDim.Render("     → "+DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render("  ✘  Не удалось скачать") + "\n\n")
		}
	} else {
		ico := sOk.Render("✔")
		if m.dlFailed > 0 {
			ico = sWarn.Render("!")
		}
		b.WriteString("  " + ico + "  " + sBold.Render("Плейлист завершён") + "  " +
			sOk.Render(fmt.Sprintf("%d", m.dlDone)) +
			sDim.Render(fmt.Sprintf("/%d успешно", m.dlTotal)) + "\n\n")
	}
	if len(m.session.Items) > 0 {
		b.WriteString(sDim.Render("  История сессии:\n  " + strings.Repeat("─", 54)) + "\n")
		for _, item := range m.session.Items {
			ico := sOk.Render("✔")
			if !item.OK {
				ico = sErr.Render("✘")
			}
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				ico,
				lipgloss.NewStyle().Width(26).Inline(true).Render(trunc(item.Label, 26)),
				sDim.Render(trunc(item.URL, 40)),
			))
		}
		b.WriteString(sDim.Render("  "+strings.Repeat("─", 54)) + "\n")
		b.WriteString(fmt.Sprintf("  %s  %s\n\n",
			sOk.Render(fmt.Sprintf("✔ %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("✘ %d", m.session.Failed)),
		))
	}
	b.WriteString(choice([]string{"Да, скачать ещё", "Нет, выйти"}, m.menuCursor))
	b.WriteString(hintLine("↑↓ — выбор   Enter / y / n"))
	return b.String()
}

func main() {
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
