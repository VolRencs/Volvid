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
	Duration      any                        `json:"duration"`
	Formats       []core.MediaFormat         `json:"formats"`
	Subtitles     map[string]json.RawMessage `json:"subtitles"`
	AutomaticCaps map[string]json.RawMessage `json:"automatic_captions"`
}

var ErrMediaDurationUnavailable = errors.New("media duration unavailable")

// probeMediaForURL parses the target and returns a fresh cached probe.
// probeMediaWithDeps already clones, so callers own the returned slices.
func probeMediaForURL(env *Env, ctx context.Context, url string) (*core.MediaProbe, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target, err := core.ParseTarget(url)
	if err != nil {
		return nil, err
	}
	return probeMediaWithDeps(env, ctx, resolveRuntimeDeps(env), target)
}

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

	probe, err := env.probeCache.Load(key, probeCacheTTL, ctx, func() (*core.MediaProbe, error) {
		return probeMediaUncached(ctx, deps, target)
	})
	if err != nil {
		return nil, err
	}
	return cloneMediaProbe(probe), nil
}
func probeMediaUncached(ctx context.Context, deps core.CheckDepsResult, target core.ParsedTarget) (*core.MediaProbe, error) {
	out, err := ytdlpOutput(
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
	// Key by video ID when known: the fragment probe uses CanonicalURL
	// (with ?list= for mixed targets) while the quality scan uses
	// DownloadURL(forceSingle) — same video, different strings.
	// Without this, one video costs two --dump-single-json probes.
	if strings.TrimSpace(target.VideoID) != "" {
		return target.VideoURL()
	}
	key := strings.TrimSpace(target.CanonicalURL)
	if key != "" {
		return key
	}
	return strings.TrimSpace(target.DownloadURL(false))
}
func decodeProbeDuration(raw any) int {
	// Duration arrives as number/null; reuse the tolerant core decoder
	// instead of a second json.Unmarshal of the same payload.
	seconds := core.MapFloat(map[string]any{"duration": raw}, "duration")
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

// sortedUniqueLangs returns trimmed non-empty map keys, sorted and deduped.
func sortedUniqueLangs(m map[string]json.RawMessage) []string {
	langs := make([]string, 0, len(m))
	for _, lang := range slices.Sorted(maps.Keys(m)) {
		if lang = strings.TrimSpace(lang); lang != "" {
			langs = append(langs, lang)
		}
	}
	return slices.Compact(langs)
}

// audioTracksFromFormats collects distinct languages of audio-only formats
// (dubbed/translated tracks). Formats without a language tag don't form a
// separate track.
func audioTracksFromFormats(formats []core.MediaFormat) []core.AudioTrack {
	langs := make([]string, 0)
	for _, format := range formats {
		if format.VCodec != "" && format.VCodec != "none" {
			continue
		}
		if format.ACodec == "" || format.ACodec == "none" {
			continue
		}
		if lang := strings.TrimSpace(format.Language); lang != "" {
			langs = append(langs, lang)
		}
	}
	slices.Sort(langs)
	tracks := make([]core.AudioTrack, 0, len(langs))
	for _, lang := range slices.Compact(langs) {
		tracks = append(tracks, core.AudioTrack{Lang: lang})
	}
	return tracks
}

// subtitleTracksFromPayload merges manual subtitles and automatic captions
// into a sorted track list (manual first, then auto, both by language).
func subtitleTracksFromPayload(manual, auto map[string]json.RawMessage) []core.SubtitleTrack {
	manualLangs := sortedUniqueLangs(manual)
	tracks := make([]core.SubtitleTrack, 0, len(manualLangs)+len(auto))
	for _, lang := range manualLangs {
		tracks = append(tracks, core.SubtitleTrack{Lang: lang})
	}
	for _, lang := range sortedUniqueLangs(auto) {
		if slices.Contains(manualLangs, lang) {
			continue
		}
		tracks = append(tracks, core.SubtitleTrack{Lang: lang, Auto: true})
	}
	return tracks
}
