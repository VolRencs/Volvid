package adapters

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"volvid/internal/core"
)

type videoQualityInfo struct {
	heights      []int
	sizeByHeight map[int]int64
}

func shouldScanQualityChoices(n int) bool {
	return n > 0 && n <= maxDetailedQualityURLs
}
func ResolveQualityChoicesContext(env *Env, ctx context.Context, urls []string) ([]core.QualityChoice, error) {
	urls = dedupeStrings(urls, func(s string) string { return s })
	if len(urls) == 0 {
		return nil, errors.New("quality scan: empty input")
	}
	if !shouldScanQualityChoices(len(urls)) {
		return core.DefaultQualityChoices(), nil
	}

	choices, err := scanQualityChoicesContext(env, ctx, urls)
	if err != nil || len(choices) == 0 {
		return core.DefaultQualityChoices(), err
	}
	return choices, nil
}

type qualityScanResult struct {
	info videoQualityInfo
	err  error
}

func scanQualityChoicesContext(env *Env, ctx context.Context, urls []string) ([]core.QualityChoice, error) {
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
	heights, counts, videos, scanned, firstErr := collectQualityScanResults(ctx, results, len(urls))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scanned == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("quality scan: no formats found")
	}

	slices.SortFunc(heights, func(a, b int) int { return cmp.Compare(b, a) })
	return buildQualityChoices(heights, counts, videos, len(urls)), nil
}
func buildQualityChoices(heights []int, counts map[int]int, videos []videoQualityInfo, total int) []core.QualityChoice {
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
	choices := make([]core.QualityChoice, 0, len(heights))
	for i, height := range heights {
		var size int64
		for _, row := range best {
			size += row[i]
		}
		choices = append(choices, core.QualityChoice{
			Key:       strconv.Itoa(height),
			Height:    height,
			Available: counts[height],
			Total:     total,
			SizeBytes: size,
			FmtChain:  slices.Clone(chains[i:]),
			FmtLabels: slices.Clone(labels[i:]),
		})
	}
	return choices
}
func scanVideoInfoContext(env *Env, ctx context.Context, deps core.CheckDepsResult, url string) (videoQualityInfo, error) {
	target, err := core.ParseTarget(url)
	if err != nil {
		return videoQualityInfo{}, err
	}

	probe, err := probeMediaWithDeps(env, ctx, deps, target)
	if err != nil {
		return videoQualityInfo{}, err
	}
	return videoQualityInfoFromProbe(probe)
}
func videoQualityInfoFromProbe(probe *core.MediaProbe) (videoQualityInfo, error) {
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
		sizeByHeight: sizeByHeight,
	}, nil
}
func qualityFormatSize(format core.MediaFormat) int64 {
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
func collectQualityScanResults(ctx context.Context, results <-chan qualityScanResult, total int) ([]int, map[int]int, []videoQualityInfo, int, error) {
	counts := make(map[int]int)
	seen := make(map[int]bool)
	videos := make([]videoQualityInfo, 0, total)
	scanned := 0
	var firstErr error

	for {
		select {
		case <-ctx.Done():
			heights := slices.Sorted(maps.Keys(seen))
			return heights, counts, videos, scanned, firstErr
		case res, ok := <-results:
			if !ok {
				heights := slices.Sorted(maps.Keys(seen))
				return heights, counts, videos, scanned, firstErr
			}
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
	}
}
