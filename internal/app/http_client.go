package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultDialTimeout           = 30 * time.Second
	defaultKeepAlive             = 30 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = time.Second
	defaultResponseHeaderTimeout = 60 * time.Second
	defaultFileDownloadTimeout   = 2 * time.Hour
	defaultSafeRetryAttempts     = 3
	defaultSafeRetryBackoff      = 250 * time.Millisecond
)

type HTTPClientConfig struct {
	Timeout               time.Duration
	DialTimeout           time.Duration
	KeepAlive             time.Duration
	IdleConnTimeout       time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return newHTTPClient(HTTPClientConfig{
		Timeout:               timeout,
		DialTimeout:           defaultDialTimeout,
		KeepAlive:             defaultKeepAlive,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
	})
}

func newDownloadHTTPClient() *http.Client {
	return newHTTPClient(HTTPClientConfig{
		DialTimeout:           defaultDialTimeout,
		KeepAlive:             defaultKeepAlive,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
	})
}

func newHTTPClient(cfg HTTPClientConfig) *http.Client {
	base := http.DefaultTransport
	transport, ok := base.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	} else {
		transport = transport.Clone()
	}

	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultDialTimeout
	}
	keepAlive := cfg.KeepAlive
	if keepAlive <= 0 {
		keepAlive = defaultKeepAlive
	}

	transport.DialContext = (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
	}).DialContext
	transport.ForceAttemptHTTP2 = true

	if cfg.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = cfg.IdleConnTimeout
	}
	if cfg.ResponseHeaderTimeout > 0 {
		transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	}
	if cfg.TLSHandshakeTimeout > 0 {
		transport.TLSHandshakeTimeout = cfg.TLSHandshakeTimeout
	}
	if cfg.ExpectContinueTimeout > 0 {
		transport.ExpectContinueTimeout = cfg.ExpectContinueTimeout
	}
	if cfg.MaxIdleConns > 0 {
		transport.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
}

func doSafeRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = NewHTTPClient(0)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var lastErr error
	for attempt := 0; attempt < defaultSafeRetryAttempts; attempt++ {
		cloned := req.Clone(ctx)
		resp, err := client.Do(cloned)
		if err == nil {
			if shouldRetryStatus(resp.StatusCode) && attempt+1 < defaultSafeRetryAttempts {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if err := sleepWithContext(ctx, defaultSafeRetryBackoff*time.Duration(attempt+1)); err != nil {
					return nil, err
				}
				continue
			}
			return resp, nil
		}

		lastErr = err
		if ctx.Err() != nil || !shouldRetryHTTPError(err) || attempt+1 >= defaultSafeRetryAttempts {
			break
		}
		if err := sleepWithContext(ctx, defaultSafeRetryBackoff*time.Duration(attempt+1)); err != nil {
			return nil, err
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, lastErr
}

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
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = dlClient
	}
	if client == nil {
		client = newDownloadHTTPClient()
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("создание директории: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("создание запроса %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "VolRenDownloader/"+Version)

	resp, err := doSafeRequest(ctx, client, req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s при загрузке %s", resp.Status, url)
	}
	if resp.Body == nil {
		return fmt.Errorf("пустой HTTP body при загрузке %s", url)
	}

	tmp := dest + ".part"
	_ = os.Remove(tmp)

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("создание файла %s: %w", tmp, err)
	}

	pw := &dlWriter{
		w:        f,
		total:    max(resp.ContentLength, 0),
		ch:       ch,
		locale:   l,
		lastTime: time.Now(),
		nextEmit: time.Now(),
	}
	_, copyErr := io.CopyBuffer(pw, resp.Body, make([]byte, 256<<10))
	closeErr := f.Close()

	if copyErr != nil {
		pw.emit(true, copyErr)
		_ = os.Remove(tmp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return copyErr
	}
	if closeErr != nil {
		pw.emit(true, closeErr)
		_ = os.Remove(tmp)
		return closeErr
	}
	pw.emit(true, nil)

	if err := replaceDownloadedFile(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
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

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func shouldRetryHTTPError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return true
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
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
