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
	cfg = normalizedHTTPClientConfig(cfg)
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: buildHTTPTransport(cfg),
	}
}

func normalizedHTTPClientConfig(cfg HTTPClientConfig) HTTPClientConfig {
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultDialTimeout
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = defaultKeepAlive
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = defaultIdleConnTimeout
	}
	if cfg.ResponseHeaderTimeout <= 0 {
		cfg.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	if cfg.TLSHandshakeTimeout <= 0 {
		cfg.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if cfg.ExpectContinueTimeout <= 0 {
		cfg.ExpectContinueTimeout = defaultExpectContinueTimeout
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 64
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 16
	}
	return cfg
}

func buildHTTPTransport(cfg HTTPClientConfig) *http.Transport {
	transport := cloneDefaultTransport()
	transport.DialContext = (&net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: cfg.KeepAlive,
	}).DialContext
	transport.ForceAttemptHTTP2 = true
	transport.IdleConnTimeout = cfg.IdleConnTimeout
	transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = cfg.TLSHandshakeTimeout
	transport.ExpectContinueTimeout = cfg.ExpectContinueTimeout
	transport.MaxIdleConns = cfg.MaxIdleConns
	transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	return transport
}

func cloneDefaultTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	return base.Clone()
}

func doSafeRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	client = effectiveHTTPClient(client)
	ctx = effectiveContext(ctx)

	var lastErr error
	for attempt := 0; attempt < defaultSafeRetryAttempts; attempt++ {
		resp, err := client.Do(req.Clone(ctx))
		if err == nil {
			if shouldRetryStatus(resp.StatusCode) && attempt+1 < defaultSafeRetryAttempts {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if err := sleepWithContext(ctx, retryBackoffForAttempt(attempt)); err != nil {
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
		if err := sleepWithContext(ctx, retryBackoffForAttempt(attempt)); err != nil {
			return nil, err
		}
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, lastErr
}

func retryBackoffForAttempt(attempt int) time.Duration {
	return defaultSafeRetryBackoff * time.Duration(attempt+1)
}

func effectiveHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return NewHTTPClient(0)
}

func effectiveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
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
	ctx = effectiveContext(ctx)
	client = effectiveDownloadClient(client)

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

func effectiveDownloadClient(client *http.Client) *http.Client {
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
