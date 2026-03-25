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

func DefaultDownloadMode() DownloadMode {
	return ModeVideo
}

type OutputProfile struct {
	Key            string
	Label          string
	Mode           DownloadMode
	VideoFmtChain  []string
	VideoFmtLabels []string
	AudioFormat    string
	AudioQuality   string
}

type DownloadRequest struct {
	Target       ParsedTarget
	Profile      OutputProfile
	ForceSingle  bool
	PlaylistInfo *PlaylistInfo
	Entries      []PlaylistEntry
	Workers      int
	OutputDir    string
	Locale       Locale
}

type CommandSpec struct {
	Args        []string
	NeedsFFmpeg bool
}

func DefaultVideoProfile(l Locale) OutputProfile {
	return OutputProfile{
		Key:           "best",
		Label:         StringsFor(l).QBest,
		Mode:          ModeVideo,
		VideoFmtChain: QualityChainAt(0),
	}
}

func NormalizeDownloadRequest(req DownloadRequest) DownloadRequest {
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
	return req
}

func ValidateDownloadRequest(req DownloadRequest) error {
	req = NormalizeDownloadRequest(req)

	if req.Target.Kind == TargetUnknown || req.Target.CanonicalURL == "" {
		return errors.New("download target is required")
	}
	if req.Profile.Mode == 0 {
		return errors.New("download profile is required")
	}
	if req.Profile.Mode == ModeAudio && FFmpegResolved == "" {
		return errors.New("ffmpeg is required for audio conversion")
	}
	return nil
}

func BuildCommandSpec(req DownloadRequest, sourceURL, outputTemplate, format string, extra []string) (CommandSpec, error) {
	req = NormalizeDownloadRequest(req)
	if err := ValidateDownloadRequest(req); err != nil {
		return CommandSpec{}, err
	}

	args := make([]string, 0, 16+len(extra))
	args = append(args, ffmpegArgs()...)

	switch req.Profile.Mode {
	case ModeThumbnail:
		args = append(args, "--skip-download", "--write-thumbnail")
	case ModeAudio:
		args = append(args, "--extract-audio")
		if req.Profile.AudioFormat != "" {
			args = append(args, "--audio-format", req.Profile.AudioFormat)
		}
		if req.Profile.AudioQuality != "" {
			args = append(args, "--audio-quality", req.Profile.AudioQuality)
		}
	case ModeVideo:
		if format == "" {
			format = "bestvideo+bestaudio/best"
		}
		args = append(args, "-f", format, "--merge-output-format", "mp4")
	default:
		return CommandSpec{}, fmt.Errorf("unsupported download mode %d", req.Profile.Mode)
	}

	args = append(args, "-o", outputTemplate, "--windows-filenames")
	args = append(args, extra...)
	args = append(args, sourceURL)

	return CommandSpec{
		Args:        args,
		NeedsFFmpeg: req.Profile.Mode == ModeAudio,
	}, nil
}
