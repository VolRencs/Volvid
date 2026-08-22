package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
)

type MediaProbe struct {
	Duration int
	Formats  []MediaFormat
	HasVideo bool
}

type MediaFormat struct {
	FormatID       string `json:"format_id"`
	Ext            string `json:"ext"`
	Height         int    `json:"height"`
	VCodec         string `json:"vcodec"`
	ACodec         string `json:"acodec"`
	Filesize       int64  `json:"filesize"`
	FilesizeApprox int64  `json:"filesize_approx"`
}

type probePayload struct {
	Duration json.RawMessage `json:"duration"`
	Formats  []MediaFormat   `json:"formats"`
}

type probeCall struct {
	done  chan struct{}
	probe *MediaProbe
	err   error
}

var (
	probeCacheMu sync.RWMutex
	probeCache   = make(map[string]*MediaProbe)
	probeFlight  = make(map[string]*probeCall)
)

var ErrMediaDurationUnavailable = errors.New("media duration unavailable")

func ProbeMediaDuration(target ParsedTarget) (int, error) {
	return ProbeMediaDurationContext(context.Background(), target)
}

func ProbeMediaDurationContext(ctx context.Context, target ParsedTarget) (int, error) {
	probe, err := probeMediaContext(ctx, target)
	if err != nil {
		return 0, err
	}
	if probe == nil || probe.Duration <= 0 {
		return 0, ErrMediaDurationUnavailable
	}
	return probe.Duration, nil
}

func probeMediaContext(ctx context.Context, target ParsedTarget) (*MediaProbe, error) {
	if !target.IsVideo() {
		return nil, errors.New("probe requires video target")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	key := probeCacheKey(target)

	if cached, ok := loadCachedProbe(key); ok {
		return cached, nil
	}

	probeCacheMu.Lock()
	if cached, ok := cloneCachedProbeLocked(key); ok {
		probeCacheMu.Unlock()
		return cached, nil
	}
	if call, ok := probeFlight[key]; ok {
		probeCacheMu.Unlock()
		select {
		case <-call.done:
			if call.err != nil {
				return nil, call.err
			}
			return cloneMediaProbe(call.probe), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &probeCall{done: make(chan struct{})}
	probeFlight[key] = call
	probeCacheMu.Unlock()

	probe, err := probeMediaUncached(ctx, target)
	cached := cloneMediaProbe(probe)

	probeCacheMu.Lock()
	delete(probeFlight, key)
	if err == nil && cached != nil {
		probeCache[key] = cached
	}
	call.probe = cached
	call.err = err
	close(call.done)
	probeCacheMu.Unlock()

	if err != nil {
		return nil, err
	}
	return cloneMediaProbe(cached), nil
}

func probeMediaUncached(ctx context.Context, target ParsedTarget) (*MediaProbe, error) {
	out, err := ytdlpOutput(
		ctx,
		qualityScanTimeout,
		"--dump-single-json",
		"--no-playlist",
		"--no-warnings",
		target.CanonicalURL,
	)
	if err != nil {
		return nil, err
	}

	var payload probePayload
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, err
	}

	probe := &MediaProbe{
		Duration: decodeProbeDuration(payload.Duration),
		Formats:  append([]MediaFormat(nil), payload.Formats...),
	}

	for _, format := range probe.Formats {
		if format.VCodec != "" && format.VCodec != "none" {
			probe.HasVideo = true
		}
	}

	if !probe.HasVideo {
		return nil, fmt.Errorf("probe: no video streams found")
	}
	return probe, nil
}

func probeCacheKey(target ParsedTarget) string {
	key := strings.TrimSpace(target.CanonicalURL)
	if key != "" {
		return key
	}
	return strings.TrimSpace(target.DownloadURL(false))
}

func loadCachedProbe(key string) (*MediaProbe, bool) {
	probeCacheMu.RLock()
	defer probeCacheMu.RUnlock()
	return cloneCachedProbeLocked(key)
}

func cloneCachedProbeLocked(key string) (*MediaProbe, bool) {
	probe, ok := probeCache[key]
	if !ok {
		return nil, false
	}
	return cloneMediaProbe(probe), true
}

func decodeProbeDuration(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var seconds float64
	if err := json.Unmarshal(raw, &seconds); err != nil {
		return 0
	}
	if seconds <= 0 || seconds > math.MaxInt32 {
		return 0
	}
	return int(math.Round(seconds))
}

func cloneMediaProbe(probe *MediaProbe) *MediaProbe {
	if probe == nil {
		return nil
	}
	cloned := &MediaProbe{
		Duration: probe.Duration,
		HasVideo: probe.HasVideo,
		Formats:  append([]MediaFormat(nil), probe.Formats...),
	}
	return cloned
}
