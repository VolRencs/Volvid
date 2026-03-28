package app

import (
	"fmt"
	"strings"
)

type fragmentStrategy uint8

const (
	fragmentNone fragmentStrategy = iota
	fragmentVideo
	fragmentAudio
)

func buildPreparedDownloadArgs(req DownloadRequest, sourceURL, outputTemplate, format string, extra []string) ([]string, error) {
	args := make([]string, 0, 20+len(extra))
	args = append(args, ffmpegArgs()...)

	modeArgs, err := downloadModeArgs(req.Profile, format)
	if err != nil {
		return nil, err
	}
	args = append(args, modeArgs...)
	args = append(args, baseDownloadArgs(outputTemplate)...)
	args = appendFragmentArgs(args, req)
	args = append(args, extra...)
	args = append(args, sourceURL)

	return args, nil
}

func baseDownloadArgs(outputTemplate string) []string {
	return []string{"-o", outputTemplate, "--windows-filenames"}
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
		return []string{"-f", format, "--merge-output-format", "mp4"}, nil
	default:
		return nil, fmt.Errorf("unsupported download mode %d", profile.Mode)
	}
}

func appendFragmentArgs(args []string, req DownloadRequest) []string {
	if req.Fragment == nil {
		return args
	}
	if section, ok := req.Fragment.sectionArg(); ok {
		args = append(args, "--download-sections", section)
		if fragmentDownloadStrategy(req) == fragmentVideo {
			args = append(args, "--force-keyframes-at-cuts")
		}
	}
	return args
}

func fragmentDownloadStrategy(req DownloadRequest) fragmentStrategy {
	if req.Fragment == nil {
		return fragmentNone
	}
	if req.Profile.Mode == ModeAudio {
		return fragmentAudio
	}
	return fragmentVideo
}

func audioOutputExtension(profile OutputProfile) string {
	switch strings.ToLower(strings.TrimSpace(profile.AudioFormat)) {
	case "", "best":
		return "mp3"
	case "aac":
		return "m4a"
	default:
		return strings.ToLower(strings.TrimSpace(profile.AudioFormat))
	}
}
