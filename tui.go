package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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
	msgUpdateChecked struct{ info *UpdateInfo }
	msgDepProgress   struct{ p FileProgress }
	msgDepDone       struct {
		err      error
		isUpdate bool
	}
	msgPlaylistFetched struct {
		info *PlaylistInfo
		err  error
	}
	msgDlUpdate struct{ u DlUpdate }
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

	locale Locale
}

func newModel() model {
	loc := loadLocale()
	syncLoc(loc)
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = sTitle
	hlp := help.New()
	hlp.ShortSeparator = "   "
	pg := progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage(), progress.WithWidth(barW))
	ti, pi := textinput.New(), textinput.New()
	ti.Placeholder, ti.CharLimit = "https://youtu.be/...", 300
	ti.SetWidth(inputW)
	pi.Placeholder, pi.CharLimit = Loc.PlInputPlaceholder, 100
	pi.SetWidth(38)
	return model{
		scr: scrUpdateCheck, sp: sp, hlp: hlp, progDep: pg, urlInput: ti, plInput: pi,
		numWorkers: 1, plSelected: map[int]bool{}, dlSW: stopwatch.New(), locale: loc,
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

func (m model) uiBusy() bool {
	switch m.scr {
	case scrUpdateDl, scrDepDl, scrDepUpdate, scrDownload, scrPlaylistFetch:
		return true
	}
	return false
}

func (m model) workerMenuOpts(n int) []string {
	u := m.u()
	opts := make([]string, n)
	opts[0] = u.WorkerSeq
	for i := 1; i < n; i++ {
		opts[i] = fmt.Sprintf(u.WorkerNFmt, i+1)
	}
	return opts
}

func (m model) syncedMenus() model {
	u := m.u()
	switch m.scr {
	case scrUpdateReady:
		m.menuList = createMenuList([]string{u.MenuUpdateY, u.MenuUpdateN})
	case scrFFmpegAsk:
		m.menuList = createMenuList([]string{u.MenuFFmpegY, u.MenuFFmpegN})
	case scrPlaylistAsk:
		m.menuList = createMenuList([]string{u.MenuVidOnly, u.MenuOpenPl})
	case scrSummary:
		m.menuList = createMenuList([]string{u.MenuAgainY, u.MenuAgainN})
	case scrWorkers:
		maxW := min(len(m.dlEntries), 5)
		m.menuList = createMenuList(m.workerMenuOpts(maxW))
	case scrQuality:
		m.menuList = createMenuList(m.qualityOpts())
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.sp.Tick, func() tea.Msg { return msgUpdateChecked{CheckUpdate()} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progDep.SetWidth(barW)
		for i := range m.progs {
			m.progs[i].SetWidth(barW)
		}
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
			var cmd tea.Cmd
			m.progDep, cmd = m.progDep.Update(msg)
			cmds = append(cmds, cmd)
		}
		if m.scr == scrDownload {
			for i := range m.progs {
				var cmd tea.Cmd
				m.progs[i], cmd = m.progs[i].Update(msg)
				cmds = append(cmds, cmd)
			}
			var cmd tea.Cmd
			m.overallProg, cmd = m.overallProg.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case stopwatch.StartStopMsg, stopwatch.ResetMsg, stopwatch.TickMsg:
		var cmd tea.Cmd
		m.dlSW, cmd = m.dlSW.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo = msg.info
		m.scr = scrUpdateReady
		m = m.syncedMenus()
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
			m.forceSingle, m.scr = true, scrQuality
			m = m.syncedMenus()
			return m, nil
		}
		m.plInfo = msg.info
		m.plCursor = 0
		m.plSelected = map[int]bool{}
		m.plVp = viewport.New(viewport.WithWidth(74), viewport.WithHeight(m.plVpHeight()))
		m = m.plRepaint()
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
		m = m.syncedMenus()
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
	var extraCmds []tea.Cmd
	if u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case EvStart:
			*s = slotState{title: trunc(u.Text, 50)}
		case EvDest:
			s.title = trunc(u.Text, 50)
		case EvProgress:
			s.pct, s.doneB, s.totalB, s.speed, s.proc = u.Pct, u.DoneB, u.TotalB, u.Speed, false
			if u.Slot < len(m.progs) {
				extraCmds = append(extraCmds, m.progs[u.Slot].SetPercent(u.Pct/100))
			}
		case EvProc, EvFallback:
			s.proc, s.label = true, u.Text
		case EvReset:
			*s = slotState{}
		case EvDone:
			s.done, s.failed, s.pct = u.OK, !u.OK, 100
			if u.Slot < len(m.progs) {
				extraCmds = append(extraCmds, m.progs[u.Slot].SetPercent(1.0))
			}
		}
	}
	if u.Type == EvDone {
		if u.OK {
			m.dlDone++
		} else {
			m.dlFailed++
		}
		if m.dlTotal > 0 {
			ratio := float64(m.dlDone+m.dlFailed) / float64(m.dlTotal)
			extraCmds = append(extraCmds, m.overallProg.SetPercent(ratio))
		}
		if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
			if m.dlTotal == 0 {
				m.singleOK = u.OK
			}
			lbl := m.cfg.Label
			if m.dlTotal > 0 {
				lbl += m.sessionPlaylistSuffix(m.dlTotal)
			}
			m.session.Record(lbl, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
			m.scr = scrSummary
			m = m.syncedMenus()
			return m, nil
		}
	}
	return m, tea.Batch(append([]tea.Cmd{cmdListenDl(m.dlCh)}, extraCmds...)...)
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if k == "tab" {
		m.locale = nextLocale(m.locale)
		syncLoc(m.locale)
		_ = saveLocale(m.locale)
		m.plInput.Placeholder = Loc.PlInputPlaceholder
		m = m.syncedMenus()
		return m, nil
	}
	if k == "ctrl+u" && !m.uiBusy() {
		return m.startDepUpdate()
	}

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
				if idx == 0 {
					m.forceSingle, m.scr = true, scrQuality
				} else {
					m.scr = scrPlaylistFetch
					return m, cmdFetchPlaylist(m.url)
				}
				m = m.syncedMenus()
				return m, nil
			case scrSummary:
				if idx == 0 {
					return m.resetForNext()
				}
				return m, tea.Quit
			case scrWorkers:
				m.numWorkers = idx + 1
				m.scr = scrQuality
				m = m.syncedMenus()
				return m, nil
			case scrQuality:
				m.cfg = m.qualityAt(idx)
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
				if err != nil {
					m.plInputErr = err.Error()
					return m, nil
				}
				m.plSelected = map[int]bool{}
				for _, idx := range indices {
					m.plSelected[idx] = true
				}
				m.plInput.Blur()
				m.plInputMode, m.plInputErr = false, ""
				m = m.plRepaint()
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
			m = m.plStepCursor(-1)
		case "down", "j":
			m = m.plStepCursor(1)
		case "space":
			idx := m.plInfo.Entries[m.plCursor].Index
			if m.plSelected[idx] {
				delete(m.plSelected, idx)
			} else {
				m.plSelected[idx] = true
			}
			m = m.plRepaint()
		case "a", "а":
			if len(m.plSelected) == total {
				m.plSelected = map[int]bool{}
			} else {
				m.plSelected = make(map[int]bool, total)
				for _, e := range m.plInfo.Entries {
					m.plSelected[e.Index] = true
				}
			}
			m = m.plRepaint()
		case "/":
			m.plInputMode = true
			m.plInput.SetValue("")
			return m, tea.Batch(m.plInput.Focus(), textinput.Blink)
		case "enter":
			sel := make([]PlaylistEntry, 0, len(m.plSelected))
			for _, e := range m.plInfo.Entries {
				if m.plSelected[e.Index] {
					sel = append(sel, e)
				}
			}
			if len(sel) == 0 {
				m.plInputErr = m.u().ErrPickOne
				return m, nil
			}
			m.dlEntries = sel
			if len(sel) >= 2 {
				m.scr = scrWorkers
				m = m.syncedMenus()
			} else {
				m.scr = scrQuality
				m = m.syncedMenus()
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
			m.urlErr = m.u().URLErrEmpty
			return m, nil
		}
		if !YtRE.MatchString(url) {
			m.urlErr = m.u().URLErrBad
			return m, nil
		}
		m.urlErr, m.url = "", url
		m.plInfo, m.dlEntries, m.forceSingle, m.numWorkers = nil, nil, false, 1
		if IsPlaylistURL(url) {
			if VideoInPlaylistRE.MatchString(url) {
				m.scr = scrPlaylistAsk
				m = m.syncedMenus()
				return m, nil
			}
			m.scr = scrPlaylistFetch
			return m, cmdFetchPlaylist(url)
		}
		m.scr = scrQuality
		m = m.syncedMenus()
	}
	return m, nil
}

func (m model) startDownload() (tea.Model, tea.Cmd) {
	w := max(m.numWorkers, 1)
	if len(m.dlEntries) == 0 {
		w = 1
	}
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
