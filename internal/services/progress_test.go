package services

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"volvid/internal/core"
)

func collect(t *testing.T, ch <-chan core.FileProgress) []core.FileProgress {
	t.Helper()
	var out []core.FileProgress
	for p := range ch {
		out = append(out, p)
	}
	if len(out) == 0 {
		t.Fatal("expected progress messages")
	}
	return out
}

func TestLaunchProgressForwardsAndTerminates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fn := func(ctx context.Context, ch chan<- core.FileProgress) error {
			ch <- core.FileProgress{Pct: 50}
			return nil
		}
		ch, cancel := LaunchProgress(t.Context(), fn)
		defer cancel()

		got := collect(t, ch)
		last := got[len(got)-1]
		if !last.Done || last.Err != nil {
			t.Fatalf("expected clean terminal, got %+v", last)
		}
	})
}

func TestLaunchProgressPropagatesError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := errors.New("boom")
		fn := func(ctx context.Context, ch chan<- core.FileProgress) error { return want }
		ch, cancel := LaunchProgress(t.Context(), fn)
		defer cancel()

		got := collect(t, ch)
		last := got[len(got)-1]
		if !last.Done || !errors.Is(last.Err, want) {
			t.Fatalf("expected terminal error %v, got %+v", want, last)
		}
	})
}

func TestLaunchProgressRecoversPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fn := func(ctx context.Context, ch chan<- core.FileProgress) error {
			panic("oops")
		}
		ch, cancel := LaunchProgress(t.Context(), fn)
		defer cancel()

		got := collect(t, ch)
		last := got[len(got)-1]
		if !last.Done || last.Err == nil {
			t.Fatalf("expected panic converted to error, got %+v", last)
		}
	})
}

func TestLaunchProgressCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{})
		fn := func(ctx context.Context, ch chan<- core.FileProgress) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}
		ch, cancel := LaunchProgress(t.Context(), fn)
		defer cancel()

		<-started
		cancel()

		got := collect(t, ch)
		last := got[len(got)-1]
		if !last.Done || !errors.Is(last.Err, context.Canceled) {
			t.Fatalf("expected canceled terminal, got %+v", last)
		}
	})
}
