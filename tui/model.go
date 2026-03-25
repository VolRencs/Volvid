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
	scrSearchInput
	scrSearchFetch
	scrSearchResults
	scrPlaylistAsk
	scrPlaylistFetch
	scrPlaylist
	scrMode
	scrAudio
	scrQualityFetch
	scrQuality
	scrWorkers
	scrDownload
	scrSummary
)

type inputTarget uint8

const (
	inputURL inputTarget = iota
	inputSearch
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
	msgPlaylistFetched struct {
		info *app.PlaylistInfo
		err  error
	}
	msgSearchResults struct {
		results []app.SearchResult
		err     error
	}
	msgQualityScanned struct {
		choices []app.QualityChoice
		err     error
	}
	msgDlUpdate struct{ update app.DlUpdate }

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

	urlInput      inputField
	urlErr        string
	target        app.ParsedTarget
	searchInput   inputField
	searchQuery   string
	searchErr     string
	searchResults []app.SearchResult

	plInfo      *app.PlaylistInfo
	plCursor    int
	plTop       int
	plSelected  map[int]bool
	plInputMode bool
	plInput     inputField
	plInputErr  string

	menu menu

	mode           app.DownloadMode
	profile        app.OutputProfile
	qualityChoices []app.QualityChoice
	audioProfiles  []app.OutputProfile
	flowErr        string
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
		screen:      scrUpdateCheck,
		locale:      loc,
		urlInput:    newInput(inputURL, "https://youtu.be/...", inputW, 300),
		searchInput: newInput(inputSearch, app.StringsFor(loc).SearchPlaceholder, inputW, 120),
		plInput:     newInput(inputPlaylist, app.StringsFor(loc).PlInputPlaceholder, 38, 100),
		mode:        app.DefaultDownloadMode(),
		profile:     app.DefaultVideoProfile(loc),
		numWorkers:  1,
		plSelected:  map[int]bool{},
	}
}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(ts time.Time) tea.Msg { return timerTickMsg(ts) })
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
		defer close(ch)

		progressCh := make(chan app.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			doneCh <- fn(progressCh)
		}()

		for progress := range progressCh {
			progress.Done = false
			progress.Err = nil
			ch <- progress
		}

		if err := <-doneCh; err != nil {
			ch <- app.FileProgress{Done: true, Err: err}
			return
		}

		ch <- app.FileProgress{Done: true}
	}()
	return ch, streamFileProgressCmd(ch, isUpdate)
}

func fetchPlaylistCmd(url string, l app.Locale) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(nil, url, l)
		return msgPlaylistFetched{info: info, err: err}
	}
}

func searchYouTubeCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := app.SearchYouTube(query)
		return msgSearchResults{results: results, err: err}
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

func (m Model) qualityProfileAt(idx int) app.OutputProfile {
	if idx < 0 || idx >= len(m.qualityChoices) {
		return app.OutputProfile{}
	}
	return m.qualityChoices[idx].Profile(m.locale)
}

func (m Model) audioOptions() []string {
	return app.OutputProfileLabels(m.audioProfiles)
}

func (m Model) audioProfileAt(idx int) app.OutputProfile {
	if idx < 0 || idx >= len(m.audioProfiles) {
		return app.OutputProfile{}
	}
	return m.audioProfiles[idx]
}

func (m Model) modeAt(idx int) app.DownloadMode {
	switch idx {
	case 1:
		return app.ModeAudio
	case 2:
		return app.ModeThumbnail
	default:
		return app.ModeVideo
	}
}

func (m Model) searchResultAt(idx int) app.SearchResult {
	if idx < 0 || idx >= len(m.searchResults) {
		return app.SearchResult{}
	}
	return m.searchResults[idx]
}

func (m Model) shouldAskWorkers() bool {
	return len(m.dlEntries) > 1
}

func (m Model) modeOptions() []string {
	u := m.u()
	return []string{u.ModeVideo, u.ModeAudio, u.ModeThumbnail}
}

func (m Model) sessionPlaylistSuffix(n int) string {
	return app.PlaylistSuffix(m.locale, n)
}

func (m Model) uiBusy() bool {
	switch m.screen {
	case scrUpdateDl, scrDepDl, scrDepUpdate, scrDownload, scrPlaylistFetch, scrQualityFetch, scrSearchFetch:
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
	case scrMode:
		m.menu.SetItems(m.modeOptions())
	case scrAudio:
		m.menu.SetItems(m.audioOptions())
	case scrSearchResults:
		m.menu.SetItems(m.searchResultOptions())
	case scrSummary:
		m.menu.SetItems([]string{u.MenuAgainY, u.MenuAgainN})
	case scrQuality:
		m.menu.SetItems(m.qualityOptions())
	case scrWorkers:
		m.menu.SetItems(m.workerMenuOptions(min(len(m.dlEntries), 5)))
	default:
		m.menu.SetItems(nil)
	}
	return m
}

func (m *Model) syncLocalizedInputs() {
	m.searchInput.SetPlaceholder(m.u().SearchPlaceholder)
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
}

func (m Model) searchResultOptions() []string {
	options := make([]string, 0, len(m.searchResults))
	for _, result := range m.searchResults {
		label := trunc(result.Title, 42)
		if result.Duration > 0 {
			label += "  " + app.FmtDuration(result.Duration)
		}
		options = append(options, label)
	}
	return options
}
