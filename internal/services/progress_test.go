package services

import (
	"context"
	"errors"
	"testing"
	"time"
	"volvid/internal/core"
)

func collect(t *testing.T, ch <-chan core.FileProgress) []core.FileProgress {
	t.Helper()
	var out []core.FileProgress
	timeout := time.After(5 * time.Second)
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, p)
			if p.Done {
				// Drain until close to avoid goroutine leak in test.
				go func() {
					for range ch {
					}
				}()
				return out
			}
		case <-timeout:
			t.Fatal("timeout waiting for progress")
		}
	}
}

func TestLaunchProgressForwardsAndTerminates(t *testing.T) {
	fn := func(ctx context.Context, ch chan<- core.FileProgress) error {
		ch <- core.FileProgress{Pct: 50}
		return nil
	}
	ch, cancel := LaunchProgress(context.Background(), fn)
	defer cancel()

	got := collect(t, ch)
	if len(got) == 0 {
		t.Fatal("expected progress messages")
	}
	last := got[len(got)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("expected clean terminal, got %+v", last)
	}
}

func TestLaunchProgressPropagatesError(t *testing.T) {
	want := errors.New("boom")
	fn := func(ctx context.Context, ch chan<- core.FileProgress) error { return want }
	ch, cancel := LaunchProgress(context.Background(), fn)
	defer cancel()

	got := collect(t, ch)
	last := got[len(got)-1]
	if !last.Done || !errors.Is(last.Err, want) {
		t.Fatalf("expected terminal error %v, got %+v", want, last)
	}
}

func TestLaunchProgressRecoversPanic(t *testing.T) {
	fn := func(ctx context.Context, ch chan<- core.FileProgress) error {
		panic("oops")
	}
	ch, cancel := LaunchProgress(context.Background(), fn)
	defer cancel()

	got := collect(t, ch)
	last := got[len(got)-1]
	if !last.Done || last.Err == nil {
		t.Fatalf("expected panic converted to error, got %+v", last)
	}
}
