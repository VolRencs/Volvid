package app

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

func FindOutputProfile(profiles []OutputProfile, key string) (OutputProfile, bool) {
	for _, profile := range profiles {
		if profile.Key == key {
			return profile, true
		}
	}
	return OutputProfile{}, false
}
