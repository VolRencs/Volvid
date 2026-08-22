package app

import (
	"context"
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

func DownloadFileContext(
	ctx context.Context,
	url, dest string,
	l Locale,
	ch chan<- FileProgress,
) error {
	ctx = resolveContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, defaultFileDownloadTimeout)
	defer cancel()
	return downloadFileWith(ctx, dlClient, url, dest, l, ch)
}

func downloadFileWith(
	ctx context.Context,
	client *http.Client,
	url, dest string,
	l Locale,
	ch chan<- FileProgress,
) error {
	client = resolveDownloadHTTPClient(client)
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

func resolveDownloadHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	if dlClient != nil {
		return dlClient
	}
	return newDownloadHTTPClient()
}

func ensureDownloadDir(dest string) error {
	if _, err := prepareDir(filepath.Dir(dest)); err != nil {
		return fmt.Errorf("создание директории: %w", err)
	}
	return nil
}

func newDownloadRequest(ctx context.Context, url string) (*http.Request, error) {
	if err := validateDownloadURL(url); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "VolRenDownloader/"+Version)
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
		return fmt.Errorf("HTTP %s при загрузке %s", resp.Status, url)
	}
	if resp.Body == nil {
		return fmt.Errorf("пустой HTTP body при загрузке %s", url)
	}
	return nil
}

func createTempDownloadFile(dest string) (string, *os.File, error) {
	dir := filepath.Dir(dest)
	pattern := sanitizeTempPattern(filepath.Base(dest)) + ".*.part"
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", nil, fmt.Errorf("создание временного файла для %s: %w", dest, err)
	}
	if err := file.Chmod(0o644); err != nil {
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
		return "", nil, fmt.Errorf("права временного файла %s: %w", name, err)
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
	_, copyErr := io.CopyBuffer(writer, body, make([]byte, 256<<10))
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

	backup := replacementBackupPath(dest)
	hadDest := true
	if err := os.Rename(dest, backup); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("подготовка замены файла %s: %w", dest, err)
		}
		hadDest = false
	}

	if err := os.Rename(tmp, dest); err != nil {
		if hadDest {
			_ = os.Rename(backup, dest)
		}
		return fmt.Errorf("замена файла %s: %w", dest, err)
	}
	if hadDest {
		_ = os.Remove(backup)
	}
	return nil
}

func replacementBackupPath(dest string) string {
	return fmt.Sprintf("%s.bak.%d", dest, time.Now().UnixNano())
}
