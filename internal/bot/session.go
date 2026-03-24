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
	StateFetchingQuality
	StateAwaitingQuality
	StateDownloading
)

type Session struct {
	State           UserState
	URL             string
	WorkDir         string
	PlInfo          *app.PlaylistInfo
	SelectedEntries []app.PlaylistEntry
	SelectedIndices map[int]bool
	PlaylistPage    int
	QualityChoices  []app.QualityChoice
	ForceSingle     bool
	StatusMsgID     int
	stopCh          chan struct{}
}

func (s *Session) cancel() {
	if s.stopCh != nil {
		select {
		case <-s.stopCh:
		default:
			close(s.stopCh)
		}
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
		if sess.State != StateIdle {
			return true
		}
	}
	return false
}
