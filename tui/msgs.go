package tui

import (
	"time"
	"volvid/internal/core"
)

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
	msgTracksLoaded struct {
		audioTracks []core.AudioTrack
		audioErr    error
		subTracks   []core.SubtitleTrack
		subErr      error
		gen         int
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
		gen  int
	}

	spinnerTickMsg struct{}
	timerTickMsg   time.Time
	cursorBlinkMsg struct {
		target inputTarget
		tag    int
	}
	menuDigitTickMsg struct{}
)
