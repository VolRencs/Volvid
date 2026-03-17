package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────
//  Стили
// ─────────────────────────────────────────────

var (
	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	sOk     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	sErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	sWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	sGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sWhite  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	sBold   = lipgloss.NewStyle().Bold(true)
	sCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))

	sBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 2)
	sInputNormal = lipgloss.NewStyle().
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

func bar(pct float64, w int, color string) string {
	filled := int(float64(w) * pct / 100)
	if filled > w {
		filled = w
	}
	c := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	g := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return c.Render(strings.Repeat("█", filled)) + g.Render(strings.Repeat("░", w-filled))
}

func cursor(active bool) string {
	if active {
		return sCursor.Render("▶ ")
	}
	return "  "
}

func trunc(s string, n int) string {
	r := []rune(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ─────────────────────────────────────────────
//  Типы сообщений
// ─────────────────────────────────────────────

type (
	msgUpdateChecked  struct{ info *UpdateInfo }
	msgDepProgress    struct{ p FileProgress }
	msgDepDone        struct{ err error }
	msgPlaylistFetched struct {
		info *PlaylistInfo
		err  error
	}
	msgDlUpdate struct{ u DlUpdate }
)

// ─────────────────────────────────────────────
//  Экраны
// ─────────────────────────────────────────────

type screen int

const (
	scrUpdateCheck  screen = iota // проверка обновлений
	scrUpdateReady               // есть обновление
	scrUpdateDl                  // скачивание обновления
	scrChecks                    // проверка зависимостей
	scrFFmpegAsk                 // спросить про ffmpeg
	scrDepDl                     // скачивание зависимости
	scrURL                       // ввод ссылки
	scrPlaylistAsk               // видео или плейлист?
	scrPlaylistFetch             // загрузка плейлиста
	scrPlaylist                  // выбор видео
	scrWorkers                   // количество потоков
	scrQuality                   // выбор качества
	scrDownload                  // процесс загрузки
	scrSummary                   // итоги
)

// ─────────────────────────────────────────────
//  Состояние слота
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Модель
// ─────────────────────────────────────────────

type model struct {
	scr    screen
	width  int

	updateInfo  *UpdateInfo
	depProgress FileProgress
	depLabel    string
	depErr      string
	checkResult CheckDepsResult

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
	plSelected2 []PlaylistEntry
	forceSingle bool
	workers     int

	menuCursor int

	dlCh      <-chan DlUpdate
	slots     []slotState
	dlDone    int
	dlFailed  int
	dlTotal   int
	singleOK  bool

	session Session
}

func newModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://youtu.be/..."
	ti.CharLimit = 300
	ti.Width = 58
	ti.Focus()

	pi := textinput.New()
	pi.Placeholder = "1-5, 2,4,7 или а (все)"
	pi.CharLimit = 100
	pi.Width = 40

	return model{
		scr:        scrUpdateCheck,
		urlInput:   ti,
		plInput:    pi,
		plPageSize: 12,
		plSelected: map[int]bool{},
		workers:    1,
	}
}

// ─────────────────────────────────────────────
//  Команды Bubble Tea
// ─────────────────────────────────────────────

func cmdCheckUpdate() tea.Cmd {
	return func() tea.Msg { return msgUpdateChecked{CheckUpdate()} }
}

func cmdCheckDeps() tea.Cmd {
	return func() tea.Msg {
		r := DetectDeps()
		return msgDepDone{func() error {
			if r.YtdlpVer == "" || (IsWindows && r.FFmpegMissing) {
				return nil
			}
			return nil
		}()}
	}
}

func cmdStreamDep(ch <-chan FileProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{nil}
		}
		if p.Done {
			return msgDepDone{p.Err}
		}
		return msgDepProgress{p}
	}
}

func cmdInstallYtdlp() tea.Cmd {
	ch := make(chan FileProgress, 8)
	go func() {
		err := InstallYtDlp(ch)
		if err != nil {
			ch <- FileProgress{Done: true, Err: err}
		}
		close(ch)
	}()
	return cmdStreamDep(ch)
}

func cmdInstallFFmpeg() tea.Cmd {
	ch := make(chan FileProgress, 8)
	go func() {
		err := InstallFFmpeg(ch)
		if err != nil {
			ch <- FileProgress{Done: true, Err: err}
		}
		close(ch)
	}()
	return cmdStreamDep(ch)
}

func cmdStreamUpdate(info *UpdateInfo) tea.Cmd {
	ch := make(chan FileProgress, 8)
	go func() {
		err := ApplyUpdate(info, ch)
		if err != nil {
			ch <- FileProgress{Done: true, Err: err}
		}
		close(ch)
	}()
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{nil}
		}
		if p.Done {
			return msgDepDone{p.Err}
		}
		return msgDepProgress{p}
	}
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
			return msgDlUpdate{DlUpdate{Type: "closed"}}
		}
		return msgDlUpdate{u}
	}
}

// ─────────────────────────────────────────────
//  Init
// ─────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tea.Batch(cmdCheckUpdate(), textinput.Blink)
}

// ─────────────────────────────────────────────
//  Update
// ─────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case msgUpdateChecked:
		if msg.info == nil {
			return m.gotoChecks()
		}
		m.updateInfo = msg.info
		m.scr = scrUpdateReady
		return m, nil

	case msgDepProgress:
		m.depProgress = msg.p
		return m, cmdStreamDep(nil)

	case msgDepDone:
		m.depErr = ""
		if msg.err != nil {
			m.depErr = msg.err.Error()
			return m, nil
		}
		return m.afterDepInstall()

	case msgPlaylistFetched:
		if msg.err != nil || msg.info == nil {
			m.forceSingle = true
			m.scr = scrQuality
			m.menuCursor = 0
			return m, nil
		}
		m.plInfo = msg.info
		m.plPage, m.plCursor = 0, 0
		m.plSelected = map[int]bool{}
		m.scr = scrPlaylist
		return m, nil

	case msgDlUpdate:
		return m.handleDlUpdate(msg.u)
	}
	return m, nil
}

func (m model) gotoChecks() (tea.Model, tea.Cmd) {
	m.scr = scrChecks
	r := DetectDeps()
	m.checkResult = r
	if r.YtdlpVer == "" {
		m.depLabel = "yt-dlp"
		m.depProgress = FileProgress{}
		m.scr = scrDepDl
		return m, cmdInstallYtdlp()
	}
	if IsWindows && r.FFmpegMissing {
		m.scr = scrFFmpegAsk
		return m, nil
	}
	m.scr = scrURL
	return m, nil
}

func (m model) afterDepInstall() (tea.Model, tea.Cmd) {
	m.checkResult = DetectDeps()
	if m.depLabel == "yt-dlp" && IsWindows && m.checkResult.FFmpegMissing {
		m.scr = scrFFmpegAsk
		return m, nil
	}
	m.scr = scrURL
	return m, nil
}

func (m model) handleDlUpdate(u DlUpdate) (tea.Model, tea.Cmd) {
	if u.Type == "closed" {
		return m, nil
	}
	switch u.Type {
	case "start":
		if u.Slot < len(m.slots) {
			m.slots[u.Slot] = slotState{title: trunc(u.Stem, 50)}
		}
	case "dest":
		if u.Slot < len(m.slots) {
			m.slots[u.Slot].title = trunc(u.Stem, 50)
		}
	case "dl":
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.pct, s.doneB, s.totalB, s.speed, s.proc = u.Pct, u.DoneB, u.TotalB, u.Speed, false
		}
	case "proc":
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.proc, s.label = true, u.Label
		}
	case "reset":
		if u.Slot < len(m.slots) {
			m.slots[u.Slot] = slotState{}
		}
	case "done":
		if u.OK {
			m.dlDone++
		} else {
			m.dlFailed++
		}
		if m.dlTotal == 0 {
			m.singleOK = u.OK
			m.recordSession(u.OK)
			m.scr = scrSummary
			m.menuCursor = 0
			return m, nil
		}
		if u.Slot < len(m.slots) {
			s := &m.slots[u.Slot]
			s.done, s.failed, s.pct = u.OK, !u.OK, 100
		}
		if m.dlDone+m.dlFailed >= m.dlTotal {
			m.recordSession(m.dlFailed == 0)
			m.scr = scrSummary
			m.menuCursor = 0
			return m, nil
		}
	}
	return m, cmdListenDl(m.dlCh)
}

func (m *model) recordSession(ok bool) {
	label := m.cfg.Label
	if m.dlTotal > 0 {
		label += fmt.Sprintf(" [плейлист/%d]", m.dlTotal)
	}
	m.session.Record(label, m.url, ok)
}

// ─────────────────────────────────────────────
//  Обработка клавиш
// ─────────────────────────────────────────────

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if k == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.scr {

	case scrUpdateReady:
		switch k {
		case "y", "д", "enter":
			m.scr = scrUpdateDl
			m.depProgress = FileProgress{}
			return m, cmdStreamUpdate(m.updateInfo)
		case "n", "н", "esc", "q":
			return m.gotoChecks()
		}

	case scrUpdateDl:

	case scrFFmpegAsk:
		switch k {
		case "y", "д", "enter":
			m.depLabel = "ffmpeg"
			m.depProgress = FileProgress{}
			m.scr = scrDepDl
			return m, cmdInstallFFmpeg()
		case "n", "н", "q":
			m.scr = scrURL
		}

	case scrDepDl:

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
			m.plInfo = nil
			m.plSelected2 = nil
			m.forceSingle = false
			m.workers = 1
			if IsPlaylistURL(url) {
				if VideoInPlaylistRE.MatchString(url) {
					m.scr = scrPlaylistAsk
					m.menuCursor = 0
					return m, nil
				}
				m.scr = scrPlaylistFetch
				return m, cmdFetchPlaylist(url)
			}
			m.scr = scrQuality
			m.menuCursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			return m, cmd
		}

	case scrPlaylistAsk:
		switch k {
		case "1", "up", "k":
			m.menuCursor = 0
		case "2", "down", "j":
			m.menuCursor = 1
		case "enter":
			if m.menuCursor == 0 {
				m.forceSingle = true
				m.scr = scrQuality
				m.menuCursor = 0
			} else {
				m.scr = scrPlaylistFetch
				return m, cmdFetchPlaylist(m.url)
			}
		}

	case scrPlaylist:
		return m.handlePlaylistKey(k)

	case scrWorkers:
		maxW := min5(len(m.plSelected2))
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
			m.scr = scrQuality
			m.menuCursor = 0
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

func (m model) handlePlaylistKey(k string) (tea.Model, tea.Cmd) {
	if m.plInputMode {
		switch k {
		case "enter":
			raw := m.plInput.Value()
			indices, err := ParseSelection(raw, len(m.plInfo.Entries))
			if err != nil {
				m.plInputErr = err.Error()
				return m, nil
			}
			m.plSelected = map[int]bool{}
			for _, i := range indices {
				m.plSelected[i] = true
			}
			m.plInputMode = false
			m.plInputErr = ""
			return m, nil
		case "esc":
			m.plInputMode = false
			m.plInputErr = ""
			return m, nil
		default:
			var cmd tea.Cmd
			m.plInput, cmd = m.plInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
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
	case " ":
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
			for _, e := range m.plInfo.Entries {
				m.plSelected[e.Index] = true
			}
		}
	case "/":
		m.plInputMode = true
		m.plInput.SetValue("")
		m.plInput.Focus()
		return m, textinput.Blink
	case "enter":
		if len(m.plSelected) == 0 {
			m.plInputErr = "Выбери хотя бы одно видео."
			return m, nil
		}
		var sel []PlaylistEntry
		for _, e := range m.plInfo.Entries {
			if m.plSelected[e.Index] {
				sel = append(sel, e)
			}
		}
		m.plSelected2 = sel
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
	workers := m.workers
	if workers < 1 {
		workers = 1
	}
	numSlots := 1
	if len(m.plSelected2) > 0 {
		numSlots = workers
	}
	m.slots = make([]slotState, numSlots)
	m.dlDone, m.dlFailed = 0, 0
	m.dlTotal = len(m.plSelected2)

	ch := make(chan DlUpdate, 256)
	m.dlCh = ch
	m.scr = scrDownload

	StartDownload(m.cfg, m.url, m.forceSingle, m.plInfo, m.plSelected2, workers, ch)
	return m, cmdListenDl(ch)
}

func (m model) resetForNext() (tea.Model, tea.Cmd) {
	m.scr = scrURL
	m.url = ""
	m.urlInput.SetValue("")
	m.urlInput.Focus()
	m.plInfo = nil
	m.plSelected = map[int]bool{}
	m.plSelected2 = nil
	m.forceSingle = false
	m.workers = 1
	m.slots = nil
	m.dlDone, m.dlFailed, m.dlTotal = 0, 0, 0
	m.menuCursor = 0
	return m, textinput.Blink
}

func min5(n int) int {
	if n > 5 {
		return 5
	}
	return n
}

// ─────────────────────────────────────────────
//  View
// ─────────────────────────────────────────────

func (m model) View() string {
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
		b.WriteString(viewYesNo("Обновить сейчас?", m.menuCursor))

	case scrUpdateDl:
		b.WriteString(sTitle.Render("  Скачиваю обновление…") + "\n\n")
		b.WriteString(viewFileProgress(m.depProgress))

	case scrChecks:
		b.WriteString(sGray.Render("  Проверяю зависимости…") + "\n")

	case scrFFmpegAsk:
		b.WriteString(sWarn.Render("  !  ffmpeg не найден") + "\n\n")
		b.WriteString("  Нужен для HD и MP3. Скачать (~80 МБ)?\n\n")
		b.WriteString(viewYesNo("Скачать ffmpeg?", m.menuCursor))

	case scrDepDl:
		b.WriteString(sTitle.Render("  Скачиваю "+m.depLabel+"…") + "\n\n")
		b.WriteString(viewFileProgress(m.depProgress))
		if m.depErr != "" {
			b.WriteString(sErr.Render("  ✘  "+m.depErr) + "\n")
		}

	case scrURL:
		b.WriteString(sBold.Render("  Вставь ссылку:") + "\n\n")
		style := sInputNormal
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
		for i, opt := range []string{"Только это видео", "Открыть плейлист"} {
			b.WriteString(cursor(i == m.menuCursor) + sTitle.Render(fmt.Sprintf("[%d]", i+1)) + "  " + opt + "\n")
		}

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

	return b.String()
}

// ─────────────────────────────────────────────
//  Вспомогательные View
// ─────────────────────────────────────────────

func viewYesNo(question string, sel int) string {
	var b strings.Builder
	b.WriteString("  " + sBold.Render(question) + "\n\n")
	for i, opt := range []string{"Да", "Нет"} {
		b.WriteString(cursor(i == sel) + opt + "\n")
	}
	b.WriteString("\n" + sGray.Render("  [↑↓] выбрать  [Enter] подтвердить  [y/n]") + "\n")
	return b.String()
}

func viewFileProgress(p FileProgress) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  [%s]  %s%.1f%%%s\n",
		bar(p.Pct, barW, "10"), sBold.Render(""), p.Pct, ""))
	if p.TotalB > 0 {
		b.WriteString(fmt.Sprintf("  %s / %s\n", FmtBytes(p.DoneB), FmtBytes(p.TotalB)))
	}
	return b.String()
}

func viewPlaylist(m model) string {
	var b strings.Builder
	info := m.plInfo
	total := len(info.Entries)

	b.WriteString(sBold.Render(fmt.Sprintf("  Плейлист: «%s»", info.Title)) +
		sGray.Render(fmt.Sprintf("  (%d видео)", total)) + "\n")
	b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n\n")

	start := m.plPage * m.plPageSize
	end := start + m.plPageSize
	if end > total {
		end = total
	}
	for i, e := range info.Entries[start:end] {
		absIdx := start + i
		cur := cursor(absIdx == m.plCursor)
		chk := sGray.Render("[ ] ")
		if m.plSelected[e.Index] {
			chk = sOk.Render("[✔] ")
		}
		b.WriteString(fmt.Sprintf("%s%s%s  %s  %s\n",
			cur, chk,
			sTitle.Render(fmt.Sprintf("%4d.", e.Index)),
			trunc(e.Title, 48),
			sGray.Render(FmtDuration(e.Duration))))
	}

	pages := (total + m.plPageSize - 1) / m.plPageSize
	if pages > 1 {
		b.WriteString("\n" + sGray.Render(fmt.Sprintf("  стр. %d/%d", m.plPage+1, pages)) + "\n")
	}

	b.WriteString("\n")
	if m.plInputMode {
		b.WriteString("  " + sTitle.Render("Выбор:") + "\n")
		b.WriteString(sInputFocus.Render(m.plInput.View()) + "\n")
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
	maxW := min5(len(m.plSelected2))
	b.WriteString(sBold.Render("  Параллельная загрузка:") +
		sGray.Render(fmt.Sprintf("  (%d видео)", len(m.plSelected2))) + "\n\n")
	for i := 1; i <= maxW; i++ {
		desc := fmt.Sprintf("%d потока(ов)", i)
		if i == 1 {
			desc = "Последовательно"
		}
		note := ""
		if i == 3 {
			note = sGray.Render("  (рекомендуется)")
		}
		b.WriteString(fmt.Sprintf("%s%s  %s%s\n",
			cursor(i-1 == m.menuCursor),
			sTitle.Render(fmt.Sprintf("[%d]", i)),
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
			cursor(i == m.menuCursor),
			sTitle.Render(fmt.Sprintf("[%d]", i+1)),
			item.label, item.note))
	}
	b.WriteString("\n" + sGray.Render("  [↑↓] или [1/2/3] выбрать  [Enter] далее") + "\n")
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
				b.WriteString(fmt.Sprintf("  %s  %s  %s\n", badge, sOk.Render("✔"), s.title))
				b.WriteString(fmt.Sprintf("       [%s]  %s\n",
					sOk.Render(strings.Repeat("█", boardBarW)),
					sGray.Render("готово")))
			case s.failed:
				b.WriteString(fmt.Sprintf("  %s  %s  %s\n", badge, sErr.Render("✘"), s.title))
				b.WriteString(sErr.Render("       ошибка загрузки") + "\n")
			case s.proc:
				b.WriteString(fmt.Sprintf("  %s  %s\n", badge, sBold.Render(s.title)))
				b.WriteString(fmt.Sprintf("       %s  %s\n", sWarn.Render("⚙"), sGray.Render(s.label)))
			case s.title != "":
				b.WriteString(fmt.Sprintf("  %s  %s\n", badge, sBold.Render(s.title)))
				b.WriteString(fmt.Sprintf("       %s  [%s]  %s  %s\n",
					sTitle.Render("↓"),
					bar(s.pct, boardBarW, "10"),
					sBold.Render(fmt.Sprintf("%5.1f%%", s.pct)),
					viewSize(s.doneB, s.totalB)+viewSpeed(s.speed)))
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
				sTitle.Render("↓"),
				bar(s.pct, barW, "10"),
				sBold.Render(fmt.Sprintf("%5.1f%%", s.pct)),
				viewSize(s.doneB, s.totalB)+viewSpeed(s.speed)))
		}
	}
	return b.String()
}

func viewSize(done, total int64) string {
	if total > 0 {
		return FmtBytes(done) + sGray.Render("/") + FmtBytes(total)
	}
	if done > 0 {
		return FmtBytes(done)
	}
	return "…"
}

func viewSpeed(speed string) string {
	if speed == "" {
		return ""
	}
	return "  " + sGray.Render(speed)
}

func viewSummary(m model) string {
	var b strings.Builder

	if m.dlTotal == 0 {
		if m.singleOK {
			b.WriteString(sOk.Render("  ✔  Готово! → "+DlDir) + "\n\n")
		} else {
			b.WriteString(sErr.Render("  ✘  Не удалось скачать.") + "\n")
			b.WriteString(sGray.Render("     Попробуй: VolRenDownloader --update") + "\n\n")
		}
	} else {
		ok := m.dlTotal - m.dlFailed
		b.WriteString(sOk.Render("  ✔  ") +
			sBold.Render(fmt.Sprintf("Плейлист завершён · успешно: %d/%d", ok, m.dlTotal)) + "\n\n")
	}

	if len(m.session.Items) > 0 {
		b.WriteString(sBold.Render("  Итоги сессии:") + "\n")
		b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n")
		for _, item := range m.session.Items {
			badge := sOk.Render("OK  ")
			if !item.OK {
				badge = sErr.Render("FAIL")
			}
			url := item.URL
			if len(url) > 45 {
				url = url[:45] + "…"
			}
			b.WriteString(fmt.Sprintf("  [%s]  %-22s  %s\n",
				badge, trunc(item.Label, 22), sGray.Render(url)))
		}
		b.WriteString(sGray.Render("  "+strings.Repeat("─", 54)) + "\n")
		b.WriteString(fmt.Sprintf("  Всего: %s  ·  %s  ·  %s\n\n",
			sBold.Render(fmt.Sprintf("%d", m.session.Success+m.session.Failed)),
			sOk.Render(fmt.Sprintf("Успешно: %d", m.session.Success)),
			sErr.Render(fmt.Sprintf("Ошибок: %d", m.session.Failed))))
	}

	b.WriteString(sBold.Render("  Скачать ещё?") + "\n\n")
	for i, opt := range []string{"Да", "Нет, выйти"} {
		b.WriteString(cursor(i == m.menuCursor) + opt + "\n")
	}
	b.WriteString("\n" + sGray.Render("  [↑↓] выбрать  [Enter] / [y/n] подтвердить") + "\n")
	return b.String()
}

// ─────────────────────────────────────────────
//  main
// ─────────────────────────────────────────────

func main() {
	args := os.Args[1:]

	if contains(args, "--update") {
		fmt.Println("Обновляю зависимости…")
		ch := make(chan FileProgress, 8)
		go func() { InstallYtDlp(ch); close(ch) }()
		for range ch {
		}
		if IsWindows {
			ch2 := make(chan FileProgress, 8)
			go func() { InstallFFmpeg(ch2); close(ch2) }()
			for range ch2 {
			}
		}
		fmt.Println("Готово.")
		return
	}

	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
