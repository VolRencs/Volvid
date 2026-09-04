package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadFileContext(
	env *Env,
	ctx context.Context,
	url, dest string,
	l Locale,
	ch chan<- FileProgress,
) error {
	ctx = resolveContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, defaultFileDownloadTimeout)
	defer cancel()
	return downloadFileWith(ctx, env.dlClient, url, dest, l, ch)
}

func downloadFileWith(
	ctx context.Context,
	client *http.Client,
	url, dest string,
	l Locale,
	ch chan<- FileProgress,
) error {
	if err := ensureDownloadDir(dest); err != nil {
		return err
	}

	req, err := newDownloadRequest(ctx, url)
	if err != nil {
		return err
	}

	resp, err := doSafeRequest(ctx, client, req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if err := validateDownloadResponse(resp, url); err != nil {
		return err
	}

	tmp, file, err := createTempDownloadFile(dest)
	if err != nil {
		return err
	}

	writer := &dlWriter{
		w:        file,
		total:    max(resp.ContentLength, 0),
		ch:       ch,
		locale:   l,
		lastTime: time.Now(),
		nextEmit: time.Now(),
	}

	if err := copyDownloadBody(ctx, file, writer, resp.Body, tmp); err != nil {
		return err
	}
	if err := replaceDownloadedFile(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func ensureDownloadDir(dest string) error {
	if _, err := prepareDir(filepath.Dir(dest)); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return nil
}

func newDownloadRequest(ctx context.Context, url string) (*http.Request, error) {
	if err := validateDownloadURL(url); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "Volvid/"+Version)
	return req, nil
}

func validateDownloadURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if u.User != nil {
		return fmt.Errorf("download URL must not contain credentials")
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return fmt.Errorf("download URL host is empty")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if isLocalHTTPHost(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("insecure download URL scheme: %s", u.Scheme)
	default:
		return fmt.Errorf("unsupported download URL scheme: %s", u.Scheme)
	}
}

func isLocalHTTPHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateDownloadResponse(resp *http.Response, url string) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s while downloading %s", resp.Status, url)
	}
	return nil
}

func createTempDownloadFile(dest string) (string, *os.File, error) {
	dir := filepath.Dir(dest)
	pattern := sanitizeTempPattern(filepath.Base(dest)) + ".*.part"
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create temp file for %s: %w", dest, err)
	}
	if err := file.Chmod(0o644); err != nil {
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
		return "", nil, fmt.Errorf("chmod temp file %s: %w", name, err)
	}
	return file.Name(), file, nil
}

func sanitizeTempPattern(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "*", "_"))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "download"
	}
	return name
}

func copyDownloadBody(ctx context.Context, file *os.File, writer *dlWriter, body io.Reader, tmp string) error {
	_, copyErr := io.CopyBuffer(writer, body, make([]byte, downloadCopyBufferSize))
	if copyErr != nil {
		closeErr := file.Close()
		writer.emit(true, copyErr)
		_ = os.Remove(tmp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		return copyErr
	}

	syncErr := file.Sync()
	closeErr := file.Close()
	switch {
	case syncErr != nil:
		writer.emit(true, syncErr)
		_ = os.Remove(tmp)
		return syncErr
	case closeErr != nil:
		writer.emit(true, closeErr)
		_ = os.Remove(tmp)
		return closeErr
	default:
		writer.emit(true, nil)
		return nil
	}
}

func replaceDownloadedFile(tmp, dest string) error {
	if err := os.Rename(tmp, dest); err == nil {
		return nil
	}
	if err := replaceFilesWithBackup(map[string]string{tmp: dest}); err != nil {
		return fmt.Errorf("replace file %s: %w", dest, err)
	}
	return nil
}

func replaceFilesWithBackup(paths map[string]string) error {
	type backupEntry struct{ dest, backup string }
	var backups []backupEntry
	rollback := func() error {
		var errs []error
		for _, b := range backups {
			if err := os.Rename(b.backup, b.dest); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	for src, dest := range paths {
		backup, err := replacementBackupPath(dest)
		if err != nil {
			return errors.Join(err, rollback())
		}
		hadDest := true
		if err := os.Rename(dest, backup); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return errors.Join(fmt.Errorf("%s: %w", filepath.Base(dest), err), rollback())
			}
			hadDest = false
		}
		if err := os.Rename(src, dest); err != nil {
			if hadDest {
				_ = os.Rename(backup, dest)
			}
			return errors.Join(fmt.Errorf("%s: %w", filepath.Base(dest), err), rollback())
		}
		if hadDest {
			backups = append(backups, backupEntry{dest: dest, backup: backup})
		}
	}

	for _, b := range backups {
		_ = os.Remove(b.backup)
	}
	return nil
}

func replacementBackupPath(dest string) (string, error) {
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	for range 8 {
		var rnd [8]byte
		if _, err := rand.Read(rnd[:]); err != nil {
			return "", fmt.Errorf("create backup for %s: %w", dest, err)
		}
		name := filepath.Join(dir, "."+base+".bak-"+hex.EncodeToString(rnd[:]))
		if _, err := os.Lstat(name); err != nil {
			if os.IsNotExist(err) {
				return name, nil
			}
			return "", fmt.Errorf("create backup for %s: %w", dest, err)
		}
	}
	return "", fmt.Errorf("create backup for %s: too many collisions", dest)
}
