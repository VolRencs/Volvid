package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"volvid/internal/core"
)

type probePayload struct {
	Duration json.RawMessage    `json:"duration"`
	Formats  []core.MediaFormat `json:"formats"`
}

var ErrMediaDurationUnavailable = errors.New("media duration unavailable")

func ProbeMediaDurationContext(env *Env, ctx context.Context, target core.ParsedTarget) (int, error) {
	probe, err := probeMediaWithDeps(env, ctx, resolveRuntimeDeps(env), target)
	if err != nil {
		return 0, err
	}
	if probe == nil || probe.Duration <= 0 {
		return 0, ErrMediaDurationUnavailable
	}
	return probe.Duration, nil
}

func probeMediaWithDeps(env *Env, ctx context.Context, deps core.CheckDepsResult, target core.ParsedTarget) (*core.MediaProbe, error) {
	if !target.IsVideo() {
		return nil, errors.New("probe requires video target")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := probeCacheKey(target)

	probe, err := env.probeCache.ProbeLoad(key, ctx, func() (*core.MediaProbe, error) {
		return probeMediaUncached(env, ctx, deps, target)
	})
	if err != nil {
		return nil, err
	}
	return cloneMediaProbe(probe), nil
}
func probeMediaUncached(env *Env, ctx context.Context, deps core.CheckDepsResult, target core.ParsedTarget) (*core.MediaProbe, error) {
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

	probe := &core.MediaProbe{
		Duration: decodeProbeDuration(payload.Duration),
		Formats:  append([]core.MediaFormat(nil), payload.Formats...),
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
func probeCacheKey(target core.ParsedTarget) string {
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
func cloneMediaProbe(probe *core.MediaProbe) *core.MediaProbe {
	if probe == nil {
		return nil
	}
	cloned := &core.MediaProbe{
		Duration: probe.Duration,
		HasVideo: probe.HasVideo,
		Formats:  append([]core.MediaFormat(nil), probe.Formats...),
	}
	return cloned
}
