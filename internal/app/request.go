package app

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
	Fragment     *DownloadFragment
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
