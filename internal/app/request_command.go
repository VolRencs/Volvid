package app

import "fmt"

func BuildCommandSpec(req DownloadRequest, sourceURL, outputTemplate, format string, extra []string) (CommandSpec, error) {
	preparedReq, err := PrepareDownloadRequest(req)
	if err != nil {
		return CommandSpec{}, err
	}
	req = preparedReq

	args := make([]string, 0, 16+len(extra))
	args = append(args, ffmpegArgs()...)

	modeArgs, needsFFmpeg, err := buildModeCommandArgs(req.Profile, format)
	if err != nil {
		return CommandSpec{}, err
	}
	args = append(args, modeArgs...)
	args = append(args, "-o", outputTemplate, "--windows-filenames")
	args = appendFragmentCommandArgs(args, req)
	args = append(args, extra...)
	args = append(args, sourceURL)

	return CommandSpec{
		Args:        args,
		NeedsFFmpeg: needsFFmpeg,
	}, nil
}

func buildModeCommandArgs(profile OutputProfile, format string) ([]string, bool, error) {
	switch profile.Mode {
	case ModeThumbnail:
		return []string{"--skip-download", "--write-thumbnail"}, false, nil
	case ModeAudio:
		args := []string{"--extract-audio"}
		if profile.AudioFormat != "" {
			args = append(args, "--audio-format", profile.AudioFormat)
		}
		if profile.AudioQuality != "" {
			args = append(args, "--audio-quality", profile.AudioQuality)
		}
		return args, true, nil
	case ModeVideo:
		if format == "" {
			format = "bestvideo+bestaudio/best"
		}
		return []string{"-f", format, "--merge-output-format", "mp4"}, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported download mode %d", profile.Mode)
	}
}

func appendFragmentCommandArgs(args []string, req DownloadRequest) []string {
	if req.Fragment == nil {
		return args
	}
	if section, ok := req.Fragment.sectionArg(); ok {
		args = append(args, "--download-sections", section)
	}
	return args
}
