package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const windowsUpdaterPrefetchTimeout = 3 * time.Minute

var (
	windowsUpdaterMu   sync.Mutex
	windowsUpdaterOnce sync.Once
)

func WindowsUpdaterPath() string {
	return filepath.Join(DepsDir, updaterName)
}

func EnsureWindowsUpdater(l Locale, ch chan<- FileProgress) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), windowsUpdaterPrefetchTimeout)
	defer cancel()
	return EnsureWindowsUpdaterContext(ctx, l, ch)
}

func EnsureWindowsUpdaterContext(
	ctx context.Context,
	l Locale,
	ch chan<- FileProgress,
) (string, error) {
	if !IsWindows {
		return "", errors.New("windows updater is only supported on Windows")
	}

	windowsUpdaterMu.Lock()
	defer windowsUpdaterMu.Unlock()

	path := WindowsUpdaterPath()
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}

	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return "", fmt.Errorf("создание _deps для updater: %w", err)
	}

	if err := DownloadFileContext(ctx, dlClient, updaterURL, path, l, ch); err != nil {
		return "", err
	}
	return path, nil
}

func PrefetchWindowsUpdater(l Locale) {
	if !IsWindows {
		return
	}

	windowsUpdaterOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), windowsUpdaterPrefetchTimeout)
			defer cancel()
			_, _ = EnsureWindowsUpdaterContext(ctx, l, nil)
		}()
	})
}
