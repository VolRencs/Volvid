package bot

import (
	"sync"

	app "YouTubeBuild/internal/app"
)

type UserState int

const (
	StateIdle UserState = iota
	StateAwaitingPlaylistOp
	StateFetchingPlaylist
	StateAwaitingPlaylistScope
	StateAwaitingPlaylistSelection
	StateAwaitingMode
	StateAwaitingAudioProfile
	StateFetchingQuality
	StateAwaitingQuality
	StateDownloading
)

type Session struct {
	mu              sync.RWMutex
	State           UserState
	URL             string
	Target          app.ParsedTarget
	WorkDir         string
	PlInfo          *app.PlaylistInfo
	SelectedEntries []app.PlaylistEntry
	SelectedIndices map[int]bool
	PlaylistPage    int
	QualityChoices  []app.QualityChoice
	Mode            app.DownloadMode
	Profile         app.OutputProfile
	ForceSingle     bool
	StatusMsgID     int
	stopCh          chan struct{}
	stopOnce        sync.Once
}

type SessionSnapshot struct {
	State           UserState
	URL             string
	Target          app.ParsedTarget
	WorkDir         string
	PlInfo          *app.PlaylistInfo
	SelectedEntries []app.PlaylistEntry
	SelectedIndices map[int]bool
	PlaylistPage    int
	QualityChoices  []app.QualityChoice
	Mode            app.DownloadMode
	Profile         app.OutputProfile
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

func (s *Session) snapshot() SessionSnapshot {
	if s == nil {
		return SessionSnapshot{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionSnapshot{
		State:           s.State,
		URL:             s.URL,
		Target:          s.Target,
		WorkDir:         s.WorkDir,
		PlInfo:          clonePlaylistInfo(s.PlInfo),
		SelectedEntries: append([]app.PlaylistEntry(nil), s.SelectedEntries...),
		SelectedIndices: cloneSelection(s.SelectedIndices),
		PlaylistPage:    s.PlaylistPage,
		QualityChoices:  append([]app.QualityChoice(nil), s.QualityChoices...),
		Mode:            s.Mode,
		Profile:         s.Profile,
		ForceSingle:     s.ForceSingle,
		StatusMsgID:     s.StatusMsgID,
	}
}

func (s *Session) mutate(fn func(*Session)) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

func newSession(url, workDir string) *Session {
	target, _ := app.ParseTarget(url)
	return &Session{
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

func clonePlaylistInfo(info *app.PlaylistInfo) *app.PlaylistInfo {
	if info == nil {
		return nil
	}
	cloned := &app.PlaylistInfo{
		Title:   info.Title,
		Entries: append([]app.PlaylistEntry(nil), info.Entries...),
	}
	return cloned
}

func cloneSelection(src map[int]bool) map[int]bool {
	if len(src) == 0 {
		return nil
	}
	out := make(map[int]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

type SessionStore struct {
	mu   sync.RWMutex
	data map[int64]*Session
}

func newSessionStore() *SessionStore {
	return &SessionStore{data: make(map[int64]*Session)}
}

func (s *SessionStore) get(id int64) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

func (s *SessionStore) set(id int64, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = sess
}

func (s *SessionStore) reset(id int64) *Session {
	sess := idleSession()
	s.set(id, sess)
	return sess
}

func (s *SessionStore) getOrNew(id int64) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.data[id]; ok {
		return sess
	}
	sess := &Session{}
	s.data[id] = sess
	return sess
}

func (s *SessionStore) hasBusy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sess := range s.data {
		if sess == nil {
			continue
		}
		if sess.snapshot().State != StateIdle {
			return true
		}
	}
	return false
}
