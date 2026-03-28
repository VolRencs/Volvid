package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	qualityScanTimeout          = 90 * time.Second
	maxDetailedQualityURLs      = 5
	maxParallelQualityScans     = 6
	maxParallelPlaylistDownload = 4
)

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

type videoQualityInfo struct {
	heights      []int
	hasHeight    map[int]bool
	sizeByHeight map[int]int64
	audioSize    int64
}

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

func ShouldScanQualityChoices(n int) bool {
	return n > 0 && n <= maxDetailedQualityURLs
}

func ResolveQualityChoices(urls []string) ([]QualityChoice, error) {
	urls = compactQualityURLs(urls)
	if len(urls) == 0 {
		return nil, errors.New("quality scan: empty input")
	}
	if !ShouldScanQualityChoices(len(urls)) {
		return DefaultQualityChoices(), nil
	}

	choices, err := ScanQualityChoices(urls)
	if err != nil || len(choices) == 0 {
		return DefaultQualityChoices(), err
	}
	return choices, nil
}

func AutoDownloadWorkers(items int) int {
	return optimalParallelism(items, maxParallelPlaylistDownload)
}

func FindQualityChoice(choices []QualityChoice, key string) (QualityChoice, bool) {
	for _, choice := range choices {
		if choice.Key == key {
			return choice, true
		}
	}
	return QualityChoice{}, false
}

func QualityChoiceLabels(choices []QualityChoice, l Locale) []string {
	labels := make([]string, len(choices))
	for i, choice := range choices {
		labels[i] = choice.Label(l)
	}
	return labels
}

func (q QualityChoice) Label(l Locale) string {
	label := q.labelWithoutSize(l)
	if q.SizeBytes > 0 {
		label += " ~" + FmtBytesFor(q.SizeBytes, l)
	}
	return label
}

func (q QualityChoice) labelWithoutSize(l Locale) string {
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

func (q QualityChoice) Profile(l Locale) OutputProfile {
	return OutputProfile{
		Key:            q.Key,
		Label:          q.Label(l),
		Mode:           ModeVideo,
		VideoFmtChain:  slices.Clone(q.FmtChain),
		VideoFmtLabels: slices.Clone(q.FmtLabels),
	}
}

func compactQualityURLs(urls []string) []string {
	compacted := make([]string, 0, len(urls))
	seen := make(map[string]bool, len(urls))
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		compacted = append(compacted, url)
	}
	return compacted
}
