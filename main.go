package main

import (
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

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

	sBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 2)
	sInput = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	sInputFocus = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("14")).
			Padding(0, 1)
)

const (
	barW      = 30
	boardBarW = 20
)

func renderBar(pct float64, w int) string {
	filled := min(int(float64(w)*pct/100), w)
	return sOk.Render(strings.Repeat("█", filled)) +
		sGray.Render(strings.Repeat("░", w-filled))
}

func arrow(active bool) string {
	if active {
		return sTitle.Render("▶ ")
	}
	return "  "
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

type (
	msgUpdateChecked   struct{ info *UpdateInfo }
	msgDepProgress     struct{ p FileProgress }
	msgDepDone         struct{ err error; isUpdate bool }
	msgPlaylistFetched struct {
		info *PlaylistInfo
		err  error
	}
	msgDlUpdate struct{ u DlUpdate }
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
	scr screen

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
	workers     int
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
	ti.SetWidth(58)

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
		workers:    1,
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
	return cmdCheckUpdate()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo = msg.info
		m.scr, m.menuCursor = scrUpdateReady, 1
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
		return m.afterDepInstall()
	case msgPlaylistFetched:
		if msg.err != nil || msg.info == nil {
			m.forceSingle, m.scr, m.menuCursor = true, scrQuality, 0
			return m, nil
		}
		m.plInfo = msg.info
		m.plPage, m.plCursor = 0, 0
		m.plSelected = map[int]bool{}
		m.scr = scrPlaylist
	case msgDlUpdate:
		return m.handleDlUpdate(msg.u)
	}

	switch {
	case m.scr == scrURL:
		var cmd tea.Cmd
		m.urlInput, cmd = m.urlInput.Update(msg)
		return m, cmd
	case m.scr == scrPlaylist && m.plInputMode:
		var cmd tea.Cmd
		m.plInput, cmd = m.plInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) gotoChecks() (tea.Model, tea.Cmd) {
	r := DetectDeps()
	if r.YtdlpVer == "" {
		m.depLabel, m.depProgress, m.scr = "yt-dlp", FileProgress{}, scrDepDl
		var cmd tea.Cmd
		m.depCh, cmd = launch(InstallYtDlp, false)
		return m, cmd
	}
	return m.afterDepInstall()
}

func (m model) afterDepInstall() (tea.Model, tea.Cmd) {
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
	switch u.Type {
	case EvStart:
		if u.Slot < len(m.slots) {
			m.slots[u.Slot] = slotState{title: trunc(u.Stem, 50)}
		}
	case EvDest:
		if u.Slot < len(m.slots) {
			m.slots[u.Slot].title = trunc(u.Stem, 50)
		}
	case EvProgress:
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.pct, s.doneB, s.totalB, s.speed, s.proc = u.Pct, u.DoneB, u.TotalB, u.Speed, false
		}
	case EvProc:
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.proc, s.label = true, u.Label
		}
	case EvFallback:
		if u.Slot < len(m.slots) {
			m.slots[u.Slot].label = u.Label
			m.slots[u.Slot].proc = true
		}
	case EvReset:
		if u.Slot < len(m.slots) {
			m.slots[u.Slot] = slotState{}
		}
	case EvDone:
		if u.OK {
			m.dlDone++
		} else {
			m.dlFailed++
		}
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.done, s.failed, s.pct = u.OK, !u.OK, 100
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

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.scr {
	case scrUpdateReady:
		switch k {
		case "up", "k":
			m.menuCursor = 0
		case "down", "j":
			m.menuCursor = 1
		case "y", "д":
			m.menuCursor = 0
			fallthrough
		case "enter":
			if m.menuCursor == 0 {
				m.scr, m.depProgress = scrUpdateDl, FileProgress{}
				info := m.updateInfo
				var cmd tea.Cmd
				m.depCh, cmd = launch(func(ch chan<- FileProgress) error {
					return ApplyUpdate(info, ch)
				}, true)
				return m, cmd
			}
			return m.gotoChecks()
		case "n", "н", "esc", "q":
			return m.gotoChecks()
		}

	case scrUpdateDone:
		return m, tea.Quit

	case scrFFmpegAsk:
		switch k {
		case "up", "k":
			m.menuCursor = 0
		case "down", "j":
			m.menuCursor = 1
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
			m.scr = scrURL
			return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
		case "n", "н", "q":
			m.scr = scrURL
			return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
		}

	case scrURL:
		switch k {
		case "enter":
			url := strings.TrimSpace(m.urlInput.Value())
			if url == "" {
				m.urlErr = "Ссылка не может быть пустой."
				return m, nil
			}
			if !YtRE.MatchString(url) {
				m.urlErr = "Не похоже на YouTube-ссылку."
				return m, nil
			}
			m.urlErr = ""
			m.url = url
			m.plInfo, m.dlEntries = nil, nil
			m.forceSingle, m.workers = false, 1
			if IsPlaylistURL(url) {
				if VideoInPlaylistRE.MatchString(url) {
					m.scr, m.menuCursor = scrPlaylistAsk, 0
					return m, nil
				}
				m.scr = scrPlaylistFetch
				return m, cmdFetchPlaylist(url)
			}
			m.scr, m.menuCursor = scrQuality, 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			return m, cmd
		}

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
		switch k {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < maxW-1 {
				m.menuCursor++
			}
		case "enter":
			m.workers = m.menuCursor + 1
			m.scr, m.menuCursor = scrQuality, 0
		}

	case scrQuality:
		switch k {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < 2 {
				m.menuCursor++
			}
		case "1":
			m.menuCursor = 0
			return m.startDownload()
		case "2":
			m.menuCursor = 1
			return m.startDownload()
		case "3":
			m.menuCursor = 2
			return m.startDownload()
		case "enter":
			return m.startDownload()
		}

	case scrSummary:
		switch k {
		case "up", "k", "y", "д":
			m.menuCursor = 0
		case "down", "j", "n", "н":
			m.menuCursor = 1
		case "enter":
			if m.menuCursor == 0 {
				return m.resetForNext()
			}
			return m, tea.Quit
		case "q":
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
			m.plInputMode, m.plInputErr = false, ""
			return m, nil
		case "esc":
			m.plInputMode, m.plInputErr = false, ""
			return m, nil
		default:
			var cmd tea.Cmd
			m.plInput, cmd = m.plInput.Update(msg)
			return m, cmd
		}
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
		m.dlEntries = sel
		m.menuCursor = 0
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
	workers := max(m.workers, 1)
	numSlots := workers
	if len(m.dlEntries) == 0 {
		numSlots = 1
	}
	m.slots = make([]slotState, numSlots)
	m.dlDone, m.dlFailed, m.dlTotal = 0, 0, len(m.dlEntries)
	ch := make(chan DlUpdate, 256)
	m.dlCh = ch
	m.scr = scrDownload
	StartDownload(m.cfg, m.url, m.forceSingle, m.plInfo, m.dlEntries, workers, ch)
	return m, cmdListenDl(ch)
}

func (m model) resetForNext() (tea.Model, tea.Cmd) {
	m.scr = scrURL
	m.url = ""
	m.urlInput.SetValue("")
	m.plInfo, m.dlEntries = nil, nil
	m.plSelected = map[int]bool{}
	m.forceSingle, m.workers = false, 1
	m.slots = nil
	m.dlDone, m.dlFailed, m.dlTotal = 0, 0, 0
	m.menuCursor = 0
	return m, tea.Batch(m.urlInput.Focus(), textinput.Blink)
}

func (m model) View() tea.View {
	var b strings.Builder
	b.WriteString(sBox.Render(
		sTitle.Render("VolRen  Video / Audio  Downloader")+"\n"+
			sGray.Render("версия "+Version+"  •  powered by yt-dlp"),
	) + "\n\n")

	switch m.scr {
	case scrUpdateCheck:
		b.WriteString(sGray.Render("  Проверяю обновления…") + "\n")

	case scrUpdateReady:
		b.WriteString(sOk.Render("  ✔  Доступна новая версия: ") +
			sBold.Render(m.updateInfo.Latest) +
			sGray.Render("  (текущая: "+Version+")") + "\n\n")
		b.WriteString(viewMenu("Обновить сейчас?", []string{"Да", "Нет"}, m.menuCursor))
		b.WriteString(sGray.Render("  [↑↓] выбрать  [Enter] / [y/n]") + "\n")

	case scrUpdateDl:
		b.WriteString(sTitle.Render("  Скачиваю обновление "+m.updateInfo.Latest+"…") + "\n\n")
		b.WriteString(viewProgress(m.depProgress))
		if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.depErr) + "\n")
		}

	case scrUpdateDone:
		b.WriteString(sOk.Render("  ✔  Обновление "+m.updateInfo.Latest+" применено.") + "\n\n")
		if IsWindows {
			b.WriteString("  Файл будет заменён после закрытия. Запустите вручную.\n")
		} else {
			b.WriteString("  Бинарник заменён. Перезапустите:\n\n")
			b.WriteString(sGray.Render("    ./VolRenDownloader") + "\n")
		}
		b.WriteString("\n" + sGray.Render("  [любая клавиша] выйти") + "\n")

	case scrFFmpegAsk:
		b.WriteString(sWarn.Render("  !  ffmpeg не найден") + "\n\n")
		b.WriteString("  Нужен для HD и MP3. Скачать (~80 МБ)?\n\n")
		b.WriteString(viewMenu("", []string{"Да, скачать", "Нет, пропустить"}, m.menuCursor))
		b.WriteString(sGray.Render("  [↑↓] выбрать  [Enter] / [y/n]") + "\n")

	case scrDepDl:
		b.WriteString(sTitle.Render("  Скачиваю "+m.depLabel+"…") + "\n\n")
		b.WriteString(viewProgress(m.depProgress))
		if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.depErr) + "\n")
		}

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
		b.WriteString("\n" + sGray.Render("  [Enter] подтвердить  [Ctrl+C] выход") + "\n")

	case scrPlaylistAsk:
		b.WriteString(sWarn.Render("  !  Ссылка содержит и видео, и плейлист.") + "\n\n")
		b.WriteString(viewMenu("", []string{"Только это видео", "Открыть плейлист"}, m.menuCursor))
		b.WriteString(sGray.Render("  [↑↓] / [1/2]  [Enter] выбрать") + "\n")

	case scrPlaylistFetch:
		b.WriteString(sGray.Render("  Загружаю плейлист…") + "\n")

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

	v := tea.NewView(b.String())
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader"
	return v
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
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  [%s]  %.1f%%\n", renderBar(p.Pct, barW), p.Pct))
	if p.DoneB > 0 {
		size := FmtBytes(p.DoneB)
		if p.TotalB > 0 {
			size += " / " + FmtBytes(p.TotalB)
		}
		b.WriteString("  " + size)
		if p.Speed != "" {
			b.WriteString("  " + sGray.Render(p.Speed))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func viewPlaylist(m model) string {
	var b strings.Builder
	info, total := m.plInfo, len(m.plInfo.Entries)
	b.WriteString(sBold.Render(fmt.Sprintf("  Плейлист: «%s»", info.Title)) +
		sGray.Render(fmt.Sprintf("  (%d видео)", total)) + "\n")
	b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n\n")

	start := m.plPage * m.plPageSize
	for i, e := range info.Entries[start:min(start+m.plPageSize, total)] {
		chk := sGray.Render("[ ] ")
		if m.plSelected[e.Index] {
			chk = sOk.Render("[✔] ")
		}
		b.WriteString(fmt.Sprintf("%s%s%s  %s  %s\n",
			arrow(start+i == m.plCursor), chk,
			sTitle.Render(fmt.Sprintf("%4d.", e.Index)),
			trunc(e.Title, 48),
			sGray.Render(FmtDuration(e.Duration))))
	}
	if pages := (total + m.plPageSize - 1) / m.plPageSize; pages > 1 {
		b.WriteString("\n" + sGray.Render(fmt.Sprintf("  стр. %d/%d", m.plPage+1, pages)) + "\n")
	}
	b.WriteString("\n")
	if m.plInputMode {
		b.WriteString("  " + sTitle.Render("Выбор:") + "\n" + sInputFocus.Render(m.plInput.View()) + "\n")
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  "+m.plInputErr) + "\n")
		}
	} else {
		if m.plInputErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.plInputErr) + "\n")
		}
		b.WriteString(sGray.Render(fmt.Sprintf("  Выбрано: %d/%d", len(m.plSelected), total)) + "\n\n")
		b.WriteString(sGray.Render("  [↑↓] навигация  [Пробел] выбрать  [a] все  [/] ввод  [Enter] далее") + "\n")
	}
	return b.String()
}

func viewWorkers(m model) string {
	var b strings.Builder
	maxW := min(len(m.dlEntries), 5)
	b.WriteString(sBold.Render("  Параллельная загрузка:") +
		sGray.Render(fmt.Sprintf("  (%d видео)", len(m.dlEntries))) + "\n\n")
	for i := range maxW {
		desc := fmt.Sprintf("%d потока(ов)", i+1)
		if i == 0 {
			desc = "Последовательно"
		}
		note := ""
		if i == 2 {
			note = sGray.Render("  (рекомендуется)")
		}
		b.WriteString(fmt.Sprintf("%s%s  %s%s\n",
			arrow(i == m.menuCursor),
			sTitle.Render(fmt.Sprintf("[%d]", i+1)),
			desc, note))
	}
	b.WriteString("\n" + sGray.Render("  [↑↓] навигация  [Enter] выбрать") + "\n")
	return b.String()
}

func viewQuality(m model) string {
	var b strings.Builder
	noFF := ""
	if FFmpegResolved == "" {
		noFF = sGray.Render("  [нужен ffmpeg]")
	}
	items := [3]struct{ label, note string }{
		{"Лучшее качество (HD / 4K)", noFF},
		{"Экономичное (360p)", ""},
		{"Только звук (MP3)", noFF},
	}
	b.WriteString(sBold.Render("  Выбери качество:") + "\n\n")
	for i, item := range items {
		b.WriteString(fmt.Sprintf("%s%s  %s%s\n",
			arrow(i == m.menuCursor),
			sTitle.Render(fmt.Sprintf("[%d]", i+1)),
			item.label, item.note))
	}
	b.WriteString("\n" + sGray.Render("  [↑↓] / [1/2/3]  [Enter] начать") + "\n")
	return b.String()
}

func viewDownload(m model) string {
	var b strings.Builder
	if m.dlTotal > 0 {
		b.WriteString(sBold.Render(fmt.Sprintf("  Плейлист  ·  %d видео", m.dlTotal)) + "\n")
		b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n\n")
		for i, s := range m.slots {
			badge := sTitle.Render(fmt.Sprintf("[%d]", i+1))
			switch {
			case s.done:
				b.WriteString(fmt.Sprintf("  %s  %s  %s\n       [%s]  %s\n",
					badge, sOk.Render("✔"), s.title,
					sOk.Render(strings.Repeat("█", boardBarW)), sGray.Render("готово")))
			case s.failed:
				b.WriteString(fmt.Sprintf("  %s  %s  %s\n%s\n",
					badge, sErr.Render("✘"), s.title, sErr.Render("       ошибка загрузки")))
			case s.proc:
				b.WriteString(fmt.Sprintf("  %s  %s\n       %s  %s\n",
					badge, sBold.Render(s.title), sWarn.Render("⚙"), sGray.Render(s.label)))
			case s.title != "":
				b.WriteString(fmt.Sprintf("  %s  %s\n       %s  [%s]  %s  %s\n",
					badge, sBold.Render(s.title),
					sTitle.Render("↓"), renderBar(s.pct, boardBarW),
					sBold.Render(fmt.Sprintf("%5.1f%%", s.pct)),
					fmtSize(s.doneB, s.totalB)+fmtSpeed(s.speed)))
			default:
				b.WriteString(fmt.Sprintf("  %s  %s\n\n", badge, sGray.Render("── ожидание ──")))
			}
		}
		pending := m.dlTotal - m.dlDone - m.dlFailed
		b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n")
		b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			sOk.Render(fmt.Sprintf("✔ %d", m.dlDone)),
			sErr.Render(fmt.Sprintf("✘ %d", m.dlFailed)),
			sGray.Render(fmt.Sprintf("◷ %d в очереди  │  %d/%d", pending, m.dlDone+m.dlFailed, m.dlTotal))))
	} else if len(m.slots) > 0 {
		s := m.slots[0]
		b.WriteString(sTitle.Render("  Загружаю…") + "\n\n")
		if s.title != "" {
			b.WriteString(sBold.Render("  "+s.title) + "\n")
		}
		if s.proc {
			b.WriteString(sWarn.Render("  ⚙  ") + sGray.Render(s.label) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s  [%s]  %s  %s\n",
				sTitle.Render("↓"), renderBar(s.pct, barW),
				sBold.Render(fmt.Sprintf("%5.1f%%", s.pct)),
				fmtSize(s.doneB, s.totalB)+fmtSpeed(s.speed)))
		}
	}
	return b.String()
}

func fmtSize(done, total int64) string {
	switch {
	case total > 0:
		return FmtBytes(done) + sGray.Render("/") + FmtBytes(total)
	case done > 0:
		return FmtBytes(done)
	default:
		return "…"
	}
}

func fmtSpeed(s string) string {
	if s == "" {
		return ""
	}
	return "  " + sGray.Render(s)
}

func viewSummary(m model) string {
	var b strings.Builder
	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render("  ✔  Готово! → "+DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render("  ✘  Не удалось скачать.") + "\n" +
				sGray.Render("     Попробуй: VolRenDownloader --update") + "\n\n")
		}
	} else {
		b.WriteString(sOk.Render("  ✔  ") +
			sBold.Render(fmt.Sprintf("Плейлист завершён · успешно: %d/%d",
				m.dlDone, m.dlTotal)) + "\n\n")
	}
	if len(m.session.Items) > 0 {
		sep := sGray.Render("  " + strings.Repeat("─", 54))
		b.WriteString(sBold.Render("  Итоги сессии:") + "\n" + sep + "\n")
		for _, item := range m.session.Items {
			badge := sOk.Render("OK  ")
			if !item.OK {
				badge = sErr.Render("FAIL")
			}
			b.WriteString(fmt.Sprintf("  [%s]  %-22s  %s\n",
				badge, trunc(item.Label, 22), sGray.Render(trunc(item.URL, 45))))
		}
		b.WriteString(sep + "\n")
		b.WriteString(fmt.Sprintf("  Всего: %s  ·  %s  ·  %s\n\n",
			sBold.Render(fmt.Sprintf("%d", m.session.Success+m.session.Failed)),
			sOk.Render(fmt.Sprintf("Успешно: %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("Ошибок: %d", m.session.Failed))))
	}
	b.WriteString(viewMenu("Скачать ещё?", []string{"Да", "Нет, выйти"}, m.menuCursor))
	b.WriteString(sGray.Render("  [↑↓] выбрать  [Enter] / [y/n]") + "\n")
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
	go func() {
		<-sigs
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	signal.Stop(sigs)
}
