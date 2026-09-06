package core

import "slices"

const (
	YtdlpBestFormat     = "bestvideo+bestaudio/best"
	YtdlpWorst360Format = "bestvideo[height<=360]+bestaudio/best[height<=360]"
)

var qualityChains = [2][]string{
	{YtdlpBestFormat, "best"},
	{YtdlpWorst360Format, "best[height<=360]", "worst"},
}

// QualityChainAt returns a clone of the static format chain (nil if OOB).
func QualityChainAt(idx int) []string {
	if idx < 0 || idx >= len(qualityChains) {
		return nil
	}
	return slices.Clone(qualityChains[idx])
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
