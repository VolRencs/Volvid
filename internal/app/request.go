package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type DownloadMode uint8

const (
	ModeVideo DownloadMode = iota + 1
	ModeAudio
	ModeThumbnail
)

type OutputProfile struct {
	Key            string
	Label          string
	Mode           DownloadMode
	VideoFmtChain  []string
	VideoFmtLabels []string
	VideoContainer string
	VideoCodec     string
	VideoCRF       string
	AudioCodec     string
	AudioBitrate   string
	RemuxOnly      bool
	AudioFormat    string
	AudioQuality   string
}

type DownloadRequest struct {
	Target        ParsedTarget
	Profile       OutputProfile
	Fragment      *DownloadFragment
	MediaDuration int
	ForceSingle   bool
	PlaylistInfo  *PlaylistInfo
	Entries       []PlaylistEntry
	Workers       int
	OutputDir     string
	Locale        Locale
}

const (
	ytdlpDownloadRetries     = "10"
	ytdlpFragmentRetries     = "10"
	ytdlpConcurrentFragments = "4"
)

func DefaultDownloadMode() DownloadMode {
	return ModeVideo
}

func DefaultVideoProfile(l Locale) OutputProfile {
	return OutputProfile{
		Key:           "best",
		Label:         StringsFor(l).QBest,
		Mode:          ModeVideo,
		VideoFmtChain: QualityChainAt(0),
	}
}

func PrepareDownloadRequest(req DownloadRequest) (DownloadRequest, error) {
	req = normalizeDownloadRequest(req)
	if req.Fragment != nil && !downloadRequestAllowsFragment(req) {
		req.Fragment = nil
	}
	if err := validateDownloadRequest(req, resolveRuntimeDeps()); err != nil {
		return DownloadRequest{}, err
	}
	return req, nil
}

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
	if req.PlaylistInfo != nil {
		info := *req.PlaylistInfo
		info.Entries = append([]PlaylistEntry(nil), req.PlaylistInfo.Entries...)
		req.PlaylistInfo = &info
	}
	if req.Fragment != nil {
		fragment := *req.Fragment
		req.Fragment = &fragment
	}
	return req
}

func validateDownloadRequest(req DownloadRequest, deps CheckDepsResult) error {
	switch {
	case req.Target.Kind == TargetUnknown || req.Target.CanonicalURL == "":
		return errors.New("download target is required")
	case req.Profile.Mode == 0:
		return errors.New("download profile is required")
	case req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) == 0:
		return errors.New("playlist entries are required")
	case !deps.YTDLP.Available:
		return errors.New("yt-dlp is required")
	}

	if req.Fragment != nil {
		if !req.Fragment.IsValid() {
			return errors.New("invalid download fragment")
		}
		if err := ValidateFragmentDuration(*req.Fragment, req.MediaDuration); err != nil {
			return err
		}
	}

	if downloadRequestRequiresFFmpeg(req) && !deps.FFmpeg.Available {
		return downloadRequestFFmpegError(req)
	}
	return nil
}

func downloadRequestUsesPlaylist(req DownloadRequest) bool {
	return req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) > 0
}

func downloadRequestEntries(req DownloadRequest) []PlaylistEntry {
	if !downloadRequestUsesPlaylist(req) {
		return nil
	}
	return append([]PlaylistEntry(nil), req.Entries...)
}

func downloadRequestAllowsFragment(req DownloadRequest) bool {
	if req.Profile.Mode == ModeThumbnail || downloadRequestUsesPlaylist(req) {
		return false
	}
	return req.Target.IsVideo()
}

func downloadRequestRequiresFFmpeg(req DownloadRequest) bool {
	return req.Profile.Mode == ModeAudio || req.Fragment != nil || req.Profile.RequiresVideoPostprocessing()
}

func downloadRequestFFmpegError(req DownloadRequest) error {
	switch {
	case req.Profile.RequiresVideoPostprocessing():
		return errors.New("ffmpeg is required for video transcoding")
	case req.Fragment != nil:
		return errors.New("ffmpeg is required for fragment downloads")
	case req.Profile.Mode == ModeAudio:
		return errors.New("ffmpeg is required for audio conversion")
	default:
		return errors.New("ffmpeg is required")
	}
}

func buildDownloadCommandArgs(req DownloadRequest, deps CheckDepsResult, sourceURL, outputTemplate, format string, extra []string) ([]string, error) {
	args := make([]string, 0, 20+len(extra))
	args = append(args, ffmpegArgs(deps)...)

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

func downloadReliabilityArgs(req DownloadRequest) []string {
	args := []string{
		"--continue",
		"--part",
		"--retries", ytdlpDownloadRetries,
		"--fragment-retries", ytdlpFragmentRetries,
		"--retry-sleep", "linear=1:5:2",
		"--abort-on-unavailable-fragments",
	}
	if req.Profile.Mode != ModeThumbnail {
		args = append(args, "--concurrent-fragments", ytdlpConcurrentFragments)
	}
	return args
}

func downloadModeArgs(profile OutputProfile, format string) ([]string, error) {
	switch profile.Mode {
	case ModeThumbnail:
		return []string{"--skip-download", "--write-thumbnail"}, nil
	case ModeAudio:
		args := []string{"-f", "bestaudio/best", "--extract-audio"}
		if profile.AudioFormat != "" {
			args = append(args, "--audio-format", profile.AudioFormat)
		}
		if profile.AudioQuality != "" {
			args = append(args, "--audio-quality", profile.AudioQuality)
		}
		return args, nil
	case ModeVideo:
		if format == "" {
			format = "bestvideo+bestaudio/best"
		}
		return videoModeArgs(profile, format), nil
	default:
		return nil, fmt.Errorf("unsupported download mode %d", profile.Mode)
	}
}

func videoModeArgs(profile OutputProfile, format string) []string {
	container := strings.TrimSpace(profile.VideoContainer)
	if container == "" {
		container = "mp4"
	}

	args := []string{"-f", format, "--merge-output-format", container}
	if !profile.RequiresVideoPostprocessing() {
		return args
	}

	if profile.RemuxOnly {
		args = append(args, "--remux-video", container)
	}
	return args
}

func (p OutputProfile) RequiresVideoPostprocessing() bool {
	if p.Mode != ModeVideo {
		return false
	}
	return strings.TrimSpace(p.VideoCodec) != "" ||
		strings.TrimSpace(p.AudioCodec) != "" ||
		strings.TrimSpace(p.VideoCRF) != "" ||
		strings.TrimSpace(p.AudioBitrate) != "" ||
		p.RemuxOnly
}

func (p OutputProfile) NeedsVideoTranscode() bool {
	if p.Mode != ModeVideo || p.RemuxOnly {
		return false
	}
	return strings.TrimSpace(p.VideoCodec) != "" ||
		strings.TrimSpace(p.AudioCodec) != "" ||
		strings.TrimSpace(p.VideoCRF) != "" ||
		strings.TrimSpace(p.AudioBitrate) != ""
}

func appendFragmentDownloadArgs(args []string, req DownloadRequest) []string {
	if req.Fragment == nil {
		return args
	}

	// yt-dlp can trim both audio and video natively, so fragments stay on the common download path.
	if section, ok := req.Fragment.sectionArg(); ok {
		args = append(args, "--download-sections", section)
		if req.Profile.Mode != ModeAudio {
			args = append(args, "--force-keyframes-at-cuts")
		}
	}
	return args
}
