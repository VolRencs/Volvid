package app

import "strings"

func DefaultProfileForMode(mode DownloadMode, l Locale) OutputProfile {
	switch mode {
	case ModeAudio:
		profiles := AudioOutputProfiles(l)
		if len(profiles) > 0 {
			return profiles[0]
		}
	case ModeThumbnail:
		return ThumbnailOutputProfile(l)
	}
	return DefaultVideoProfile(l)
}

func VideoOutputProfiles(base OutputProfile, l Locale) []OutputProfile {
	u := StringsFor(l)
	base = normalizeVideoBaseProfile(base, l)

	return []OutputProfile{
		withVideoOutput(base, "video_original", u.VideoOriginal, "mp4", "", "", "", "", false),
		withVideoOutput(base, "video_h264", u.VideoH264, "mp4", "libx264", "20", "aac", "192k", false),
		withVideoOutput(base, "video_h265", u.VideoH265, "mp4", "libx265", "24", "aac", "192k", false),
		withVideoOutput(base, "video_vp9", u.VideoVP9, "webm", "libvpx-vp9", "31", "libopus", "160k", false),
		withVideoOutput(base, "video_av1", u.VideoAV1, "mkv", "libsvtav1", "35", "libopus", "160k", false),
		withVideoOutput(base, "video_mkv_copy", u.VideoMKVCopy, "mkv", "", "", "", "", true),
	}
}

func normalizeVideoBaseProfile(base OutputProfile, l Locale) OutputProfile {
	if base.Mode != ModeVideo {
		base = DefaultVideoProfile(l)
	}
	if strings.TrimSpace(base.Label) == "" {
		base.Label = StringsFor(l).QBest
	}
	if len(base.VideoFmtChain) == 0 {
		base.VideoFmtChain = QualityChainAt(0)
	}
	return base
}

func withVideoOutput(
	base OutputProfile,
	key, label, container, videoCodec, crf, audioCodec, audioBitrate string,
	remuxOnly bool,
) OutputProfile {
	base.Key = strings.TrimSpace(base.Key + "_" + key)
	base.Label = strings.TrimSpace(base.Label + " · " + label)
	base.VideoContainer = container
	base.VideoCodec = videoCodec
	base.VideoCRF = crf
	base.AudioCodec = audioCodec
	base.AudioBitrate = audioBitrate
	base.RemuxOnly = remuxOnly
	return base
}

func AudioOutputProfiles(l Locale) []OutputProfile {
	u := StringsFor(l)
	return []OutputProfile{
		{Key: "audio_mp3_320", Label: u.AudioMP3320, Mode: ModeAudio, AudioFormat: "mp3", AudioQuality: "320K"},
		{Key: "audio_mp3_192", Label: u.AudioMP3192, Mode: ModeAudio, AudioFormat: "mp3", AudioQuality: "192K"},
		{Key: "audio_m4a_best", Label: u.AudioM4ABest, Mode: ModeAudio, AudioFormat: "m4a", AudioQuality: "0"},
		{Key: "audio_opus_best", Label: u.AudioOpusBest, Mode: ModeAudio, AudioFormat: "opus", AudioQuality: "0"},
		{Key: "audio_flac", Label: u.AudioFLAC, Mode: ModeAudio, AudioFormat: "flac", AudioQuality: "0"},
	}
}

func ThumbnailOutputProfile(l Locale) OutputProfile {
	return OutputProfile{
		Key:   "thumbnail",
		Label: StringsFor(l).OutThumbnail,
		Mode:  ModeThumbnail,
	}
}

func OutputProfileLabels(profiles []OutputProfile) []string {
	labels := make([]string, len(profiles))
	for i, profile := range profiles {
		labels[i] = profile.Label
	}
	return labels
}
