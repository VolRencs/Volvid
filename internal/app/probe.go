package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

type MediaProbe struct {
	Duration int
	Formats  []MediaFormat
	HasVideo bool
}

type MediaFormat struct {
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

var ErrMediaDurationUnavailable = errors.New("media duration unavailable")

func ProbeMediaDurationContext(env *Env, ctx context.Context, target ParsedTarget) (int, error) {
	probe, err := probeMediaContext(env, ctx, target)
	if err != nil {
		return 0, err
	}
	if probe == nil || probe.Duration <= 0 {
		return 0, ErrMediaDurationUnavailable
	}
	return probe.Duration, nil
}

func probeMediaContext(env *Env, ctx context.Context, target ParsedTarget) (*MediaProbe, error) {
	return probeMediaWithDeps(env, ctx, resolveRuntimeDeps(env), target)
}

func probeMediaWithDeps(env *Env, ctx context.Context, deps CheckDepsResult, target ParsedTarget) (*MediaProbe, error) {
	if !target.IsVideo() {
		return nil, errors.New("probe requires video target")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := probeCacheKey(target)

	probe, err := env.probeCache.ProbeLoad(key, ctx, func() (*MediaProbe, error) {
		return probeMediaUncached(env, context.WithoutCancel(ctx), deps, target)
	})
	if err != nil {
		return nil, err
	}
	return cloneMediaProbe(probe), nil
}

func probeMediaUncached(env *Env, ctx context.Context, deps CheckDepsResult, target ParsedTarget) (*MediaProbe, error) {
	out, err := ytdlpOutput(
		env,
		ctx,
		qualityScanTimeout,
		deps,
		"--dump-single-json",
		"--no-playlist",
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
