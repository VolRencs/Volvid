package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const schedulerSyncInterval = time.Second

type schedulerRunner struct {
	cancel context.CancelFunc
	entry  TimerEntry
}

type Scheduler struct {
	mu      sync.Mutex
	bot     *Bot
	store   *TimerStore
	rootCtx context.Context
	runners map[string]schedulerRunner
}

func newScheduler(store *TimerStore) *Scheduler {
	return &Scheduler{
		store:   store,
		runners: make(map[string]schedulerRunner),
	}
}

func (s *Scheduler) bind(bot *Bot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bot = bot
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.rootCtx = ctx
	s.mu.Unlock()

	if err := s.applyStoreState(s.store.List(), false); err != nil {
		log.Printf("scheduler sync: %v", err)
	}
	go s.syncLoop(ctx)
}

func (s *Scheduler) Add(entry TimerEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("tmr_%d", time.Now().UnixNano())
	}

	entry, _, changed, err := s.normalizeEntry(entry, false)
	if err != nil {
		return err
	}
	if changed {
		if err := s.store.Upsert(entry); err != nil {
			return err
		}
	} else if err := s.store.Upsert(entry); err != nil {
		return err
	}
	return s.startOrUpdateEntry(entry, false)
}

func (s *Scheduler) Remove(id string) error {
	if err := s.store.Delete(id); err != nil {
		return err
	}
	s.stopEntry(id)
	return nil
}

func (s *Scheduler) List() []TimerEntry {
	return s.store.List()
}

func (s *Scheduler) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(schedulerSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.syncOnce(); err != nil {
				log.Printf("scheduler sync: %v", err)
			}
		}
	}
}

func (s *Scheduler) syncOnce() error {
	changed, err := s.store.reloadIfChanged()
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return s.applyStoreState(s.store.List(), true)
}

func (s *Scheduler) applyStoreState(entries []TimerEntry, allowImmediatePast bool) error {
	desired := timerEntryMap(entries)
	var firstErr error

	for _, entry := range desired {
		if err := s.startOrUpdateEntry(entry, allowImmediatePast); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("apply %s: %w", entry.ID, err)
			}
		}
	}
	s.stopMissingEntries(desired)
	return firstErr
}

func (s *Scheduler) startOrUpdateEntry(entry TimerEntry, allowImmediatePast bool) error {
	entry, interval, changed, err := s.normalizeEntry(entry, allowImmediatePast)
	if err != nil {
		return err
	}
	if changed {
		if err := s.store.Upsert(entry); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rootCtx == nil || s.bot == nil {
		return nil
	}

	if runner, ok := s.runners[entry.ID]; ok {
		if timerEntryEqual(runner.entry, entry) {
			return nil
		}
		runner.cancel()
	}

	runCtx, cancel := context.WithCancel(s.rootCtx)
	s.runners[entry.ID] = schedulerRunner{
		cancel: cancel,
		entry:  entry,
	}
	go s.run(runCtx, entry, interval)
	return nil
}

func (s *Scheduler) stopMissingEntries(desired map[string]TimerEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, runner := range s.runners {
		if _, ok := desired[id]; ok {
			continue
		}
		runner.cancel()
		delete(s.runners, id)
	}
}

func (s *Scheduler) stopEntry(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runner, ok := s.runners[id]
	if !ok {
		return
	}
	runner.cancel()
	delete(s.runners, id)
}

func (s *Scheduler) updateRunnerEntry(entry TimerEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runner, ok := s.runners[entry.ID]
	if !ok {
		return
	}
	runner.entry = entry
	s.runners[entry.ID] = runner
}

func (s *Scheduler) normalizeEntry(entry TimerEntry, allowImmediatePast bool) (TimerEntry, time.Duration, bool, error) {
	if strings.TrimSpace(entry.ID) == "" {
		return TimerEntry{}, 0, false, errors.New("timer id is required")
	}

	interval, err := entry.Duration()
	if err != nil {
		return TimerEntry{}, 0, false, err
	}
	if interval <= 0 {
		return TimerEntry{}, 0, false, fmt.Errorf("invalid interval %q", entry.Interval)
	}

	originalNextRun := entry.NextRun
	if entry.NextRun.IsZero() {
		entry.NextRun = time.Now().Add(interval)
	}
	if !allowImmediatePast && entry.NextRun.Before(time.Now()) {
		entry.NextRun = time.Now().Add(interval)
	}

	changed := originalNextRun.IsZero() != entry.NextRun.IsZero() || !originalNextRun.Equal(entry.NextRun)
	return entry, interval, changed, nil
}

func (s *Scheduler) run(ctx context.Context, entry TimerEntry, interval time.Duration) {
	for {
		wait := time.Until(entry.NextRun)
		if wait < 0 {
			wait = 0
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if s.bot != nil {
			if err := s.bot.executeTimer(entry); err != nil {
				log.Printf("timer %s: %v", entry.ID, err)
			}
		}
		select {
		case <-ctx.Done():
			return
		default:
		}

		entry.NextRun = time.Now().Add(interval)
		if err := s.store.Upsert(entry); err != nil {
			s.updateRunnerEntry(entry)
			log.Printf("timer %s save: %v", entry.ID, err)
			continue
		}
		s.updateRunnerEntry(entry)
	}
}

func timerEntryEqual(left, right TimerEntry) bool {
	return left.ID == right.ID &&
		left.Interval == right.Interval &&
		left.NextRun.Equal(right.NextRun) &&
		left.Type == right.Type &&
		left.Text == right.Text &&
		left.Caption == right.Caption &&
		left.SourceChatID == right.SourceChatID &&
		left.SourceMessageID == right.SourceMessageID
}
