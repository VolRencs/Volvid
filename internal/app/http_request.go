package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

func doSafeRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = NewHTTPClient(0)
	}
	ctx = resolveContext(ctx)

	var lastErr error
	for attempt := range defaultSafeRetryAttempts {
		resp, err := client.Do(req.Clone(ctx))
		if err == nil {
			if shouldRetryStatus(resp.StatusCode) && attempt+1 < defaultSafeRetryAttempts {
				_, _ = io.CopyN(io.Discard, resp.Body, 1<<20)
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
