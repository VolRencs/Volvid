package tui

import (
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
	scrDepDl
	scrDepUpdate
	scrURL
	scrSearchInput
	scrSearchFetch
	scrSearchResults
	scrPlaylistAsk
	scrPlaylistFetch
	scrPlaylist
	scrFragmentProbe
	scrFragmentChoice
	scrFragmentInput
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
	inputFragment
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

type depScreenMode uint8

const (
	depModeStartup depScreenMode = iota + 1
	depModeManage
)

type depActionKind uint8

const (
	depActionInstall depActionKind = iota + 1
	depActionContinue
	depActionRefresh
	depActionBack
	depActionExit
)

type depAction struct {
	Kind  depActionKind
	Key   string
	Label string
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
	msgFragmentDuration struct {
		duration int
		err      error
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

	deps    app.CheckDepsResult
	depMode depScreenMode

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

	plInfo        *app.PlaylistInfo
	plCursor      int
	plTop         int
	plSelected    map[int]bool
	plInputMode   bool
	plInput       inputField
	plInputErr    string
	mediaDuration int
	fragment      *app.DownloadFragment
	fragmentErr   string
	fragmentIn    inputField

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
	downloadErr    string
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
		fragmentIn:  newInput(inputFragment, "1:00-2:30", 28, 32),
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		func() tea.Msg { return msgUpdateChecked{info: app.CheckUpdate()} },
	)
}

func (m Model) u() *app.UIStrings {
	return app.StringsFor(m.locale)
}
