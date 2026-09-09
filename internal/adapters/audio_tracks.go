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
	if len(probe.AudioTracks) < 2 {
		return nil, errors.New("no alternate audio tracks")
	}
	return append([]core.AudioTrack(nil), probe.AudioTracks...), nil
}
