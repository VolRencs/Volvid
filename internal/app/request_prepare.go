package app

import (
	"errors"
	"slices"
	"strings"
)

func normalizeDownloadRequest(req DownloadRequest) DownloadRequest {
	if req.Profile.Mode == 0 {
		req.Profile = DefaultVideoProfile(req.Locale)
	}
	if req.Locale != LocaleRU {
		req.Locale = LocaleEN
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		req.OutputDir = DlDir
	}
	if req.Workers <= 0 {
		req.Workers = 1
	}
	req.Profile.VideoFmtChain = slices.Clone(req.Profile.VideoFmtChain)
	req.Profile.VideoFmtLabels = slices.Clone(req.Profile.VideoFmtLabels)
	req.Entries = append([]PlaylistEntry(nil), req.Entries...)
	if req.Fragment != nil {
		fragment := *req.Fragment
		req.Fragment = &fragment
	}
	return req
}

func PrepareDownloadRequest(req DownloadRequest) (DownloadRequest, error) {
	req = prepareDownloadRequest(req)
	if err := validatePreparedDownloadRequest(req); err != nil {
		return DownloadRequest{}, err
	}
	return req, nil
}

func prepareDownloadRequest(req DownloadRequest) DownloadRequest {
	req = normalizeDownloadRequest(req)
	if req.Fragment != nil && !requestAllowsFragment(req) {
		req.Fragment = nil
	}
	return req
}

func validatePreparedDownloadRequest(req DownloadRequest) error {
	if req.Target.Kind == TargetUnknown || req.Target.CanonicalURL == "" {
		return errors.New("download target is required")
	}
	if req.Profile.Mode == 0 {
		return errors.New("download profile is required")
	}
	if req.Fragment != nil {
		if !req.Fragment.IsValid() {
			return errors.New("invalid download fragment")
		}
		if err := ValidateFragmentDuration(*req.Fragment, req.MediaDuration); err != nil {
			return err
		}
	}
	deps := resolveRuntimeDeps()
	if requestNeedsFFmpeg(req) && !deps.FFmpeg.Available {
		return requestFFmpegError(req)
	}
	return nil
}

func requestAllowsFragment(req DownloadRequest) bool {
	if req.Profile.Mode == ModeThumbnail {
		return false
	}
	if requestUsesPlaylist(req) {
		return false
	}
	return req.Target.IsVideo()
}

func requestNeedsFFmpeg(req DownloadRequest) bool {
	if req.Profile.Mode == ModeAudio {
		return true
	}
	return req.Fragment != nil
}

func requestFFmpegError(req DownloadRequest) error {
	switch {
	case req.Fragment != nil:
		return errors.New("ffmpeg is required for fragment downloads")
	case req.Profile.Mode == ModeAudio:
		return errors.New("ffmpeg is required for audio conversion")
	default:
		return errors.New("ffmpeg is required")
	}
}
