package adapters

import (
	"context"
	"errors"
	"volvid/internal/core"
)

// ResolveSubtitlesContext returns available subtitle tracks for a video URL.
// It reuses the probe cache, so right after a quality scan this is free.
func ResolveSubtitlesContext(env *Env, ctx context.Context, url string) ([]core.SubtitleTrack, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target, err := core.ParseTarget(url)
	if err != nil {
		return nil, err
	}
	probe, err := probeMediaWithDeps(env, ctx, resolveRuntimeDeps(env), target)
	if err != nil {
		return nil, err
	}
	if len(probe.Subtitles) == 0 {
		return nil, errors.New("no subtitles available")
	}
	return append([]core.SubtitleTrack(nil), probe.Subtitles...), nil
}
