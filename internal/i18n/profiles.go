package i18n

import (
	"fmt"
	"slices"
	"strings"

	"volvid/internal/core"
)

func DefaultProfileForMode(mode core.DownloadMode, l core.Locale) core.OutputProfile {
	switch mode {
	case core.ModeAudio:
		profiles := AudioOutputProfiles(l)
		if len(profiles) > 0 {
			return profiles[0]
		}
	case core.ModeThumbnail:
		return ThumbnailOutputProfile(l)
	}
	return DefaultVideoProfile(l)
}

func VideoOutputProfiles(base core.OutputProfile, l core.Locale) []core.OutputProfile {
	u := StringsFor(l)
	base = normalizeVideoBaseProfile(base, l)

	return []core.OutputProfile{
		withVideoOutput(base, u.VideoOriginal, "mp4", "", "", "", "", false),
		withVideoOutput(base, u.VideoH264, "mp4", "libx264", "20", "aac", "192k", false),
		withVideoOutput(base, u.VideoH265, "mp4", "libx265", "24", "aac", "192k", false),
		withVideoOutput(base, u.VideoVP9, "webm", "libvpx-vp9", "31", "libopus", "160k", false),
		withVideoOutput(base, u.VideoAV1, "mkv", "libsvtav1", "35", "libopus", "160k", false),
		withVideoOutput(base, u.VideoMKVCopy, "mkv", "", "", "", "", true),
	}
}

func normalizeVideoBaseProfile(base core.OutputProfile, l core.Locale) core.OutputProfile {
	if base.Mode != core.ModeVideo {
		base = DefaultVideoProfile(l)
	}
	if strings.TrimSpace(base.Label) == "" {
		base.Label = StringsFor(l).QBest
	}
	if len(base.VideoFmtChain) == 0 {
		base.VideoFmtChain = core.QualityChainAt(0)
	}
	return base
}

func withVideoOutput(
	base core.OutputProfile,
	label, container, videoCodec, crf, audioCodec, audioBitrate string,
	remuxOnly bool,
) core.OutputProfile {
	base.Label = strings.TrimSpace(base.Label + " · " + label)
	base.VideoContainer = container
	base.VideoCodec = videoCodec
	base.VideoCRF = crf
	base.AudioCodec = audioCodec
	base.AudioBitrate = audioBitrate
	base.RemuxOnly = remuxOnly
	return base
}

func AudioOutputProfiles(l core.Locale) []core.OutputProfile {
	u := StringsFor(l)
	return []core.OutputProfile{
		{Label: u.AudioMP3320, Mode: core.ModeAudio, AudioFormat: "mp3", AudioQuality: "320K"},
		{Label: u.AudioMP3192, Mode: core.ModeAudio, AudioFormat: "mp3", AudioQuality: "192K"},
		{Label: u.AudioM4ABest, Mode: core.ModeAudio, AudioFormat: "m4a", AudioQuality: "0"},
		{Label: u.AudioOpusBest, Mode: core.ModeAudio, AudioFormat: "opus", AudioQuality: "0"},
		{Label: u.AudioFLAC, Mode: core.ModeAudio, AudioFormat: "flac", AudioQuality: "0"},
	}
}

func ThumbnailOutputProfile(l core.Locale) core.OutputProfile {
	return core.OutputProfile{
		Label: StringsFor(l).OutThumbnail,
		Mode:  core.ModeThumbnail,
	}
}

func DefaultVideoProfile(l core.Locale) core.OutputProfile {
	return core.OutputProfile{
		Label:         StringsFor(l).QBest,
		Mode:          core.ModeVideo,
		VideoFmtChain: core.QualityChainAt(0),
	}
}

func QualityChoiceLabels(choices []core.QualityChoice, l core.Locale) []string {
	labels := make([]string, len(choices))
	for i, choice := range choices {
		labels[i] = qualityLabel(choice, l)
	}
	return labels
}

func qualityLabel(q core.QualityChoice, l core.Locale) string {
	label := qualityLabelWithoutSize(q, l)
	if q.SizeBytes > 0 {
		label += " ~" + FormatBytes(q.SizeBytes, l)
	}
	return label
}

func qualityLabelWithoutSize(q core.QualityChoice, l core.Locale) string {
	switch {
	case q.Best:
		return StringsFor(l).QBest
	case q.Worst:
		return StringsFor(l).QEcon
	default:
		label := fmt.Sprintf("%dp", q.Height)
		if q.Total > 1 {
			label = fmt.Sprintf("%s (%d/%d)", label, q.Available, q.Total)
		}
		return label
	}
}

func QualityProfile(q core.QualityChoice, l core.Locale) core.OutputProfile {
	return core.OutputProfile{
		Label:          qualityLabel(q, l),
		Mode:           core.ModeVideo,
		VideoFmtChain:  slices.Clone(q.FmtChain),
		VideoFmtLabels: slices.Clone(q.FmtLabels),
	}
}
