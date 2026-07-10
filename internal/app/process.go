package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const processTerminateGrace = 2 * time.Second

func commandOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	defer cancel()

	cmd := newProcessTreeCommand(runCtx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	if err := startCommand(cmd, runCtx); err != nil {
		return nil, err
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

func commandCombinedOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	defer cancel()

	cmd := newProcessTreeCommand(runCtx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := startCommand(cmd, runCtx); err != nil {
		return nil, err
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}

func startMergedOutputCommand(
	ctx context.Context,
	timeout time.Duration,
	name string,
	args ...string,
) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	cmd := newProcessTreeCommand(runCtx, name, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, nil, nil, nil, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := startCommand(cmd, runCtx); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		cancel()
		return nil, nil, nil, nil, err
	}
	_ = pw.Close()
	return cmd, pr, runCtx, cancel, nil
}

func newProcessTreeCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureCommandForProcessTree(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return interruptProcessTree(cmd, processTerminateGrace)
	}
	cmd.WaitDelay = processTerminateGrace
	return cmd
}

func startCommand(cmd *exec.Cmd, ctx context.Context) error {
	if cmd == nil {
		return errors.New("command is not initialized")
	}
	if err := cmd.Start(); err != nil {
		return normalizeCommandError(ctx, err)
	}
	if err := startProcessTree(cmd); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		cleanupProcessTree(cmd)
		return fmt.Errorf("start process tree: %w", err)
	}
	return nil
}

func waitCommand(cmd *exec.Cmd, ctx context.Context) error {
	if cmd == nil {
		return nil
	}
	defer cleanupProcessTree(cmd)
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
