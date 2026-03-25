package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	updaterDialTimeout           = 30 * time.Second
	updaterKeepAlive             = 30 * time.Second
	updaterIdleConnTimeout       = 90 * time.Second
	updaterTLSHandshakeTimeout   = 10 * time.Second
	updaterExpectContinueTimeout = time.Second
	updaterResponseHeaderTimeout = 60 * time.Second
	updaterDownloadTimeout       = 2 * time.Hour
	updaterReplaceTimeout        = 45 * time.Second
	updaterRetryStep             = 300 * time.Millisecond
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	waitPID := flag.Int("wait-pid", 0, "PID of the running app process")
	downloadURL := flag.String("download-url", "", "release asset URL")
	target := flag.String("target", "", "path to the executable to replace")
	restart := flag.String("restart", "", "path to the executable to restart")
	timeoutSeconds := flag.Int("timeout", 90, "seconds to wait for the app process to exit")
	flag.Parse()

	if *downloadURL == "" {
		return errors.New("download URL is required")
	}
	if *target == "" {
		return errors.New("target path is required")
	}

	targetPath, err := filepath.Abs(*target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	restartPath := targetPath
	if *restart != "" {
		restartPath, err = filepath.Abs(*restart)
		if err != nil {
			return fmt.Errorf("resolve restart path: %w", err)
		}
	}

	tmp := targetPath + ".download"
	ctx, cancel := context.WithTimeout(context.Background(), updaterDownloadTimeout)
	defer cancel()

	if err := downloadFile(ctx, *downloadURL, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	_ = waitForProcess(*waitPID, time.Duration(max(*timeoutSeconds, 1))*time.Second)

	if err := replaceWithRetry(tmp, targetPath, updaterReplaceTimeout); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := startDetached(restartPath); err != nil {
		return fmt.Errorf("restart app: %w", err)
	}
	return nil
}

func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   updaterDialTimeout,
		KeepAlive: updaterKeepAlive,
	}).DialContext
	transport.ForceAttemptHTTP2 = true
	transport.IdleConnTimeout = updaterIdleConnTimeout
	transport.ResponseHeaderTimeout = updaterResponseHeaderTimeout
	transport.TLSHandshakeTimeout = updaterTLSHandshakeTimeout
	transport.ExpectContinueTimeout = updaterExpectContinueTimeout
	transport.MaxIdleConns = 8
	transport.MaxIdleConnsPerHost = 4

	return &http.Client{
		Timeout:   updaterDownloadTimeout,
		Transport: transport,
	}
}

func downloadFile(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "VolRenUpdater")

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: HTTP %s", resp.Status)
	}

	_ = os.Remove(dest)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	_, copyErr := io.CopyBuffer(f, resp.Body, make([]byte, 256<<10))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("write temp file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close temp file: %w", closeErr)
	}
	return nil
}

func replaceWithRetry(src, dest string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	backup := dest + ".old"
	for {
		_ = os.Remove(backup)

		renamedOld := false
		if _, err := os.Stat(dest); err == nil {
			if err := os.Rename(dest, backup); err != nil {
				lastErr = err
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(updaterRetryStep)
				continue
			}
			renamedOld = true
		} else if !errors.Is(err, os.ErrNotExist) {
			lastErr = err
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(updaterRetryStep)
			continue
		}

		if err := os.Rename(src, dest); err == nil {
			_ = os.Remove(backup)
			return nil
		} else {
			lastErr = err
			if renamedOld {
				_ = os.Rename(backup, dest)
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(updaterRetryStep)
	}
	return fmt.Errorf("replace executable: %w", lastErr)
}
