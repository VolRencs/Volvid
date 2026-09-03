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
	m := newModel()
	if env != nil {
		m.env = env
	}
	if ctx != nil {
		m.baseCtx = ctx
	}
	return m
}

func newModel() Model {
	env := app.NewEnv()
	loc := app.LoadLocale(env)

	m := Model{
		env:         env,
		baseCtx:     context.Background(),
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		checkUpdateCmd(m.env),
	)
}

func (m Model) u() *app.UIStrings {
	return app.StringsFor(m.locale)
}

// ---------- operation contexts ----------

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

// clearOpCancel releases a finished operation's context. Call it once the
// matching result arrives, before starting any new operation.
func (m Model) clearOpCancel() Model {
	if m.opCancel != nil {
		m.opCancel()
		m.opCancel = nil
	}
	return m
}
