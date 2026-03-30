package bot

import (
	"sync"

	app "YouTubeBuild/internal/app"
)

type UserState int

const (
	StateIdle UserState = iota
	StateAwaitingSearchSelection
	StateAwaitingPlaylistOp
	StateFetchingPlaylist
	StateAwaitingPlaylistSelection
	StateFetchingFragmentMetadata
	StateAwaitingFragmentChoice
	StateAwaitingFragmentInput
	StateAwaitingMode
	StateAwaitingAudioProfile
	StateFetchingQuality
	StateAwaitingQuality
	StateDownloading
)

type Session struct {
	mu              sync.RWMutex
	UserID          int64
	State           UserState
	URL             string
	Target          app.ParsedTarget
	WorkDir         string
	SearchQuery     string
	SearchResults   []app.SearchResult
	PlInfo          *app.PlaylistInfo
	SelectedIndices map[int]bool
	PlaylistPage    int
	QualityChoices  []app.QualityChoice
	Mode            app.DownloadMode
	Profile         app.OutputProfile
	MediaDuration   int
	Fragment        *app.DownloadFragment
	ForceSingle     bool
	StatusMsgID     int
	stopCh          chan struct{}
	stopOnce        sync.Once
}

type SessionSnapshot struct {
	UserID          int64
	State           UserState
	URL             string
	Target          app.ParsedTarget
	WorkDir         string
	SearchQuery     string
	SearchResults   []app.SearchResult
	PlInfo          *app.PlaylistInfo
	SelectedIndices map[int]bool
	PlaylistPage    int
	QualityChoices  []app.QualityChoice
	Mode            app.DownloadMode
	Profile         app.OutputProfile
	MediaDuration   int
	Fragment        *app.DownloadFragment
	ForceSingle     bool
	StatusMsgID     int
}

func (s *Session) cancel() {
	if s.stopCh != nil {
		s.stopOnce.Do(func() { close(s.stopCh) })
	}
}

func (s *Session) isCancelled() bool {
	if s.stopCh == nil {
		return false
	}
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

func (s *Session) stopSignal() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopCh
}

func (s *Session) mutate(fn func(*Session)) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

func (s *Session) transition(state UserState, fn func(*Session)) {
	s.mutate(func(sess *Session) {
		sess.State = state
		if fn != nil {
			fn(sess)
		}
	})
}

func (s *Session) setStatusMessage(msgID int) {
	s.mutate(func(sess *Session) {
		sess.StatusMsgID = msgID
	})
}

func (s *Session) setTarget(target app.ParsedTarget) {
	s.mutate(func(sess *Session) {
		sess.Target = target
	})
}

func (s *Session) beginSearch(query string) {
	s.mutate(func(sess *Session) {
		sess.SearchQuery = query
	})
}

func (s *Session) storeSearchResults(query string, results []app.SearchResult) {
	s.transition(StateAwaitingSearchSelection, func(sess *Session) {
		sess.SearchQuery = query
		sess.SearchResults = results
	})
}

func (s *Session) chooseSingleVideo(msgID int) {
	s.mutate(func(sess *Session) {
		sess.ForceSingle = true
		sess.PlInfo = nil
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.MediaDuration = 0
		sess.Fragment = nil
		sess.StatusMsgID = msgID
	})
}

func (s *Session) startPlaylistChoice() {
	s.transition(StateAwaitingPlaylistOp, nil)
}

func (s *Session) beginPlaylistFetch() {
	s.transition(StateFetchingPlaylist, func(sess *Session) {
		sess.ForceSingle = false
		sess.PlInfo = nil
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.QualityChoices = nil
		sess.Profile = app.OutputProfile{}
		sess.MediaDuration = 0
		sess.Fragment = nil
	})
}

func (s *Session) storePlaylist(info *app.PlaylistInfo) {
	s.mutate(func(sess *Session) {
		sess.PlInfo = info
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.QualityChoices = nil
		sess.Profile = app.OutputProfile{}
		sess.MediaDuration = 0
		sess.Fragment = nil
	})
}

func (s *Session) beginPlaylistSelection(msgID int) {
	s.transition(StateAwaitingPlaylistSelection, func(sess *Session) {
		sess.StatusMsgID = msgID
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.QualityChoices = nil
		sess.Profile = app.OutputProfile{}
	})
}

func (s *Session) setPlaylistSelection(selected map[int]bool) {
	s.mutate(func(sess *Session) {
		sess.SelectedIndices = cloneSelection(selected)
	})
}

func (s *Session) setPlaylistPage(page int) {
	s.mutate(func(sess *Session) {
		sess.PlaylistPage = page
	})
}

func (s *Session) beginFragmentProbe() {
	s.transition(StateFetchingFragmentMetadata, func(sess *Session) {
		sess.MediaDuration = 0
		sess.Fragment = nil
	})
}

func (s *Session) setMediaDuration(duration int) {
	s.mutate(func(sess *Session) {
		sess.MediaDuration = duration
	})
}

func (s *Session) setFragment(fragment *app.DownloadFragment) {
	s.mutate(func(sess *Session) {
		sess.Fragment = cloneDownloadFragment(fragment)
	})
}

func (s *Session) beginFragmentChoice() {
	s.transition(StateAwaitingFragmentChoice, func(sess *Session) {
		sess.Fragment = nil
	})
}

func (s *Session) beginFragmentInput(msgID int) {
	s.transition(StateAwaitingFragmentInput, func(sess *Session) {
		sess.StatusMsgID = msgID
	})
}

func (s *Session) beginModeSelection() {
	s.transition(StateAwaitingMode, func(sess *Session) {
		sess.Mode = app.DefaultDownloadMode()
		sess.Profile = app.DefaultVideoProfile(app.LocaleRU)
		sess.QualityChoices = nil
		if sess.PlInfo != nil && !sess.ForceSingle {
			sess.Fragment = nil
		}
	})
}

func (s *Session) chooseMode(mode app.DownloadMode, msgID int) {
	s.mutate(func(sess *Session) {
		sess.Mode = mode
		sess.Profile = app.OutputProfile{}
		sess.QualityChoices = nil
		sess.StatusMsgID = msgID
	})
}

func (s *Session) beginAudioSelection() {
	s.transition(StateAwaitingAudioProfile, func(sess *Session) {
		sess.Profile = app.OutputProfile{}
	})
}

func (s *Session) beginQualityScan() {
	s.transition(StateFetchingQuality, func(sess *Session) {
		sess.QualityChoices = nil
	})
}

func (s *Session) setQualityChoices(choices []app.QualityChoice) {
	s.transition(StateAwaitingQuality, func(sess *Session) {
		sess.QualityChoices = choices
	})
}

func (s *Session) setProfile(profile app.OutputProfile, msgID int) {
	s.mutate(func(sess *Session) {
		sess.Profile = profile
		sess.StatusMsgID = msgID
	})
}

func (s *Session) beginDownload() {
	s.transition(StateDownloading, nil)
}

func newSession(userID int64, url, workDir string) *Session {
	target, _ := app.ParseTarget(url)
	return &Session{
		UserID:  userID,
		State:   StateIdle,
		URL:     url,
		Target:  target,
		WorkDir: workDir,
		Mode:    app.DefaultDownloadMode(),
		Profile: app.DefaultVideoProfile(app.LocaleRU),
		stopCh:  make(chan struct{}),
	}
}

func idleSession() *Session {
	return &Session{State: StateIdle}
}
