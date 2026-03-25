package app

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

func commandOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	defer cancel()

	out, err := exec.CommandContext(runCtx, name, args...).Output()
	if err != nil {
		return nil, normalizeCommandError(runCtx, err)
	}
	return out, nil
}

func commandCombinedOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	defer cancel()

	out, err := exec.CommandContext(runCtx, name, args...).CombinedOutput()
	if err != nil {
		return out, normalizeCommandError(runCtx, err)
	}
	return out, nil
}

func startMergedOutputCommand(
	ctx context.Context,
	timeout time.Duration,
	name string,
	args ...string,
) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	cmd := exec.CommandContext(runCtx, name, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, nil, nil, nil, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		cancel()
		return nil, nil, nil, nil, normalizeCommandError(runCtx, err)
	}
	_ = pw.Close()
	return cmd, pr, runCtx, cancel, nil
}

func waitCommand(cmd *exec.Cmd, ctx context.Context) error {
	if cmd == nil {
		return nil
	}
	if err := cmd.Wait(); err != nil {
		return normalizeCommandError(ctx, err)
	}
	return nil
}

func commandContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func normalizeCommandError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return err
}
