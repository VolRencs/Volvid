package adapters

import (
	"errors"
	"slices"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

func PrepareDownloadRequestWithDeps(env *Env, req core.DownloadRequest, deps core.CheckDepsResult) (core.DownloadRequest, error) {
	req = normalizeDownloadRequest(env, req)
	if req.Fragment != nil && (req.Profile.Mode == core.ModeThumbnail || downloadRequestUsesPlaylist(req)) {
		req.Fragment = nil
	}
	if err := validateDownloadRequest(req, deps); err != nil {
		return core.DownloadRequest{}, err
	}
	return req, nil
}

func normalizeDownloadRequest(env *Env, req core.DownloadRequest) core.DownloadRequest {
	if req.Profile.Mode == 0 {
		req.Profile = i18n.DefaultVideoProfile(req.Locale)
	}
	if req.Locale != core.LocaleRU {
		req.Locale = core.LocaleEN
	}
	if req.OutputDir == "" {
		req.OutputDir = env.DownloadsDir()
	}
	if req.Workers <= 0 {
		req.Workers = 1
	}

	req.Profile.VideoFmtChain = slices.Clone(req.Profile.VideoFmtChain)
	req.Profile.VideoFmtLabels = slices.Clone(req.Profile.VideoFmtLabels)
	req.Profile.SubLangs = slices.Clone(req.Profile.SubLangs)
	req.Profile.AudioLangs = slices.Clone(req.Profile.AudioLangs)
	req.Entries = slices.Clone(req.Entries)
	if req.PlaylistInfo != nil {
		info := *req.PlaylistInfo
		info.Entries = slices.Clone(req.PlaylistInfo.Entries)
		req.PlaylistInfo = &info
	}
	if req.Fragment != nil {
		fragment := *req.Fragment
		req.Fragment = &fragment
	}
	return req
}

func validateDownloadRequest(req core.DownloadRequest, deps core.CheckDepsResult) error {
	switch {
	case req.Target.Kind == core.TargetUnknown || req.Target.CanonicalURL == "":
		return errors.New("download target is required")
	case req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) == 0:
		return errors.New("playlist entries are required")
	case !deps.YTDLP.Available:
		return errors.New("yt-dlp is required")
	}

	if req.Fragment != nil {
		if !req.Fragment.IsValid() {
			return errors.New("invalid download fragment")
		}
		if err := core.ValidateFragmentDuration(*req.Fragment, req.MediaDuration); err != nil {
			return err
		}
	}

	if req.Profile.NeedsFFmpeg(req.Fragment) && !deps.FFmpeg.Available {
		return downloadRequestFFmpegError(req)
	}
	return nil
}

func downloadRequestUsesPlaylist(req core.DownloadRequest) bool {
	return req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) > 0
}

func downloadRequestFFmpegError(req core.DownloadRequest) error {
	switch {
	case req.Profile.RequiresVideoPostprocessing():
		return errors.New("ffmpeg is required for video transcoding")
	case req.Fragment != nil:
		return errors.New("ffmpeg is required for fragment downloads")
	case req.Profile.Mode == core.ModeAudio:
		return errors.New("ffmpeg is required for audio conversion")
	case req.Profile.WantsSubtitles():
		return errors.New("ffmpeg is required for subtitles")
	default:
		return errors.New("ffmpeg is required")
	}
}
