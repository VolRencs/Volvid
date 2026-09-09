package core

import (
	"slices"
	"strings"
)

type DownloadMode uint8

const (
	ModeVideo DownloadMode = iota + 1
	ModeAudio
	ModeThumbnail
)

// FormatChain is a yt-dlp -f fallback chain with human labels.
// Labels[i] describes Formats[i] when present.
type FormatChain struct {
	Formats []string
	Labels  []string
}

// Chain returns the formats as a slice (nil when empty).
func (c FormatChain) Chain() []string {
	return slices.Clone(c.Formats)
}

func (c FormatChain) LabelAt(i int, fallback string) string {
	if i >= 0 && i < len(c.Labels) && c.Labels[i] != "" {
		return c.Labels[i]
	}
	if i >= 0 && i < len(c.Formats) {
		return c.Formats[i]
	}
	return fallback
}

const (
	YtdlpBestFormat     = "bestvideo+bestaudio/best"
	ytdlpWorst360Format = "bestvideo[height<=360]+bestaudio/best[height<=360]"
)

var qualityChains = [2]FormatChain{
	{Formats: []string{YtdlpBestFormat, "best"}},
	{Formats: []string{ytdlpWorst360Format, "best[height<=360]", "worst"}, Labels: []string{"worst", "360p", "worst"}},
}

// QualityChainAt returns a clone of the static format chain (nil if OOB).
func QualityChainAt(idx int) []string {
	if idx < 0 || idx >= len(qualityChains) {
		return nil
	}
	return qualityChains[idx].Chain()
}

type QualityChoice struct {
	Key       string
	Height    int
	Best      bool
	Worst     bool
	Available int
	Total     int
	SizeBytes int64
	FmtChain  []string
	FmtLabels []string
}

// Chain returns the choice formats as a FormatChain.
func (q QualityChoice) Chain() FormatChain {
	return FormatChain{Formats: slices.Clone(q.FmtChain), Labels: slices.Clone(q.FmtLabels)}
}

// DefaultQualityChoices is the fallback when a live scan is impossible.
func DefaultQualityChoices() []QualityChoice {
	return []QualityChoice{
		{Key: "best", Best: true, FmtChain: QualityChainAt(0)},
		{
			Key:       "worst",
			Worst:     true,
			FmtChain:  QualityChainAt(1),
			FmtLabels: []string{"worst", "360p", "worst"},
		},
	}
}

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

// Chain returns the video format chain of the profile.
func (p OutputProfile) Chain() FormatChain {
	return FormatChain{
		Formats: slices.Clone(p.VideoFmtChain),
		Labels:  slices.Clone(p.VideoFmtLabels),
	}
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

// RequiresVideoPostprocessing reports whether the profile needs remux/transcode.
func (p OutputProfile) RequiresVideoPostprocessing() bool {
	return p.videoPostprocessKind() != videoPostprocessNone
}

// NeedsVideoTranscode reports whether the profile needs a full transcode.
func (p OutputProfile) NeedsVideoTranscode() bool {
	return p.videoPostprocessKind() == videoPostprocessTranscode
}

// NeedsFFmpeg reports whether the profile/fragment combination needs ffmpeg.
func (p OutputProfile) NeedsFFmpeg(fragment *DownloadFragment) bool {
	return p.Mode == ModeAudio || fragment != nil || p.RequiresVideoPostprocessing()
}

// ProfileRequiresFFmpeg reports whether the profile/fragment combination
// needs ffmpeg (audio extraction, section cuts, transcoding).
func ProfileRequiresFFmpeg(profile OutputProfile, fragment *DownloadFragment) bool {
	return profile.NeedsFFmpeg(fragment)
}

// OutputProfileLabels extracts menu labels from profiles.
func OutputProfileLabels(profiles []OutputProfile) []string {
	labels := make([]string, len(profiles))
	for i, profile := range profiles {
		labels[i] = profile.Label
	}
	return labels
}
