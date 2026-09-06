package services

import (
	"context"
	"fmt"
	"volvid/internal/core"
)

// ProgressBridge moves the concurrent worker from tui/cmds.go:launchProgress
// into the service layer. TUI keeps only thin tea.Cmd wrappers that read a
// single message from the returned channel.
//
// The worker runs fn in a goroutine, forwards progress (with Done cleared)
// and guarantees exactly one terminal message (Done=true) carrying either
// fn's error or ctx cancellation.
func LaunchProgress(
	base context.Context,
	fn func(context.Context, chan<- core.FileProgress) error,
) (<-chan core.FileProgress, context.CancelFunc) {
	ch := make(chan core.FileProgress, 16)
	ctx, cancel := context.WithCancel(base)

	go func() {
		defer close(ch)

		progressCh := make(chan core.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			defer func() {
				if r := recover(); r != nil {
					doneCh <- fmt.Errorf("progress worker panicked: %v", r)
				}
			}()
			doneCh <- fn(ctx, progressCh)
		}()

		cancelled := false
		for progress := range progressCh {
			progress.Done = false
			progress.Err = nil
			if cancelled {
				continue
			}
			select {
			case ch <- progress:
			case <-ctx.Done():
				cancelled = true
			}
		}

		terminal := core.FileProgress{Done: true}
		if fnErr := <-doneCh; fnErr != nil {
			terminal.Err = fnErr
		} else if ctx.Err() != nil {
			terminal.Err = ctx.Err()
		}
		select {
		case ch <- terminal:
		case <-ctx.Done():
			select {
			case ch <- terminal:
			default:
			}
		}
	}()

	return ch, cancel
}
