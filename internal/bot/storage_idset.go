package bot

import "sync"

type idSetStore struct {
	mu   sync.Mutex
	file jsonFileState
	ids  map[int64]struct{}
}

func newIDSetStore(name, path string) (*idSetStore, error) {
	store := &idSetStore{
		file: newJSONFileState(name, path),
		ids:  make(map[int64]struct{}),
	}
	if _, err := store.reloadLocked(true); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *idSetStore) Has(id int64) bool {
	if id == 0 {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		logStoreReloadError(s.file.name, err)
	}
	_, ok := s.ids[id]
	return ok
}

func (s *idSetStore) Add(id int64) error {
	if id == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		return err
	}
	if _, ok := s.ids[id]; ok {
		return nil
	}
	s.ids[id] = struct{}{}
	return s.flushLocked()
}

func (s *idSetStore) Remove(id int64) error {
	if id == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		return err
	}
	if _, ok := s.ids[id]; !ok {
		return nil
	}
	delete(s.ids, id)
	return s.flushLocked()
}

func (s *idSetStore) List() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		logStoreReloadError(s.file.name, err)
	}
	return sortedIDSet(s.ids)
}

func (s *idSetStore) reloadIfChanged() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked(false)
}

func (s *idSetStore) reloadLocked(force bool) (bool, error) {
	return reloadJSONStateLocked(
		&s.file,
		force,
		loadIDSetFile,
		func(ids map[int64]struct{}) {
			s.ids = ids
		},
	)
}

func (s *idSetStore) flushLocked() error {
	return flushJSONStateLocked(&s.file, sortedIDSet(s.ids))
}

type PremiumStore struct {
	*idSetStore
}

func newPremiumStore(path string) (*PremiumStore, error) {
	store, err := newIDSetStore("premium", path)
	if err != nil {
		return nil, err
	}
	return &PremiumStore{idSetStore: store}, nil
}

func (s *PremiumStore) HasPremium(id int64) bool {
	if s == nil || s.idSetStore == nil {
		return false
	}
	return s.Has(id)
}

func (s *PremiumStore) AddPremium(id int64) error {
	if s == nil || s.idSetStore == nil {
		return nil
	}
	return s.Add(id)
}

type UserStore struct {
	*idSetStore
}

func newUserStore(path string) (*UserStore, error) {
	store, err := newIDSetStore("users", path)
	if err != nil {
		return nil, err
	}
	return &UserStore{idSetStore: store}, nil
}
