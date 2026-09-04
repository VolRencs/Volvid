package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
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

// UnmarshalJSON терпит null в строковых/числовых полях yt-dlp.
func (f *MediaFormat) UnmarshalJSON(data []byte) error {
	var aux struct {
		Height         int `json:"height"`
		VCodec         any `json:"vcodec"`
		ACodec         any `json:"acodec"`
		Filesize       any `json:"filesize"`
		FilesizeApprox any `json:"filesize_approx"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*f = MediaFormat{
		Height:         aux.Height,
		VCodec:         stringOrEmpty(aux.VCodec),
		ACodec:         stringOrEmpty(aux.ACodec),
		Filesize:       intOrZero(aux.Filesize),
		FilesizeApprox: intOrZero(aux.FilesizeApprox),
	}
	return nil
}

func intOrZero(v any) int64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(n)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

func stringOrEmpty(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	default:
		return ""
	}
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
		return probeMediaUncached(env, ctx, deps, target)
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
