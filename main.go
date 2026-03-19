package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
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
	sNormal = sBold.Bold(false)

	sHeader = lipgloss.NewStyle().Bold(true).Foreground(cCyan).
		Border(lipgloss.RoundedBorder()).BorderForeground(cCyan).Padding(0, 3)

	sInputBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cGray).Padding(0, 1)
	sInputBoxFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cCyan).Padding(0, 1)

	sPlTitle = lipgloss.NewStyle().Width(44).Inline(true)
	sSlotTitle = lipgloss.NewStyle().Width(46).Inline(true)

	keyUp      = key.NewBinding(key.WithKeys("up", "k"),  key.WithHelp("↑/k", "вверх"))
	keyDown    = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "вниз"))
	keyEnter   = key.NewBinding(key.WithKeys("enter"),     key.WithHelp("enter", "выбрать"))
	keyQuit    = key.NewBinding(key.WithKeys("ctrl+c"),    key.WithHelp("ctrl+c", "выход"))
	keyUpdDeps = key.NewBinding(key.WithKeys("ctrl+u"),    key.WithHelp("ctrl+u", "обновить зависимости"))
	keySpace   = key.NewBinding(key.WithKeys("space"),      key.WithHelp("пробел", "выбрать"))
	keyAll     = key.NewBinding(key.WithKeys("a", "а"),    key.WithHelp("a", "все"))
	keySlash   = key.NewBinding(key.WithKeys("/"),         key.WithHelp("/", "ввод номеров"))

	qualityOpts = []string{"▲ Лучшее качество (HD·4K)", "▼ Экономичное (360p)", "♪ Только аудио (MP3)"}
)

const (
	barW   = 40
	inputW = 50
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

type menuItem string
func (menuItem) FilterValue() string { return "" }

type simpleMenuDelegate struct{}
func (simpleMenuDelegate) Height() int             { return 1 }
func (simpleMenuDelegate) Spacing() int            { return 0 }
func (simpleMenuDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (simpleMenuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s := string(item.(menuItem))
	if index == m.Index() {
		fmt.Fprint(w, sTitle.Render(" > ")+sBold.Render(s))
	} else {
		fmt.Fprint(w, sDim.Render("   "+s))
	}
}

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
	hlp     help.Model

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
	plVp        viewport.Model
	plCursor    int
	plSelected  map[int]bool
	plInputMode bool
	plInput     textinput.Model
	plInputErr  string

	menuList list.Model

	cfg         QualityConfig
	url         string
	dlEntries   []PlaylistEntry
	forceSingle bool
	numWorkers  int
	dlCh        <-chan DlUpdate
	slots       []slotState
	progs       []progress.Model
	overallProg progress.Model
	dlSW        stopwatch.Model
	dlDone      int
	dlFailed    int
	dlTotal     int
	singleOK    bool

	session       Session
	prevScr       screen
	depUpdateDone bool
}

func createMenuList(opts []string) list.Model {
	items := make([]list.Item, len(opts))
	for i, o := range opts { items[i] = menuItem(o) }
	l := list.New(items, simpleMenuDelegate{}, 60, 8)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	return l
}

func newModel() model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = sTitle
	hlp := help.New()
	hlp.ShortSeparator = "   "
	pg := progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage(), progress.WithWidth(barW))
	ti := textinput.New(); ti.Placeholder = "https://youtu.be/..."; ti.CharLimit = 300; ti.SetWidth(inputW)
	pi := textinput.New(); pi.Placeholder = "1-5, 2,4,7 или а (все)"; pi.CharLimit = 100; pi.SetWidth(38)
	return model{scr: scrUpdateCheck, sp: sp, hlp: hlp, progDep: pg, urlInput: ti, plInput: pi, numWorkers: 1, plSelected: map[int]bool{}, dlSW: stopwatch.New()}
}

func cmdStream(ch <-chan FileProgress, isUpdate bool) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok { return msgDepDone{nil, isUpdate} }
		if p.Done { return msgDepDone{p.Err, isUpdate} }
		return msgDepProgress{p}
	}
}

func launch(fn func(chan<- FileProgress) error, isUpdate bool) (<-chan FileProgress, tea.Cmd) {
	ch := make(chan FileProgress, 16)
	go func() {
		if err := fn(ch); err != nil { ch <- FileProgress{Done: true, Err: err} }
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
		if !ok { return msgDlUpdate{DlUpdate{Type: EvClosed}} }
		return msgDlUpdate{u}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, func() tea.Msg { return msgUpdateChecked{CheckUpdate()} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progDep.SetWidth(barW)
		for i := range m.progs { m.progs[i].SetWidth(barW) }
		m.overallProg.SetWidth(barW)
		m.plVp.SetWidth(74)
		m.plVp.SetHeight(m.plVpHeight())
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		var cmds []tea.Cmd
		if m.scr == scrUpdateDl || m.scr == scrDepDl || m.scr == scrDepUpdate {
			var cmd tea.Cmd; m.progDep, cmd = m.progDep.Update(msg); cmds = append(cmds, cmd)
		}
		if m.scr == scrDownload {
			for i := range m.progs { var cmd tea.Cmd; m.progs[i], cmd = m.progs[i].Update(msg); cmds = append(cmds, cmd) }
			var cmd tea.Cmd; m.overallProg, cmd = m.overallProg.Update(msg); cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case stopwatch.StartStopMsg, stopwatch.ResetMsg, stopwatch.TickMsg:
		var cmd tea.Cmd
		m.dlSW, cmd = m.dlSW.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case msgUpdateChecked:
		if msg.info == nil { return m.gotoChecks() }
		m.updateInfo = msg.info
		m.scr = scrUpdateReady
		m.menuList = createMenuList([]string{"Да, обновить", "Пропустить"})
		return m, nil
	case msgDepProgress:
		m.depProgress = msg.p
		cmd := m.progDep.SetPercent(msg.p.Pct / 100)
		return m, tea.Batch(cmd, cmdStream(m.depCh, m.scr == scrUpdateDl))
	case msgDepDone:
		if msg.err != nil { m.depErr = msg.err.Error(); return m, nil }
		if msg.isUpdate { m.scr = scrUpdateDone; return m, nil }
		if m.scr == scrDepUpdate {
			deps := DetectDeps()
			m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
			m.depUpdateDone = true
			return m, nil
		}
		return m.gotoURL()
	case msgPlaylistFetched:
		if msg.err != nil || msg.info == nil {
			m.forceSingle, m.scr = true, scrQuality
			m.menuList = createMenuList(qualityOpts)
			return m, nil
		}
		m.plInfo = msg.info
		m.plCursor = 0
		m.plSelected = map[int]bool{}
		m.plVp = viewport.New(viewport.WithWidth(74), viewport.WithHeight(m.plVpHeight()))
		m.plVp.SetContent(m.renderPlItems())
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
	return m.gotoURLWithDeps(deps)
}

func (m model) gotoURL() (tea.Model, tea.Cmd) {
	return m.gotoURLWithDeps(DetectDeps())
}

func (m model) gotoURLWithDeps(deps CheckDepsResult) (tea.Model, tea.Cmd) {
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if IsWindows && deps.FFmpegMissing {
		m.scr = scrFFmpegAsk
		m.menuList = createMenuList([]string{"Да, скачать (~80 МБ)", "Пропустить"})
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
	if u.Type == EvClosed { return m, nil }
	var extraCmds []tea.Cmd
	if u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case EvStart: *s = slotState{title: trunc(u.Text, 50)}
		case EvDest: s.title = trunc(u.Text, 50)
		case EvProgress:
			s.pct, s.doneB, s.totalB, s.speed, s.proc = u.Pct, u.DoneB, u.TotalB, u.Speed, false
			if u.Slot < len(m.progs) { extraCmds = append(extraCmds, m.progs[u.Slot].SetPercent(u.Pct/100)) }
		case EvProc, EvFallback: s.proc, s.label = true, u.Text
		case EvReset: *s = slotState{}
		case EvDone:
			s.done, s.failed, s.pct = u.OK, !u.OK, 100
			if u.Slot < len(m.progs) { extraCmds = append(extraCmds, m.progs[u.Slot].SetPercent(1.0)) }
		}
	}
	if u.Type == EvDone {
		if u.OK { m.dlDone++ } else { m.dlFailed++ }
		if m.dlTotal > 0 {
			ratio := float64(m.dlDone+m.dlFailed) / float64(m.dlTotal)
			extraCmds = append(extraCmds, m.overallProg.SetPercent(ratio))
		}
		if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
			if m.dlTotal == 0 { m.singleOK = u.OK }
			lbl := m.cfg.Label
			if m.dlTotal > 0 { lbl += fmt.Sprintf(" [плейлист/%d]", m.dlTotal) }
			m.session.Record(lbl, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
			m.scr = scrSummary
			m.menuList = createMenuList([]string{"Да, скачать ещё", "Нет, выйти"})
			return m, nil
		}
	}
	return m, tea.Batch(append([]tea.Cmd{cmdListenDl(m.dlCh)}, extraCmds...)...)
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" { return m, tea.Quit }
	busy := m.scr == scrUpdateDl || m.scr == scrDepDl || m.scr == scrDepUpdate || m.scr == scrDownload || m.scr == scrPlaylistFetch
	if k == "ctrl+u" && !busy { return m.startDepUpdate() }

	if m.scr == scrUpdateReady || m.scr == scrFFmpegAsk || m.scr == scrPlaylistAsk || m.scr == scrSummary || m.scr == scrWorkers || m.scr == scrQuality {
		if k == "enter" {
			idx := m.menuList.Index()
			switch m.scr {
			case scrUpdateReady:
				if idx == 0 {
					m.scr, m.depProgress = scrUpdateDl, FileProgress{}
					info := m.updateInfo
					var cmd tea.Cmd
					m.depCh, cmd = launch(func(ch chan<- FileProgress) error { return ApplyUpdate(info, ch) }, true)
					return m, cmd
				}
				return m.gotoChecks()
			case scrFFmpegAsk:
				if idx == 0 {
					m.depLabel, m.depProgress, m.scr = "ffmpeg", FileProgress{}, scrDepDl
					var cmd tea.Cmd
					m.depCh, cmd = launch(InstallFFmpeg, false)
					return m, cmd
				}
				return m.gotoURL()
			case scrPlaylistAsk:
				if idx == 0 { m.forceSingle, m.scr = true, scrQuality } else { m.scr = scrPlaylistFetch; return m, cmdFetchPlaylist(m.url) }
				m.menuList = createMenuList(qualityOpts)
				return m, nil
			case scrSummary:
				if idx == 0 { return m.resetForNext() }
				return m, tea.Quit
			case scrWorkers:
				m.numWorkers = idx + 1
				m.scr = scrQuality
				m.menuList = createMenuList(qualityOpts)
				return m, nil
			case scrQuality:
				m.cfg = Qualities[idx]
				return m.startDownload()
			}
		}
		var cmd tea.Cmd
		m.menuList, cmd = m.menuList.Update(msg)
		return m, cmd
	}

	if m.scr == scrPlaylist {
		if m.plInputMode {
			switch k {
			case "enter":
				indices, err := ParseSelection(m.plInput.Value(), len(m.plInfo.Entries))
				if err != nil { m.plInputErr = err.Error(); return m, nil }
				m.plSelected = map[int]bool{}
				for _, idx := range indices { m.plSelected[idx] = true }
				m.plInput.Blur(); m.plInputMode, m.plInputErr = false, ""
				m.plVp.SetContent(m.renderPlItems())
			case "esc":
				m.plInput.Blur(); m.plInputMode, m.plInputErr = false, ""
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
				m.plVp.SetContent(m.renderPlItems())
				m.plVp.EnsureVisible(m.plCursor, 0, 0)
			}
		case "down", "j":
			if m.plCursor < total-1 {
				m.plCursor++
				m.plVp.SetContent(m.renderPlItems())
				m.plVp.EnsureVisible(m.plCursor, 0, 0)
			}
		case "space":
			idx := m.plInfo.Entries[m.plCursor].Index
			if m.plSelected[idx] {
				delete(m.plSelected, idx)
			} else {
				m.plSelected[idx] = true
			}
			m.plVp.SetContent(m.renderPlItems())
		case "a", "а":
			if len(m.plSelected) == total {
				m.plSelected = map[int]bool{}
			} else {
				m.plSelected = make(map[int]bool, total)
				for _, e := range m.plInfo.Entries { m.plSelected[e.Index] = true }
			}
			m.plVp.SetContent(m.renderPlItems())
		case "/":
			m.plInputMode = true
			m.plInput.SetValue("")
			return m, tea.Batch(m.plInput.Focus(), textinput.Blink)
		case "enter":
			sel := make([]PlaylistEntry, 0, len(m.plSelected))
			for _, e := range m.plInfo.Entries {
				if m.plSelected[e.Index] { sel = append(sel, e) }
			}
			if len(sel) == 0 { m.plInputErr = "Выбери хотя бы одно видео"; return m, nil }
			m.dlEntries = sel
			if len(sel) >= 2 {
				m.scr = scrWorkers
				maxW := min(len(sel), 5)
				opts := make([]string, maxW)
				for i := 0; i < maxW; i++ {
					if i == 0 { opts[i] = "Последовательно (1 поток)" } else { opts[i] = fmt.Sprintf("%d потоков", i+1) }
				}
				m.menuList = createMenuList(opts)
			} else {
				m.scr = scrQuality
				m.menuList = createMenuList(qualityOpts)
			}
			return m, nil
		}
		return m, nil
	}

	switch m.scr {
	case scrUpdateDone:
		return m, tea.Quit
	case scrDepUpdate:
		if m.depUpdateDone || m.depErr != "" {
			if k == "enter" || k == "esc" || k == "q" {
				m.scr = m.prevScr
				if m.scr == scrURL { return m, tea.Batch(m.urlInput.Focus(), textinput.Blink) }
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
		if url == "" { m.urlErr = "Ссылка не может быть пустой"; return m, nil }
		if !YtRE.MatchString(url) { m.urlErr = "Не похоже на YouTube-ссылку"; return m, nil }
		m.urlErr, m.url = "", url
		m.plInfo, m.dlEntries, m.forceSingle, m.numWorkers = nil, nil, false, 1
		if IsPlaylistURL(url) {
			if VideoInPlaylistRE.MatchString(url) {
				m.scr = scrPlaylistAsk
				m.menuList = createMenuList([]string{"Только это видео", "Открыть плейлист"})
				return m, nil
			}
			m.scr = scrPlaylistFetch
			return m, cmdFetchPlaylist(url)
		}
		m.scr = scrQuality
		m.menuList = createMenuList(qualityOpts)
	}
	return m, nil
}

func (m model) startDownload() (tea.Model, tea.Cmd) {
	w := max(m.numWorkers, 1)
	if len(m.dlEntries) == 0 { w = 1 }
	m.slots = make([]slotState, w)
	m.progs = make([]progress.Model, w)
	for i := range m.progs {
		m.progs[i] = progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage(), progress.WithWidth(barW))
	}
	m.overallProg = progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage(), progress.WithWidth(barW))
	m.dlDone, m.dlFailed, m.dlTotal = 0, 0, len(m.dlEntries)
	ch := make(chan DlUpdate, 256)
	m.dlCh, m.scr = ch, scrDownload
	StartDownload(m.cfg, m.url, m.forceSingle, m.plInfo, m.dlEntries, w, ch)
	return m, tea.Batch(cmdListenDl(ch), m.dlSW.Start())
}

func (m model) resetForNext() (tea.Model, tea.Cmd) {
	m.scr, m.url, m.urlErr = scrURL, "", ""
	m.urlInput.SetValue("")
	m.plInfo = nil
	m.plVp = viewport.Model{}
	m.plCursor = 0
	m.plSelected = map[int]bool{}
	m.plInputMode, m.plInputErr = false, ""
	m.forceSingle, m.numWorkers = false, 1
	m.slots = nil
	m.progs = nil
	m.overallProg = progress.Model{}
	return m, tea.Batch(m.urlInput.Focus(), textinput.Blink, m.dlSW.Stop(), m.dlSW.Reset())
}

func (m model) plVpHeight() int {
	available := m.height - 15
	lines := min(15, available)
	return max(3, lines)
}

func (m model) renderPlItems() string {
	if m.plInfo == nil { return "" }
	var b strings.Builder
	for i, e := range m.plInfo.Entries {
		cur := i == m.plCursor
		ar := "  "; if cur { ar = sTitle.Render("> ") }
		chk := sGray.Render("[ ]"); if m.plSelected[e.Index] { chk = sOk.Render("[✔]") }
		num := sDim.Render(fmt.Sprintf("%4d", e.Index))
		ttl := sPlTitle.Render(trunc(e.Title, 40))
		if cur { ttl = sNormal.Render(ttl) } else { ttl = sDim.Render(ttl) }
		dur := sDim.Render(FmtDuration(e.Duration))
		b.WriteString(ar + chk + "  " + num + "  " + ttl + "  " + dur + "\n")
	}
	return b.String()
}

func sep(n int) string { return sDim.Render(strings.Repeat("─", n)) }

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n { return string(r[:n-1]) + "…" }
	return s
}

func fmtStats(done, total int64, speed string) string {
	var s string
	switch {
	case total > 0: s = sBold.Render(FmtBytes(done)) + sDim.Render("/"+FmtBytes(total))
	case done > 0: s = sBold.Render(FmtBytes(done))
	default: s = sDim.Render("…")
	}
	if speed != "" { s += "  " + sTitle.Render(speed) }
	return s
}

type inlineKM []key.Binding
func (k inlineKM) ShortHelp() []key.Binding  { return k }
func (k inlineKM) FullHelp() [][]key.Binding { return [][]key.Binding{k} }

func (m model) hint(bindings ...key.Binding) string {
	return "\n" + sDim.Render(m.hlp.View(inlineKM(bindings)))
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
	if m.width == 0 || m.height == 0 { return topBar + "\n" + body }
	mainH := m.height - 1; if mainH < 1 { mainH = 1 }
	return topBar + "\n" + lipgloss.Place(m.width, mainH, lipgloss.Center, lipgloss.Center, body)
}

func (m model) renderTopBar() string {
	ytVal := sDim.Render("—"); if m.ytdlpVer != "" { ytVal = sOk.Render(m.ytdlpVer) }
	ffVal := sDim.Render("—"); if m.ffmpegVer != "" { ffVal = sOk.Render(m.ffmpegVer) }
	badge := " " + sGray.Render("yt-dlp ") + ytVal + sDim.Render("  ·  ") + sGray.Render("ffmpeg ") + ffVal + " "
	var btnText string
	if m.scr == scrDepUpdate {
		btnText = " " + sWarn.Render(m.sp.View()+" обновление…") + " "
	} else {
		btnText = " " + sGray.Render("↻ обновить зависимости") + "  " + sDim.Render("Ctrl+U") + " "
	}
	if m.width == 0 { return badge + btnText }
	badgeW := lipgloss.Width(badge)
	btnW := lipgloss.Width(btnText)
	gap := m.width - badgeW - btnW; if gap < 1 { gap = 1 }
	return badge + strings.Repeat(" ", gap) + btnText
}

func (m model) renderBody() string {
	var b strings.Builder
	header := sHeader.Render(sTitle.Render("VolRen") + sDim.Render("  ·  Video / Audio Downloader") + "\n" + sDim.Render("v"+Version+"   powered by yt-dlp"))
	b.WriteString(header + "\n\n")

	switch m.scr {
	case scrUpdateCheck:
		b.WriteString("  " + m.sp.View() + sDim.Render("  Проверяю обновления…"))
	case scrUpdateReady:
		b.WriteString(sOk.Render("  ✔  Доступна версия ") + sBold.Render(m.updateInfo.Latest) + sDim.Render("  (сейчас "+Version+")") + "\n\n")
		b.WriteString(m.menuList.View())
		b.WriteString(m.hint(keyUp, keyDown, keyEnter))
	case scrFFmpegAsk:
		b.WriteString(sWarn.Render("  ⚠️ ffmpeg не найден") + "\n")
		b.WriteString(sDim.Render("     Необходим для HD, 4K и MP3") + "\n\n")
		b.WriteString(m.menuList.View())
		b.WriteString(m.hint(keyUp, keyDown, keyEnter))
	case scrPlaylistAsk:
		b.WriteString(sWarn.Render("  ⚠️ Ссылка содержит и видео, и плейлист") + "\n\n")
		b.WriteString(m.menuList.View())
		b.WriteString(m.hint(keyUp, keyDown, keyEnter))
	case scrUpdateDl, scrDepDl:
		lbl := m.depLabel
		if m.scr == scrUpdateDl { lbl = "обновление " + m.updateInfo.Latest }
		b.WriteString(viewDlProgress(m, lbl))
	case scrUpdateDone:
		b.WriteString(sOk.Render("  ✔  Обновление применено — ")+sBold.Render(m.updateInfo.Latest)+"\n\n")
		if IsWindows {
			b.WriteString(sDim.Render("  Файл заменится после закрытия. Запустите вручную.") + "\n")
		} else {
			b.WriteString(sDim.Render("  Бинарник заменён. Перезапустите программу.") + "\n")
		}
		b.WriteString(m.hint(key.NewBinding(key.WithKeys("any"), key.WithHelp("любая клавиша", "выйти"))))
	case scrDepUpdate:
		if m.depUpdateDone {
			b.WriteString(sOk.Render("  ✔  Зависимости обновлены") + "\n\n")
			b.WriteString("  " + sGray.Render("yt-dlp  ") + sOk.Render(m.ytdlpVer) + "\n")
			if m.ffmpegVer != "" { b.WriteString("  " + sGray.Render("ffmpeg  ") + sOk.Render(m.ffmpegVer) + "\n") }
			b.WriteString(m.hint(keyEnter))
		} else if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  Ошибка: ")+sDim.Render(m.depErr)+"\n")
			b.WriteString(m.hint(keyEnter))
		} else {
			b.WriteString(viewDlProgress(m, "Обновление зависимостей…"))
		}
	case scrURL:
		b.WriteString(sBold.Render("  Вставь ссылку на видео или плейлист") + "\n\n")
		inputStyle := sInputBox; if m.urlInput.Focused() { inputStyle = sInputBoxFocus }
		b.WriteString(inputStyle.Render(m.urlInput.View()) + "\n")
		if m.urlErr != "" {
			b.WriteString("\n" + sErr.Render("  ✘  "+m.urlErr) + "\n")
		} else {
			b.WriteString(sDim.Render("\nyoutube.com/watch   youtube.com/playlist   youtu.be/") + "\n")
		}
		b.WriteString(m.hint(keyEnter, keyQuit, keyUpdDeps))
	case scrPlaylistFetch:
		b.WriteString("  " + m.sp.View() + sDim.Render("  Загружаю информацию о плейлисте…"))
	case scrPlaylist:
		b.WriteString(viewPlaylist(m))
	case scrWorkers:
		b.WriteString(sBold.Render("  Параллельная загрузка") + sDim.Render(fmt.Sprintf("   %d видео в очереди", len(m.dlEntries))) + "\n\n")
		b.WriteString(m.menuList.View())
		b.WriteString(m.hint(keyUp, keyDown, keyEnter))
	case scrQuality:
		b.WriteString(sBold.Render("  Выбери качество") + "\n\n")
		b.WriteString(m.menuList.View())
		b.WriteString(m.hint(keyUp, keyDown, keyEnter))
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
		if m.depProgress.TotalB > 0 { b.WriteString(sDim.Render(" / " + FmtBytes(m.depProgress.TotalB))) }
		if m.depProgress.Speed != "" { b.WriteString("  " + sTitle.Render(m.depProgress.Speed)) }
	}
	b.WriteString("\n")
	if m.depErr != "" { b.WriteString("\n" + sErr.Render("  ✘  "+m.depErr) + "\n") }
	return b.String()
}

func viewPlaylist(m model) string {
	var b strings.Builder
	info := m.plInfo
	total := len(info.Entries)
	b.WriteString(sBold.Render(trunc(info.Title, 46)) + sDim.Render(fmt.Sprintf("  %d видео", total)) + "\n")
	b.WriteString("  " + sep(54) + "\n\n")
	b.WriteString(m.plVp.View() + "\n\n")
	if m.plInputMode {
		b.WriteString(sTitle.Render("  Введи номера:") + "\n")
		b.WriteString(sInputBoxFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" { b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n") }
	} else {
		b.WriteString(sDim.Render("  Выбрано: ") + sOk.Render(fmt.Sprintf("%d", len(m.plSelected))) + sDim.Render(fmt.Sprintf(" / %d", total)) + "\n")
		if m.plInputErr != "" { b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n") }
		b.WriteString(m.hint(keyUp, keyDown, keySpace, keyAll, keySlash, keyEnter))
	}
	return b.String()
}

func viewDownload(m model) string {
	var b strings.Builder
	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		pct := float64(done) / float64(m.dlTotal) * 100
		b.WriteString(sBold.Render(fmt.Sprintf("  Плейлист  ·  %d видео", m.dlTotal)) + "\n")
		b.WriteString("  " + m.overallProg.View() + "\n")
		b.WriteString("  " + sOk.Render(fmt.Sprintf("%.1f%%", pct)) + sOk.Render(fmt.Sprintf(" %d", m.dlDone)) + sDim.Render(fmt.Sprintf("/%d", m.dlTotal)) + "\n\n")
		for i, s := range m.slots { b.WriteString(viewSlot(i, s, m.progs[i].View(), true)) }
		b.WriteString("\n  " + sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)) + "  " + sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)) + "  " + sDim.Render(fmt.Sprintf("◷ %d в очереди", m.dlTotal-done)) + "  " + sDim.Render(m.dlSW.View()) + "\n")
	} else if len(m.slots) > 0 {
		b.WriteString(sTitle.Render("  Загружаю…") + "\n\n")
		b.WriteString(viewSlot(0, m.slots[0], m.progs[0].View(), false))
	}
	return b.String()
}

func viewSlot(i int, s slotState, bar string, badge bool) string {
	var ind string
	switch {
	case s.done:               ind = sOk.Render("●")
	case s.failed:             ind = sErr.Render("●")
	case s.proc, s.title != "": ind = sTitle.Render("●")
	default:                   ind = sGray.Render("○")
	}
	pre := "  " + ind + "  "
	if badge { pre = "  " + ind + "  " + sDim.Render(fmt.Sprintf("[%d]", i+1)) + "  " }
	indent := strings.Repeat(" ", lipgloss.Width(pre))
	if s.title == "" && !s.done && !s.failed { return pre + sDim.Render("ожидание…") + "\n" }
	ttlRaw := sSlotTitle.Render(trunc(s.title, 46))
	ttl := sDim.Render(ttlRaw)
	if s.done { ttl = sNormal.Render(ttlRaw) }
	row1 := pre + ttl
	switch {
	case s.done:   return row1 + "  " + sOk.Render("✔") + "\n" + indent + bar + "\n"
	case s.failed: return row1 + "\n" + indent + sErr.Render("✘  ошибка загрузки") + "\n"
	case s.proc:   return row1 + "\n" + indent + sWarn.Render("⚙ ") + sDim.Render(s.label) + "\n"
	default:
		return row1 + "\n" + indent + bar + "  " + sOk.Render(fmt.Sprintf("%.1f%%", s.pct)) + "  " + fmtStats(s.doneB, s.totalB, s.speed) + "\n"
	}
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
		if m.dlFailed > 0 { ico = sWarn.Render("!") }
		b.WriteString("  " + ico + "  " + sBold.Render("Плейлист завершён") + "  " + sOk.Render(fmt.Sprintf("%d", m.dlDone)) + sDim.Render(fmt.Sprintf("/%d успешно", m.dlTotal)) + "\n\n")
	}
	if len(m.session.Items) > 0 {
		b.WriteString(sDim.Render("  История сессии:") + "\n" + "  " + sep(54) + "\n")
		for _, item := range m.session.Items {
			ico := sOk.Render("✔")
			if !item.OK { ico = sErr.Render("✘") }
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
	b.WriteString(m.menuList.View())
	b.WriteString(m.hint(keyUp, keyDown, keyEnter))
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
}
