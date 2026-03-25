package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type jsonFileState struct {
	name     string
	path     string
	modTime  time.Time
	fileSize int64
}

func newJSONFileState(name, path string) jsonFileState {
	return jsonFileState{name: name, path: path}
}

func (s *jsonFileState) changedLocked() (bool, error) {
	stat, err := os.Stat(s.path)
	switch {
	case err == nil:
		return !stat.ModTime().Equal(s.modTime) || stat.Size() != s.fileSize, nil
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("stat %s: %w", s.path, err)
	}
}

func (s *jsonFileState) refreshFileStateLocked() error {
	stat, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", s.path, err)
	}
	s.modTime = stat.ModTime()
	s.fileSize = stat.Size()
	return nil
}

func logStoreReloadError(name string, err error) {
	if err != nil {
		log.Printf("%s store reload: %v", name, err)
	}
}

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
	changed := force
	if !force {
		var err error
		changed, err = s.file.changedLocked()
		if err != nil {
			return false, err
		}
		if !changed {
			return false, nil
		}
	}

	ids, err := loadIDSetFile(s.file.path)
	if err != nil {
		_ = s.file.refreshFileStateLocked()
		return changed, err
	}
	s.ids = ids
	if err := s.file.refreshFileStateLocked(); err != nil {
		return true, err
	}
	return true, nil
}

func (s *idSetStore) flushLocked() error {
	if err := writeJSONFile(s.file.path, sortedIDSet(s.ids)); err != nil {
		return err
	}
	return s.file.refreshFileStateLocked()
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
	changed := force
	if !force {
		var err error
		changed, err = s.file.changedLocked()
		if err != nil {
			return false, err
		}
		if !changed {
			return false, nil
		}
	}

	items, err := loadTimerEntryMapFile(s.file.path)
	if err != nil {
		_ = s.file.refreshFileStateLocked()
		return changed, err
	}
	s.items = items
	if err := s.file.refreshFileStateLocked(); err != nil {
		return true, err
	}
	return true, nil
}

func (s *TimerStore) flushLocked() error {
	if err := writeJSONFile(s.file.path, sortedTimerEntries(s.items)); err != nil {
		return err
	}
	return s.file.refreshFileStateLocked()
}

func loadJSONFile(path string, zeroValue any, dest any) error {
	data, err := readOrInitJSONFile(path, zeroValue)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func loadIDSetFile(path string) (map[int64]struct{}, error) {
	var ids []int64
	if err := loadJSONFile(path, []int64{}, &ids); err != nil {
		return nil, err
	}
	return idSliceToSet(ids), nil
}

func loadTimerEntryMapFile(path string) (map[string]TimerEntry, error) {
	var entries []TimerEntry
	if err := loadJSONFile(path, []TimerEntry{}, &entries); err != nil {
		return nil, err
	}
	return timerEntryMap(entries), nil
}

func writeJSONFile(path string, value any) error {
	if err := ensureJSONParentDir(path); err != nil {
		return err
	}
	data, err := marshalJSONFileData(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func readOrInitJSONFile(path string, zeroValue any) ([]byte, error) {
	if err := ensureJSONParentDir(path); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		data, err = marshalJSONFileData(zeroValue)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", path, err)
		}
		if err := writeJSONFile(path, zeroValue); err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) == 0 {
		data, err = marshalJSONFileData(zeroValue)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", path, err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		return data, nil
	}
	return data, nil
}

func ensureJSONParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func marshalJSONFileData(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func sortedIDSet(ids map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func idSliceToSet(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func sortedTimerEntries(items map[string]TimerEntry) []TimerEntry {
	out := make([]TimerEntry, 0, len(items))
	for _, entry := range items {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NextRun.Equal(out[j].NextRun) {
			return out[i].ID < out[j].ID
		}
		return out[i].NextRun.Before(out[j].NextRun)
	})
	return out
}

func timerEntryMap(entries []TimerEntry) map[string]TimerEntry {
	out := make(map[string]TimerEntry, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		out[entry.ID] = entry
	}
	return out
}
