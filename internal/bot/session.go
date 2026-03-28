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
