package adapters

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	if strings.TrimSpace(req.OutputDir) == "" {
		req.OutputDir = env.DownloadsDir()
	}
	if req.Workers <= 0 {
		req.Workers = 1
	}

	req.Profile.VideoFmtChain = slices.Clone(req.Profile.VideoFmtChain)
	req.Profile.VideoFmtLabels = slices.Clone(req.Profile.VideoFmtLabels)
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

	if core.ProfileRequiresFFmpeg(req.Profile, req.Fragment) && !deps.FFmpeg.Available {
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
	default:
		return errors.New("ffmpeg is required")
	}
}
func buildDownloadCommandArgs(req core.DownloadRequest, deps core.CheckDepsResult, sourceURL, outputTemplate, format string, extra []string) ([]string, error) {
	args := make([]string, 0, 20+len(extra))
	args = append(args, core.FFmpegArgs(deps)...)

	modeArgs, err := downloadModeArgs(req.Profile, format)
	if err != nil {
		return nil, err
	}
	args = append(args, modeArgs...)
	args = append(args, downloadReliabilityArgs(req)...)
	args = append(args, "-o", outputTemplate, "--windows-filenames")
	args = appendFragmentDownloadArgs(args, req)
	args = append(args, extra...)
	args = append(args, sourceURL)
	return args, nil
}
func downloadReliabilityArgs(req core.DownloadRequest) []string {
	args := []string{
		"--continue",
		"--part",
		"--retries", strconv.Itoa(ytdlpDownloadRetries),
		"--fragment-retries", strconv.Itoa(ytdlpFragmentRetries),
		"--retry-sleep", "linear=1:5:2",
		"--abort-on-unavailable-fragments",
	}
	if req.Profile.Mode != core.ModeThumbnail {
		args = append(args, "--concurrent-fragments", strconv.Itoa(ytdlpConcurrentFragments))
	}
	return args
}
func downloadModeArgs(profile core.OutputProfile, format string) ([]string, error) {
	switch profile.Mode {
	case core.ModeThumbnail:
		return []string{"--skip-download", "--write-thumbnail"}, nil
	case core.ModeAudio:
		args := []string{"-f", "bestaudio/best", "--extract-audio"}
		if profile.AudioFormat != "" {
			args = append(args, "--audio-format", profile.AudioFormat)
		}
		if profile.AudioQuality != "" {
			args = append(args, "--audio-quality", profile.AudioQuality)
		}
		return args, nil
	case core.ModeVideo:
		return videoModeArgs(profile, format), nil
	default:
		return nil, fmt.Errorf("unsupported download mode %d", profile.Mode)
	}
}
func videoModeArgs(profile core.OutputProfile, format string) []string {
	container := strings.TrimSpace(profile.VideoContainer)
	if container == "" {
		container = "mp4"
	}

	if profile.RemuxOnly {
		return []string{"-f", format, "--remux-video", container}
	}

	return []string{"-f", format, "--merge-output-format", container}
}
func appendFragmentDownloadArgs(args []string, req core.DownloadRequest) []string {
	if req.Fragment == nil {
		return args
	}

	if section, ok := req.Fragment.SectionArg(); ok {
		args = append(args, "--download-sections", section)
		if req.Profile.Mode != core.ModeAudio {
			args = append(args, "--force-keyframes-at-cuts")
		}
	}
	return args
}
