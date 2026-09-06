package adapters

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
	"volvid/internal/core"
	"volvid/internal/i18n"
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

func newTimeoutHTTPClient(timeout time.Duration) *http.Client {
	return newHTTPClient(defaultHTTPClientConfig(timeout))
}
func defaultHTTPClientConfig(timeout time.Duration) HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:               timeout,
		DialTimeout:           defaultDialTimeout,
		KeepAlive:             defaultKeepAlive,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
	}
}
func newDownloadHTTPClient() *http.Client {
	return newHTTPClient(downloadHTTPClientConfig())
}
func downloadHTTPClientConfig() HTTPClientConfig {
	cfg := defaultHTTPClientConfig(0)
	cfg.MaxIdleConns = 32
	cfg.MaxIdleConnsPerHost = 16
	return cfg
}
func newHTTPClient(cfg HTTPClientConfig) *http.Client {
	cfg = normalizeHTTPClientConfig(cfg)
	return &http.Client{
		Timeout:       cfg.Timeout,
		Transport:     buildHTTPTransport(cfg),
		CheckRedirect: safeRedirectPolicy,
	}
}
func safeRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxHTTPRedirects {
		return errors.New("stopped after 10 redirects")
	}
	if req == nil || req.URL == nil {
		return errors.New("redirect URL is empty")
	}
	return validateDownloadURL(req.URL.String())
}
func normalizeHTTPClientConfig(cfg HTTPClientConfig) HTTPClientConfig {
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
		cfg.MaxIdleConnsPerHost = 32
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
func resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func doSafeRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	ctx = resolveContext(ctx)

	var lastErr error
	for attempt := range defaultSafeRetryAttempts {
		resp, err := client.Do(req.Clone(ctx))
		if err == nil {
			if shouldRetryStatus(resp.StatusCode) && attempt+1 < defaultSafeRetryAttempts {
				_, _ = io.CopyN(io.Discard, resp.Body, retryBodyDrainLimit)
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
		return netErr.Timeout()
	}
	// Не ретраим TLS/redirect-policy/url.Error без таймаута:
	// повтор не поможет, только маскирует ошибку.
	return false
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

func downloadFileContext(
	env *Env,
	ctx context.Context,
	url, dest string,
	l core.Locale,
	ch chan<- core.FileProgress,
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
	l core.Locale,
	ch chan<- core.FileProgress,
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
func tempFilePattern(name string) string {
	return core.SanitizeFileStem(name, "download")
}

func createTempDownloadFile(dest string) (string, *os.File, error) {
	dir := filepath.Dir(dest)
	pattern := tempFilePattern(filepath.Base(dest)) + ".*.part"
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

type dlWriter struct {
	w        io.Writer
	total    int64
	done     int64
	lastDone int64
	lastTime time.Time
	nextEmit time.Time
	locale   core.Locale
	ch       chan<- core.FileProgress
}

func (w *dlWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.done += int64(n)
	if w.ch != nil && time.Now().After(w.nextEmit) {
		w.emit(false, nil)
		w.nextEmit = time.Now().Add(progressEmitInterval)
	}
	return n, err
}
func (w *dlWriter) emit(fin bool, e error) {
	if w.ch == nil {
		return
	}
	now := time.Now()
	var speed string
	if elapsed := now.Sub(w.lastTime).Seconds(); elapsed > 0 {
		speed = i18n.FormatSpeed(int64(float64(w.done-w.lastDone)/elapsed), w.locale)
		w.lastDone, w.lastTime = w.done, now
	}
	pct := 0.0
	if w.total > 0 {
		pct = float64(w.done) / float64(w.total) * 100
	}
	select {
	case w.ch <- core.FileProgress{
		Pct:    pct,
		DoneB:  w.done,
		TotalB: w.total,
		Speed:  speed,
		Done:   fin,
		Err:    e,
	}:
	default:
	}
}
