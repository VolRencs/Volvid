package bot

import (
	"context"
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
