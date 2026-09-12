package tui

import (
	"context"
	"time"
	"volvid/internal/core"
	"volvid/internal/i18n"

	"volvid/internal/adapters"

	tea "charm.land/bubbletea/v2"
)

type Model struct {
	api    AppAPI
	screen screen

	width  int
	height int

	locale core.Locale

	spinnerFrame int

	deps    core.CheckDepsResult
	depMode depScreenMode

	updateInfo  *core.UpdateInfo
	depProgress core.FileProgress
	depLabel    string
	depErr      string
	depCh       <-chan core.FileProgress
	depGen      int

	urlInput      inputField
	urlErr        string
	target        core.ParsedTarget
	searchInput   inputField
	searchQuery   string
	searchErr     string
	searchResults []core.SearchResult

	plInfo        *core.PlaylistInfo
	plCursor      int
	plTop         int
	plSelected    map[int]bool
	plInputMode   bool
	plInput       inputField
	plInputErr    string
	mediaDuration int
	fragment      *core.DownloadFragment
	fragmentErr   string
	fragmentIn    inputField

	menu menu

	mode           core.DownloadMode
	profile        core.OutputProfile
	qualityChoices []core.QualityChoice
	videoProfiles  []core.OutputProfile
	audioProfiles  []core.OutputProfile
	subTracks      []core.SubtitleTrack
	subsOffered    bool
	audioTracks    []core.AudioTrack
	audioOffered   bool
	audioList      checklist[core.AudioTrack]
	subList        checklist[core.SubtitleTrack]
	flowErr        string
	url            string
	dlEntries      []core.PlaylistEntry
	forceSingle    bool
	numWorkers     int
	dlCh           <-chan core.DlUpdate
	slots          []slotState
	dlDone         int
	dlFailed       int
	dlTotal        int
	singleOK       bool
	downloadErr    string
	dlStartedAt    time.Time
	dlElapsed      time.Duration
	timerActive    bool

	session         core.Session
	depReturnScreen screen
	depRefreshing   bool
	depRefreshToken int
	depUpdateDone   bool

	baseCtx  context.Context
	opCancel context.CancelFunc
	opGen    int

	// pickGen/pickCancel isolate the folder-picker modal from opGen so
	// opening the picker never cancels in-flight network requests.
	pickCancel context.CancelFunc
	pickGen    int

	dlCancel    context.CancelFunc
	depCancel   context.CancelFunc
	dlCancelled bool
	dlGen       int
}

func New(env *adapters.Env, ctx context.Context) tea.Model {
	if ctx == nil {
		ctx = context.Background()
	}
	return newModelWithAPI(ctx, newAppAPI(env))
}

func newModelWithAPI(ctx context.Context, api AppAPI) Model {
	loc := api.LoadLocale()

	m := Model{
		api:         api,
		baseCtx:     ctx,
		screen:      scrUpdateCheck,
		locale:      loc,
		urlInput:    newInput(inputURL, "https://youtu.be/...", inputW, 300),
		searchInput: newInput(inputSearch, i18n.StringsFor(loc).SearchPlaceholder, inputW, 120),
		plInput:     newInput(inputPlaylist, i18n.StringsFor(loc).PlInputPlaceholder, 38, 100),
		fragmentIn:  newInput(inputFragment, "1:00-2:30", 28, 32),
		mode:        core.ModeVideo,
		profile:     i18n.DefaultVideoProfile(loc),
		numWorkers:  1,
		plSelected:  map[int]bool{},
		audioList:   newChecklist(func(track core.AudioTrack) string { return track.Lang }),
		subList:     newChecklist(func(track core.SubtitleTrack) string { return track.Lang }),
	}
	m.syncLayout()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerTickCmd(),
		checkUpdateCmd(m.api),
	)
}

func (m Model) u() *i18n.UIStrings {
	return i18n.StringsFor(m.locale)
}

func (m Model) cancelOps() Model {
	return m.cancelOp(true)
}

func (m Model) cancelOp(bump bool) Model {
	if bump {
		m.opGen++
	}
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
	return m.cancelOp(false)
}
