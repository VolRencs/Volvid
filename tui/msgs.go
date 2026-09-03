package tui

import (
	"time"

	app "volvid/internal/app"
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
	msgDepsRefreshed struct {
		deps  app.CheckDepsResult
		token int
	}
	msgPlaylistFetched struct {
		info *app.PlaylistInfo
		err  error
		gen  int
	}
	msgSearchResults struct {
		results []app.SearchResult
		err     error
		gen     int
	}
	msgQualityScanned struct {
		choices []app.QualityChoice
		err     error
		gen     int
	}
	msgFragmentDuration struct {
		duration int
		err      error
		gen      int
	}
	msgDlUpdate struct {
		update app.DlUpdate
		gen    int
	}
	msgOpenDownloadsDirDone struct{ err error }
	msgPickDownloadsDirDone struct {
		path string
		err  error
	}

	spinnerTickMsg struct{}
	timerTickMsg   time.Time
	cursorBlinkMsg struct {
		target inputTarget
		tag    int
	}
	menuDigitTickMsg struct{}
)
