package bot

import (
	"context"
	"fmt"
	"log"
	"time"
)

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.rootCtx = ctx
	s.mu.Unlock()

	if err := s.applyStoreState(s.store.List(), false); err != nil {
		logSchedulerError("sync", err)
	}
	go s.syncLoop(ctx)
}

func (s *Scheduler) Add(entry TimerEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("tmr_%d", time.Now().UnixNano())
	}

	entry, _, _, err := normalizeTimerEntry(entry, false)
	if err != nil {
		return err
	}
	if err := s.store.Upsert(entry); err != nil {
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
				logSchedulerError("sync", err)
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
		if err := s.startOrUpdateEntry(entry, allowImmediatePast); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("apply %s: %w", entry.ID, err)
		}
	}
	s.stopMissingEntries(desired)
	return firstErr
}

func logSchedulerError(scope string, err error) {
	if err != nil {
		log.Printf("scheduler %s: %v", scope, err)
	}
}
