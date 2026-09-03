package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type limitedBuffer struct {
	buf []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(b.buf) < commandStderrCaptureSize {
		room := commandStderrCaptureSize - len(b.buf)
		if len(p) > room {
			p = p[:room]
		}
		b.buf = append(b.buf, p...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return strings.TrimSpace(string(b.buf))
}

func commandErrorWithStderr(err error, stderr limitedBuffer) error {
	if err == nil {
		return nil
	}
	if text := stderr.String(); text != "" {
		return fmt.Errorf("%w: %s", err, text)
	}
	return err
}

func commandOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return runCommandOutput(ctx, timeout, false, name, args...)
}

func commandCombinedOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return runCommandOutput(ctx, timeout, true, name, args...)
}

func runCommandOutput(ctx context.Context, timeout time.Duration, merge bool, name string, args ...string) ([]byte, error) {
	runCtx, cancel := commandContext(ctx, timeout)
	defer cancel()

	cmd := newProcessTreeCommand(runCtx, name, args...)
	var stdout bytes.Buffer
	var stderr limitedBuffer
	if merge {
		cmd.Stdout = &stdout
		cmd.Stderr = &stdout
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	if err := startCommand(cmd, runCtx); err != nil {
		return nil, err
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		if merge {
			return stdout.Bytes(), err
		}
		return nil, commandErrorWithStderr(err, stderr)
	}
	return stdout.Bytes(), nil
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
	return err
}
