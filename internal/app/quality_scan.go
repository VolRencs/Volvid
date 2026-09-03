package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
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
}

func DefaultQualityChoices() []QualityChoice {
	return []QualityChoice{
		{Key: "best", Best: true, FmtChain: qualityChainAt(0)},
		{
			Key:       "worst",
			Worst:     true,
			FmtChain:  qualityChainAt(1),
			FmtLabels: []string{"worst", "360p", "worst"},
		},
	}
}

func shouldScanQualityChoices(n int) bool {
	return n > 0 && n <= maxDetailedQualityURLs
}

func ResolveQualityChoicesContext(env *Env, ctx context.Context, urls []string) ([]QualityChoice, error) {
	urls = dedupeStrings(urls, func(s string) string { return s })
	if len(urls) == 0 {
		return nil, errors.New("quality scan: empty input")
	}
	if !shouldScanQualityChoices(len(urls)) {
		return DefaultQualityChoices(), nil
	}

	choices, err := scanQualityChoicesContext(env, ctx, urls)
	if err != nil || len(choices) == 0 {
		return DefaultQualityChoices(), err
	}
	return choices, nil
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

type qualityScanResult struct {
	info videoQualityInfo
	err  error
}

func scanQualityChoicesContext(env *Env, ctx context.Context, urls []string) ([]QualityChoice, error) {
	if len(urls) == 0 {
		return nil, errors.New("quality scan: empty input")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := runQualityScan(env, ctx, urls)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	heights, counts, videos, scanned, firstErr := collectQualityScanResults(results, len(urls))
	if scanned == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("quality scan: no formats found")
	}

	slices.SortFunc(heights, func(a, b int) int { return cmp.Compare(b, a) })
	return buildQualityChoices(heights, counts, videos, len(urls)), nil
}

func buildQualityChoices(heights []int, counts map[int]int, videos []videoQualityInfo, total int) []QualityChoice {
	chains := make([]string, len(heights))
	labels := make([]string, len(heights))
	for i, height := range heights {
		chains[i] = fmt.Sprintf(
			"bestvideo[height=%d]+bestaudio/best[height=%d]",
			height, height,
		)
		labels[i] = fmt.Sprintf("%dp", height)
	}
	best := make([][]int64, len(videos))
	for v, video := range videos {
		row := make([]int64, len(heights)+1)
		for i := len(heights) - 1; i >= 0; i-- {
			row[i] = row[i+1]
			if size := video.sizeByHeight[heights[i]]; size > 0 {
				row[i] = size
			}
		}
		best[v] = row
	}
	choices := make([]QualityChoice, 0, len(heights))
	for i, height := range heights {
		var size int64
		for _, row := range best {
			size += row[i]
		}
		choices = append(choices, QualityChoice{
			Key:       strconv.Itoa(height),
			Height:    height,
			Available: counts[height],
			Total:     total,
			SizeBytes: size,
			FmtChain:  chains[i:],
			FmtLabels: labels[i:],
		})
	}
	return choices
}

func scanVideoInfoContext(env *Env, ctx context.Context, deps CheckDepsResult, url string) (videoQualityInfo, error) {
	target, err := ParseTarget(url)
	if err != nil {
		return videoQualityInfo{}, err
	}

	probe, err := probeMediaWithDeps(env, ctx, deps, target)
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

	slices.SortFunc(heights, func(a, b int) int { return cmp.Compare(b, a) })
	return videoQualityInfo{
		heights:      heights,
		hasHeight:    heightsSeen,
		sizeByHeight: sizeByHeight,
	}, nil
}

func qualityFormatSize(format MediaFormat) int64 {
	if format.Filesize > 0 {
		return format.Filesize
	}
	return format.FilesizeApprox
}

func runQualityScan(env *Env, ctx context.Context, urls []string) <-chan qualityScanResult {
	jobs := make(chan string)
	results := make(chan qualityScanResult, len(urls))
	workers := optimalParallelism(len(urls), maxParallelQualityScans)
	deps := resolveRuntimeDeps(env)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case url, ok := <-jobs:
					if !ok {
						return
					}

					info, err := scanVideoInfoContext(env, ctx, deps, url)
					select {
					case results <- qualityScanResult{info: info, err: err}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, url := range urls {
			select {
			case <-ctx.Done():
				return
			case jobs <- url:
			}
		}
	}()

	go func() {
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

	heights := slices.Sorted(maps.Keys(seen))
	return heights, counts, videos, scanned, firstErr
}
