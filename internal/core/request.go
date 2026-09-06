package core

import "strings"

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

type videoPostprocessKind uint8

const (
	videoPostprocessNone videoPostprocessKind = iota
	videoPostprocessRemux
	videoPostprocessTranscode
)

func (p OutputProfile) videoPostprocessKind() videoPostprocessKind {
	if p.Mode != ModeVideo {
		return videoPostprocessNone
	}
	if p.RemuxOnly {
		return videoPostprocessRemux
	}
	if strings.TrimSpace(p.VideoCodec) != "" ||
		strings.TrimSpace(p.AudioCodec) != "" ||
		strings.TrimSpace(p.VideoCRF) != "" ||
		strings.TrimSpace(p.AudioBitrate) != "" {
		return videoPostprocessTranscode
	}
	return videoPostprocessNone
}

func (p OutputProfile) RequiresVideoPostprocessing() bool {
	return p.videoPostprocessKind() != videoPostprocessNone
}

func (p OutputProfile) NeedsVideoTranscode() bool {
	return p.videoPostprocessKind() == videoPostprocessTranscode
}

// ProfileRequiresFFmpeg reports whether the profile/fragment combination
// needs ffmpeg (audio extraction, section cuts, transcoding).
func ProfileRequiresFFmpeg(profile OutputProfile, fragment *DownloadFragment) bool {
	return profile.Mode == ModeAudio || fragment != nil || profile.RequiresVideoPostprocessing()
}

// OutputProfileLabels extracts menu labels from profiles.
func OutputProfileLabels(profiles []OutputProfile) []string {
	labels := make([]string, len(profiles))
	for i, profile := range profiles {
		labels[i] = profile.Label
	}
	return labels
}
