package tui

import (
	"time"
	"volvid/internal/core"
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
	msgUpdateChecked struct{ info *core.UpdateInfo }
	msgDepProgress   struct {
		progress core.FileProgress
		gen      int
	}
	msgDepDone struct {
		err      error
		isUpdate bool
		gen      int
	}
	msgDepsRefreshed struct {
		deps  core.CheckDepsResult
		token int
	}
	msgPlaylistFetched struct {
		info *core.PlaylistInfo
		err  error
		gen  int
	}
	msgSearchResults struct {
		results []core.SearchResult
		err     error
		gen     int
	}
	msgQualityScanned struct {
		choices []core.QualityChoice
		err     error
		gen     int
	}
	msgFragmentDuration struct {
		duration int
		err      error
		gen      int
	}
	msgDlUpdate struct {
		update core.DlUpdate
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
