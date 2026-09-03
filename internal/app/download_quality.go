package app

import "slices"

const (
	ytdlpBestFormat     = "bestvideo+bestaudio/best"
	ytdlpWorst360Format = "bestvideo[height<=360]+bestaudio/best[height<=360]"
)

var qualityChains = [2][]string{
	{ytdlpBestFormat, "bestvideo+bestaudio/best", "best"},
	{ytdlpWorst360Format, "best[height<=360]", "worst"},
}

func qualityChainAt(idx int) []string {
	if idx < 0 || idx >= len(qualityChains) {
		return nil
	}
	return slices.Clone(qualityChains[idx])
}
