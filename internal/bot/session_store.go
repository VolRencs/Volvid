package bot

import "sync"

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
