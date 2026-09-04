package tui

import (
	"context"
	"time"

	app "volvid/internal/app"

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
	scrVideoOutput
	scrWorkers
	scrDownload
	scrSummary
)

type Model struct {
	env    *app.Env
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
	depGen      int

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

	menuDigits       string
	menuDigitsScreen screen

	mode           app.DownloadMode
	profile        app.OutputProfile
	qualityChoices []app.QualityChoice
	videoProfiles  []app.OutputProfile
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

	session         app.Session
	depReturnScreen screen
	depRefreshing   bool
	depRefreshToken int
	depUpdateDone   bool

	baseCtx  context.Context
	opCancel context.CancelFunc
	opGen    int

	dlCancel    context.CancelFunc
	depCancel   context.CancelFunc
	dlCancelled bool
	dlGen       int
}

func New(env *app.Env, ctx context.Context) tea.Model {
	if env == nil {
		env = app.NewEnv()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return newModel(env, ctx)
}

func newModel(env *app.Env, ctx context.Context) Model {
	loc := app.LoadLocale(env)

	m := Model{
		env:         env,
		baseCtx:     ctx,
		screen:      scrUpdateCheck,
		locale:      loc,
		urlInput:    newInput(inputURL, "https://youtu.be/...", inputW, 300),
		searchInput: newInput(inputSearch, app.StringsFor(loc).SearchPlaceholder, inputW, 120),
		plInput:     newInput(inputPlaylist, app.StringsFor(loc).PlInputPlaceholder, 38, 100),
		fragmentIn:  newInput(inputFragment, "1:00-2:30", 28, 32),
		mode:        app.ModeVideo,
		profile:     app.DefaultVideoProfile(loc),
		numWorkers:  1,
		plSelected:  map[int]bool{},
	}
	m.syncLayout()
	return m
}

func newTestModel() Model {
	return newModel(app.NewEnv(), context.Background())
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		checkUpdateCmd(m.env),
	)
}

func (m Model) u() *app.UIStrings {
	return app.StringsFor(m.locale)
}

func (m Model) spinnerVisible() bool {
	return m.screen.props().spinner
}

type screenProps struct {
	spinner  bool
	menu     bool
	busy     bool
	updating bool
}

func (s screen) props() screenProps {
	switch s {
	case scrUpdateCheck:
		return screenProps{spinner: true, busy: true, updating: true}
	case scrUpdateReady:
		return screenProps{menu: true, updating: true}
	case scrUpdateDl:
		return screenProps{busy: true, updating: true}
	case scrUpdateDone:
		return screenProps{updating: true}
	case scrDepDl:
		return screenProps{busy: true}
	case scrDepUpdate:
		return screenProps{menu: true, busy: true}
	case scrSearchFetch, scrPlaylistFetch, scrFragmentProbe, scrQualityFetch:
		return screenProps{spinner: true, busy: true}
	case scrPlaylistAsk, scrSearchResults, scrFragmentChoice, scrMode, scrAudio, scrQuality, scrVideoOutput, scrWorkers, scrSummary:
		return screenProps{menu: true}
	case scrDownload:
		return screenProps{busy: true}
	default:
		return screenProps{}
	}
}

func (m Model) cancelOps() Model {
	m.opGen++
	if m.opCancel != nil {
		m.opCancel()
		m.opCancel = nil
	}
	return m
}

func (m Model) nextOpCtx() (Model, context.Context) {
	m = m.cancelOps()
	ctx, cancel := context.WithCancel(m.baseCtx)
	m.opCancel = cancel
	return m, ctx
}

func (m Model) clearOpCancel() Model {
	if m.opCancel != nil {
		m.opCancel()
		m.opCancel = nil
	}
	return m
}
