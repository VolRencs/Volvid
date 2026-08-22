package app

import "slices"

var qualityChains = [3][]string{
	{"bestvideo+bestaudio/best", "bestvideo+bestaudio", "best"},
	{"bestvideo[height<=360]+bestaudio/best[height<=360]", "best[height<=360]", "worst"},
	nil,
}

func qualityChainAt(idx int) []string {
	if idx < 0 || idx >= len(qualityChains) {
		return nil
	}
	return slices.Clone(qualityChains[idx])
}
