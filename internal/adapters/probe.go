package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"volvid/internal/core"
)

type probePayload struct {
	Duration      json.RawMessage            `json:"duration"`
	Formats       []core.MediaFormat         `json:"formats"`
	Subtitles     map[string]json.RawMessage `json:"subtitles"`
	AutomaticCaps map[string]json.RawMessage `json:"automatic_captions"`
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
		Duration:    decodeProbeDuration(payload.Duration),
		Formats:     append([]core.MediaFormat(nil), payload.Formats...),
		Subtitles:   subtitleTracksFromPayload(payload.Subtitles, payload.AutomaticCaps),
		AudioTracks: audioTracksFromFormats(payload.Formats),
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
		Duration:    probe.Duration,
		HasVideo:    probe.HasVideo,
		Formats:     append([]core.MediaFormat(nil), probe.Formats...),
		Subtitles:   append([]core.SubtitleTrack(nil), probe.Subtitles...),
		AudioTracks: append([]core.AudioTrack(nil), probe.AudioTracks...),
	}
	return cloned
}

// audioTracksFromFormats collects distinct languages of audio-only formats
// (dubbed/translated tracks). Formats without a language tag don't form a
// separate track.
func audioTracksFromFormats(formats []core.MediaFormat) []core.AudioTrack {
	seen := map[string]bool{}
	tracks := []core.AudioTrack{}
	for _, format := range formats {
		if format.VCodec != "" && format.VCodec != "none" {
			continue
		}
		if format.ACodec == "" || format.ACodec == "none" {
			continue
		}
		lang := strings.TrimSpace(format.Language)
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		tracks = append(tracks, core.AudioTrack{Lang: lang})
	}
	slices.SortFunc(tracks, func(a, b core.AudioTrack) int { return strings.Compare(a.Lang, b.Lang) })
	return tracks
}

// subtitleTracksFromPayload merges manual subtitles and automatic captions
// into a sorted track list (manual first, then auto, both by language).
func subtitleTracksFromPayload(manual, auto map[string]json.RawMessage) []core.SubtitleTrack {
	seen := map[string]bool{}
	tracks := make([]core.SubtitleTrack, 0, len(manual)+len(auto))
	for _, lang := range slices.Sorted(maps.Keys(manual)) {
		lang = strings.TrimSpace(lang)
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		tracks = append(tracks, core.SubtitleTrack{Lang: lang})
	}
	for _, lang := range slices.Sorted(maps.Keys(auto)) {
		lang = strings.TrimSpace(lang)
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		tracks = append(tracks, core.SubtitleTrack{Lang: lang, Auto: true})
	}
	return tracks
}
