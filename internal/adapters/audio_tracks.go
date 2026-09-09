package adapters

import (
	"context"
	"errors"
	"volvid/internal/core"
)

// ResolveAudioTracksContext returns dubbed/translated audio tracks for a
// video URL. It reuses the probe cache, so right after a quality scan this
// is free. An error (or fewer than 2 tracks) means the picker is skipped.
func ResolveAudioTracksContext(env *Env, ctx context.Context, url string) ([]core.AudioTrack, error) {
	probe, err := probeMediaForURL(env, ctx, url)
	if err != nil {
		return nil, err
	}
	if len(probe.AudioTracks) < 2 {
		return nil, errors.New("no alternate audio tracks")
	}
	return probe.AudioTracks, nil
}
