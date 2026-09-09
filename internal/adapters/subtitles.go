package adapters

import (
	"context"
	"errors"
	"volvid/internal/core"
)

// ResolveSubtitlesContext returns available subtitle tracks for a video URL.
// It reuses the probe cache, so right after a quality scan this is free.
func ResolveSubtitlesContext(env *Env, ctx context.Context, url string) ([]core.SubtitleTrack, error) {
	probe, err := probeMediaForURL(env, ctx, url)
	if err != nil {
		return nil, err
	}
	if len(probe.Subtitles) == 0 {
		return nil, errors.New("no subtitles available")
	}
	return probe.Subtitles, nil
}
