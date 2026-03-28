package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func DownloadFile(url, dest string, l Locale, ch chan<- FileProgress) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultFileDownloadTimeout)
	defer cancel()
	return DownloadFileContext(ctx, dlClient, url, dest, l, ch)
}

func DownloadFileContext(
	ctx context.Context,
	client *http.Client,
	url, dest string,
	l Locale,
	ch chan<- FileProgress,
) error {
	ctx = resolveContext(ctx)
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
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("создание директории: %w", err)
	}
	return nil
}

func newDownloadRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "VolRenDownloader/"+Version)
	return req, nil
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
	tmp := dest + ".part"
	_ = os.Remove(tmp)

	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", nil, fmt.Errorf("создание файла %s: %w", tmp, err)
	}
	return tmp, file, nil
}

func copyDownloadBody(ctx context.Context, file *os.File, writer *dlWriter, body io.Reader, tmp string) error {
	_, copyErr := io.CopyBuffer(writer, body, make([]byte, 256<<10))
	closeErr := file.Close()

	switch {
	case copyErr != nil:
		writer.emit(true, copyErr)
		_ = os.Remove(tmp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return copyErr
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
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("замена файла %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("замена файла %s: %w", dest, err)
	}
	return nil
}
