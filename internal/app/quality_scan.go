package app

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
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

type qualityScanResult struct {
	info videoQualityInfo
	err  error
}

func ScanQualityChoices(urls []string) ([]QualityChoice, error) {
	urls = compactQualityURLs(urls)
	if len(urls) == 0 {
		return nil, errors.New("quality scan: empty input")
	}

	results := runQualityScan(urls)
	heights, counts, videos, scanned, firstErr := collectQualityScanResults(results, len(urls))
	if scanned == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("quality scan: no formats found")
	}

	slices.Sort(heights)
	slices.Reverse(heights)
	return buildQualityChoices(heights, counts, videos, len(urls)), nil
}

func buildQualityChoices(heights []int, counts map[int]int, videos []videoQualityInfo, total int) []QualityChoice {
	choices := make([]QualityChoice, 0, len(heights))
	for i, height := range heights {
		chain, labels := buildHeightChain(heights[i:])
		choices = append(choices, QualityChoice{
			Key:       strconv.Itoa(height),
			Height:    height,
			Available: counts[height],
			Total:     total,
			SizeBytes: estimateChoiceSize(heights[i:], videos),
			FmtChain:  chain,
			FmtLabels: labels,
		})
	}
	return choices
}

func buildHeightChain(heights []int) ([]string, []string) {
	chain := make([]string, 0, len(heights))
	labels := make([]string, 0, len(heights))
	for _, height := range heights {
		chain = append(chain, fmt.Sprintf(
			"bestvideo[height=%d]+bestaudio/best[height=%d]",
			height, height,
		))
		labels = append(labels, fmt.Sprintf("%dp", height))
	}
	return chain, labels
}

func estimateChoiceSize(heights []int, videos []videoQualityInfo) int64 {
	var (
		total int64
		found bool
	)
	for _, video := range videos {
		size, ok := estimateVideoSize(video, heights)
		if !ok {
			continue
		}
		total += size
		found = true
	}
	if !found {
		return 0
	}
	return total
}

func estimateVideoSize(video videoQualityInfo, heights []int) (int64, bool) {
	for _, height := range heights {
		if !video.hasHeight[height] {
			continue
		}
		if size := video.sizeByHeight[height]; size > 0 {
			return size, true
		}
	}
	return 0, false
}

func scanVideoInfo(url string) (videoQualityInfo, error) {
	probe, err := ProbeMediaURL(url)
	if err != nil {
		return videoQualityInfo{}, err
	}
	return videoQualityInfoFromProbe(probe)
}

func videoQualityInfoFromProbe(probe *MediaProbe) (videoQualityInfo, error) {
	if probe == nil {
		return videoQualityInfo{}, errors.New("quality scan: nil probe")
	}

	audioSize := int64(0)
	heightsSeen := make(map[int]bool)
	videoOnlySizes := make(map[int]int64)
	combinedSizes := make(map[int]int64)
	for _, format := range probe.Formats {
		size := qualityFormatSize(format)
		if format.VCodec == "none" && format.ACodec != "" && format.ACodec != "none" {
			audioSize = max(audioSize, size)
		}
		if format.Height <= 0 || format.VCodec == "" || format.VCodec == "none" {
			continue
		}
		heightsSeen[format.Height] = true
		if format.ACodec == "" || format.ACodec == "none" {
			videoOnlySizes[format.Height] = max(videoOnlySizes[format.Height], size)
			continue
		}
		combinedSizes[format.Height] = max(combinedSizes[format.Height], size)
	}
	if len(heightsSeen) == 0 {
		return videoQualityInfo{}, errors.New("quality scan: no video heights")
	}

	sizeByHeight := make(map[int]int64, len(heightsSeen))
	heights := make([]int, 0, len(heightsSeen))
	for height := range heightsSeen {
		heights = append(heights, height)
		switch {
		case videoOnlySizes[height] > 0 && audioSize > 0:
			sizeByHeight[height] = videoOnlySizes[height] + audioSize
		case videoOnlySizes[height] > 0:
			sizeByHeight[height] = videoOnlySizes[height]
		case combinedSizes[height] > 0:
			sizeByHeight[height] = combinedSizes[height]
		}
	}

	slices.Sort(heights)
	slices.Reverse(heights)
	return videoQualityInfo{
		heights:      heights,
		hasHeight:    heightsSeen,
		sizeByHeight: sizeByHeight,
		audioSize:    audioSize,
	}, nil
}

func qualityFormatSize(format MediaFormat) int64 {
	if format.Filesize > 0 {
		return format.Filesize
	}
	return format.FilesizeApprox
}

func runQualityScan(urls []string) <-chan qualityScanResult {
	jobs := make(chan string, min(len(urls), maxParallelQualityScans))
	results := make(chan qualityScanResult, len(urls))
	workers := optimalParallelism(len(urls), maxParallelQualityScans)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				info, err := scanVideoInfo(url)
				results <- qualityScanResult{info: info, err: err}
			}
		}()
	}

	go func() {
		for _, url := range urls {
			jobs <- url
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	return results
}

func collectQualityScanResults(results <-chan qualityScanResult, total int) ([]int, map[int]int, []videoQualityInfo, int, error) {
	counts := make(map[int]int)
	seen := make(map[int]bool)
	videos := make([]videoQualityInfo, 0, total)
	scanned := 0
	var firstErr error

	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		if len(res.info.heights) == 0 {
			continue
		}

		scanned++
		videos = append(videos, res.info)
		for _, height := range res.info.heights {
			counts[height]++
			seen[height] = true
		}
	}

	heights := make([]int, 0, len(seen))
	for height := range seen {
		heights = append(heights, height)
	}
	return heights, counts, videos, scanned, firstErr
}
