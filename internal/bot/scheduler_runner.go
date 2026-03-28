package bot

import (
	"context"
	"log"
	"time"
)

func (s *Scheduler) startOrUpdateEntry(entry TimerEntry, allowImmediatePast bool) error {
	entry, interval, changed, err := normalizeTimerEntry(entry, allowImmediatePast)
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
