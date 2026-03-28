package bot

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type TimerEntry struct {
	ID              string    `json:"id"`
	Interval        string    `json:"interval"`
	NextRun         time.Time `json:"next_run"`
	Type            string    `json:"type"`
	Text            string    `json:"text,omitempty"`
	Caption         string    `json:"caption,omitempty"`
	SourceChatID    int64     `json:"source_chat_id,omitempty"`
	SourceMessageID int       `json:"source_message_id,omitempty"`
}

func (t TimerEntry) Duration() (time.Duration, error) {
	return time.ParseDuration(t.Interval)
}

type TimerStore struct {
	mu    sync.Mutex
	file  jsonFileState
	items map[string]TimerEntry
}

func newTimerStore(path string) (*TimerStore, error) {
	store := &TimerStore{
		file:  newJSONFileState("timers", path),
		items: make(map[string]TimerEntry),
	}
	if _, err := store.reloadLocked(true); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *TimerStore) Upsert(entry TimerEntry) error {
	if strings.TrimSpace(entry.ID) == "" {
		return errors.New("timer id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		return err
	}
	s.items[entry.ID] = entry
	return s.flushLocked()
}

func (s *TimerStore) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		return err
	}
	delete(s.items, id)
	return s.flushLocked()
}

func (s *TimerStore) Get(id string) (TimerEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		logStoreReloadError(s.file.name, err)
	}
	entry, ok := s.items[id]
	return entry, ok
}

func (s *TimerStore) List() []TimerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.reloadLocked(false); err != nil {
		logStoreReloadError(s.file.name, err)
	}
	return sortedTimerEntries(s.items)
}

func (s *TimerStore) reloadIfChanged() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked(false)
}

func (s *TimerStore) reloadLocked(force bool) (bool, error) {
	return reloadJSONStateLocked(
		&s.file,
		force,
		loadTimerEntryMapFile,
		func(items map[string]TimerEntry) {
			s.items = items
		},
	)
}

func (s *TimerStore) flushLocked() error {
	return flushJSONStateLocked(&s.file, sortedTimerEntries(s.items))
}
