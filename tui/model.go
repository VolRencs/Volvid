package tui

import (
	"fmt"
	"time"

	app "YouTubeBuild/internal/app"

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
	scrQualityFetch
	scrQuality
	scrDownload
	scrSummary
)

type inputTarget uint8

const (
	inputURL inputTarget = iota
	inputPlaylist
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
	msgUpdateChecked struct{ info *app.UpdateInfo }
	msgDepProgress   struct{ progress app.FileProgress }
	msgDepDone       struct {
		err      error
		isUpdate bool
	}
	msgUpdateRestart   struct{}
	msgPlaylistFetched struct {
		info *app.PlaylistInfo
		err  error
	}
	msgQualityScanned struct {
		choices []app.QualityChoice
		err     error
	}
	msgDlUpdate       struct{ update app.DlUpdate }
	msgClipboardPaste struct {
		target  inputTarget
		content string
		err     error
	}

	spinnerTickMsg struct{}
	timerTickMsg   time.Time
	cursorBlinkMsg struct {
		target inputTarget
		tag    int
	}
)

type Model struct {
	screen screen

	width  int
	height int

	locale app.Locale

	spinnerFrame int

	ytdlpVer  string
	ffmpegVer string

	updateInfo  *app.UpdateInfo
	depProgress app.FileProgress
	depLabel    string
	depErr      string
	depCh       <-chan app.FileProgress

	urlInput inputField
	urlErr   string

	plInfo      *app.PlaylistInfo
	plCursor    int
	plTop       int
	plSelected  map[int]bool
	plInputMode bool
	plInput     inputField
	plInputErr  string

	menu menu

	cfg            app.QualityConfig
	qualityChoices []app.QualityChoice
	url            string
	dlEntries      []app.PlaylistEntry
	forceSingle    bool
	numWorkers     int
	dlCh           <-chan app.DlUpdate
	slots          []slotState
	dlDone         int
	dlFailed       int
	dlTotal        int
	singleOK       bool
	dlStartedAt    time.Time
	dlElapsed      time.Duration
	timerActive    bool

	session       app.Session
	prevScreen    screen
	depUpdateDone bool
}

func New() tea.Model {
	return newModel()
}

func newModel() Model {
	loc := app.LoadLocale()
	app.SyncLoc(loc)

	return Model{
		screen:     scrUpdateCheck,
		locale:     loc,
		urlInput:   newInput(inputURL, "https://youtu.be/...", inputW, 300),
		plInput:    newInput(inputPlaylist, app.StringsFor(loc).PlInputPlaceholder, 38, 100),
		numWorkers: 1,
		plSelected: map[int]bool{},
	}
}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(ts time.Time) tea.Msg { return timerTickMsg(ts) })
}

func updateRestartCmd() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(time.Time) tea.Msg { return msgUpdateRestart{} })
}

func streamFileProgressCmd(ch <-chan app.FileProgress, isUpdate bool) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{isUpdate: isUpdate}
		}
		if p.Done {
			return msgDepDone{err: p.Err, isUpdate: isUpdate}
		}
		return msgDepProgress{progress: p}
	}
}

func launchProgress(fn func(chan<- app.FileProgress) error, isUpdate bool) (<-chan app.FileProgress, tea.Cmd) {
	ch := make(chan app.FileProgress, 16)
	go func() {
		if err := fn(ch); err != nil {
			ch <- app.FileProgress{Done: true, Err: err}
		}
		close(ch)
	}()
	return ch, streamFileProgressCmd(ch, isUpdate)
}

func fetchPlaylistCmd(url string, l app.Locale) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(nil, url, l)
		return msgPlaylistFetched{info: info, err: err}
	}
}

func loadQualityChoicesCmd(urls []string) tea.Cmd {
	return func() tea.Msg {
		choices, err := app.ResolveQualityChoices(urls)
		return msgQualityScanned{choices: choices, err: err}
	}
}

func listenDownloadCmd(ch <-chan app.DlUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{update: app.DlUpdate{Type: app.EvClosed}}
		}
		return msgDlUpdate{update: u}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		func() tea.Msg { return msgUpdateChecked{info: app.CheckUpdate()} },
	)
}

func (m Model) u() *app.UIStrings {
	return app.StringsFor(m.locale)
}

func (m Model) qualityOptions() []string {
	return app.QualityChoiceLabels(m.qualityChoices, m.locale)
}

func (m Model) qualityConfigAt(idx int) app.QualityConfig {
	if idx < 0 || idx >= len(m.qualityChoices) {
		return app.QualityConfig{}
	}
	return m.qualityChoices[idx].Config(m.locale)
}

func (m Model) sessionPlaylistSuffix(n int) string {
	return app.PlaylistSuffix(m.locale, n)
}

func (m Model) uiBusy() bool {
	switch m.screen {
	case scrUpdateDl, scrDepDl, scrDepUpdate, scrDownload, scrPlaylistFetch, scrQualityFetch:
		return true
	}
	return false
}

func (m Model) workerMenuOptions(n int) []string {
	u := m.u()
	opts := make([]string, n)
	opts[0] = u.WorkerSeq
	for i := 1; i < n; i++ {
		opts[i] = fmt.Sprintf(u.WorkerNFmt, i+1)
	}
	return opts
}

func (m Model) syncMenu() Model {
	u := m.u()
	switch m.screen {
	case scrUpdateReady:
		m.menu.SetItems([]string{u.MenuUpdateY, u.MenuUpdateN})
	case scrFFmpegAsk:
		m.menu.SetItems([]string{u.MenuFFmpegY, u.MenuFFmpegN})
	case scrPlaylistAsk:
		m.menu.SetItems([]string{u.MenuVidOnly, u.MenuOpenPl})
	case scrSummary:
		m.menu.SetItems([]string{u.MenuAgainY, u.MenuAgainN})
	case scrWorkers:
		m.menu.SetItems(m.workerMenuOptions(min(len(m.dlEntries), 5)))
	case scrQuality:
		m.menu.SetItems(m.qualityOptions())
	default:
		m.menu.SetItems(nil)
	}
	return m
}

func (m *Model) syncLocalizedInputs() {
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
}
