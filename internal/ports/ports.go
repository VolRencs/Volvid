// Package ports defines narrow infrastructure seams used by services.
//
// The goal is to stop passing the *adapters.Env god-object into every function
// and to make exec/network/fs/clock replaceable in tests. Concrete
// implementations live in internal/adapters; services and TUI depend
// only on these interfaces.
package ports

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"
)

// Executor runs external processes (yt-dlp, ffmpeg, helpers).
type Executor interface {
	// CommandOutput runs name with args and returns capped stdout.
	CommandOutput(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error)
	// StartMerged starts name with merged stdout+stderr pipe for line streaming.
	StartMerged(ctx context.Context, timeout time.Duration, name string, args ...string) (wait func() error, cancel context.CancelFunc, r io.ReadCloser, err error)
}

// HTTPDoer is the minimal HTTP surface services need (mockable).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// FileSystem covers the file operations scattered across fs.go,
// http_download.go and deps_install.go (atomic writes, dirs, backups).
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(path string, data []byte, perm os.FileMode) error
	AtomicWriteFile(path string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	ReadFileLimit(path string, limit int64) ([]byte, error)
	Rename(oldpath, newpath string) error
	Remove(path string) error
	Stat(path string) (os.FileInfo, error)
	PrepareDir(path string) (string, error)
}

// Clock allows deterministic tests for TTL caches and progress.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// SystemClock is the production Clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// DirPicker abstracts the platform folder picker (D-Bus portal on Linux,
// FolderBrowserDialog on Windows, unsupported elsewhere).
type DirPicker interface {
	PickDirectory(ctx context.Context, current, title string) (string, error)
}
