package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
)

type MediaProbe struct {
	Title        string
	Duration     int
	ThumbnailURL string
	ThumbnailExt string
	Formats      []MediaFormat
	HasAudio     bool
	HasVideo     bool
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
	Title        string        `json:"title"`
	Duration     int           `json:"duration"`
	Thumbnail    string        `json:"thumbnail"`
	ThumbnailExt string        `json:"thumbnail_ext"`
	Formats      []MediaFormat `json:"formats"`
}

var probeCache sync.Map

var ErrMediaDurationUnavailable = errors.New("media duration unavailable")

func ProbeMediaURL(raw string) (*MediaProbe, error) {
	target, err := ParseTarget(raw)
	if err != nil {
		return nil, err
	}
	return ProbeMediaContext(context.Background(), target)
}

func ProbeMediaDuration(target ParsedTarget) (int, error) {
	return ProbeMediaDurationContext(context.Background(), target)
}

func ProbeMediaDurationContext(ctx context.Context, target ParsedTarget) (int, error) {
	probe, err := ProbeMediaContext(ctx, target)
	if err != nil {
		return 0, err
	}
	if probe == nil || probe.Duration <= 0 {
		return 0, ErrMediaDurationUnavailable
	}
	return probe.Duration, nil
}

func ProbeMediaContext(ctx context.Context, target ParsedTarget) (*MediaProbe, error) {
	if !target.IsVideo() {
		return nil, errors.New("probe requires video target")
	}

	key := target.CanonicalURL
	if cached, ok := probeCache.Load(key); ok {
		return cloneMediaProbe(cached.(*MediaProbe)), nil
	}

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
		Title:        strings.TrimSpace(payload.Title),
		Duration:     payload.Duration,
		ThumbnailURL: strings.TrimSpace(payload.Thumbnail),
		ThumbnailExt: detectThumbnailExt(payload.ThumbnailExt, payload.Thumbnail),
		Formats:      append([]MediaFormat(nil), payload.Formats...),
	}

	for _, format := range probe.Formats {
		if format.ACodec != "" && format.ACodec != "none" {
			probe.HasAudio = true
		}
		if format.VCodec != "" && format.VCodec != "none" {
			probe.HasVideo = true
		}
	}

	if !probe.HasVideo {
		return nil, fmt.Errorf("probe: no video streams found")
	}

	probeCache.Store(key, cloneMediaProbe(probe))
	return probe, nil
}

func detectThumbnailExt(ext, thumbURL string) string {
	ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
	if ext != "" {
		return ext
	}
	if thumbURL == "" {
		return ""
	}
	suffix := strings.TrimPrefix(path.Ext(thumbURL), ".")
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return ""
	}
	return suffix
}

func cloneMediaProbe(probe *MediaProbe) *MediaProbe {
	if probe == nil {
		return nil
	}
	cloned := &MediaProbe{
		Title:        probe.Title,
		Duration:     probe.Duration,
		ThumbnailURL: probe.ThumbnailURL,
		ThumbnailExt: probe.ThumbnailExt,
		HasAudio:     probe.HasAudio,
		HasVideo:     probe.HasVideo,
		Formats:      append([]MediaFormat(nil), probe.Formats...),
	}
	return cloned
}
