package app

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"
)

type SizeEstimate struct {
	TotalBytes   int64
	Known        bool
	KnownItems   int
	UnknownItems int
}

const maxParallelSizeEstimates = 6

var (
	heightExactRE = regexp.MustCompile(`height=(\d+)`)
	heightMaxRE   = regexp.MustCompile(`height<=?(\d+)`)
)

type sizeEstimateJob struct {
	target      ParsedTarget
	occurrences int
}

type sizeEstimateResult struct {
	size        int64
	known       bool
	occurrences int
	err         error
}

func EstimateDownloadSize(req DownloadRequest) (SizeEstimate, error) {
	return EstimateDownloadSizeContext(context.Background(), req)
}

func EstimateDownloadSizeContext(ctx context.Context, req DownloadRequest) (SizeEstimate, error) {
	preparedReq, err := PrepareDownloadRequest(req)
	if err != nil {
		return SizeEstimate{}, err
	}
	req = preparedReq

	urls := downloadRequestSourceURLs(req)
	if len(urls) == 0 {
		return SizeEstimate{}, errors.New("download size: empty input")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return SizeEstimate{}, err
	}

	jobs, estimate := buildSizeEstimateJobs(urls)
	if len(jobs) == 0 {
		return estimate, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var firstErr error
	for result := range runSizeEstimateJobs(runCtx, req, jobs) {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
				cancel()
			}
			continue
		}
		if !result.known {
			estimate.Known = false
			estimate.UnknownItems += result.occurrences
			continue
		}

		estimate.TotalBytes += result.size * int64(result.occurrences)
		estimate.KnownItems += result.occurrences
	}

	if firstErr != nil {
		return estimate, firstErr
	}
	if err := runCtx.Err(); err != nil {
		return estimate, err
	}
	return estimate, nil
}

func buildSizeEstimateJobs(urls []string) ([]sizeEstimateJob, SizeEstimate) {
	estimate := SizeEstimate{Known: true}
	counts := make(map[string]int, len(urls))
	targets := make(map[string]ParsedTarget, len(urls))
	order := make([]string, 0, len(urls))

	// Probe each canonical URL once and scale the result back by its occurrences.
	for _, rawURL := range urls {
		target, err := ParseTarget(rawURL)
		if err != nil || !target.IsVideo() {
			estimate.Known = false
			estimate.UnknownItems++
			continue
		}

		key := strings.TrimSpace(target.CanonicalURL)
		if key == "" {
			key = strings.TrimSpace(rawURL)
		}
		if counts[key] == 0 {
			order = append(order, key)
			targets[key] = target
		}
		counts[key]++
	}

	jobs := make([]sizeEstimateJob, 0, len(order))
	for _, key := range order {
		jobs = append(jobs, sizeEstimateJob{
			target:      targets[key],
			occurrences: counts[key],
		})
	}
	return jobs, estimate
}

func runSizeEstimateJobs(ctx context.Context, req DownloadRequest, jobs []sizeEstimateJob) <-chan sizeEstimateResult {
	jobCh := make(chan sizeEstimateJob)
	resultCh := make(chan sizeEstimateResult, min(len(jobs), maxParallelSizeEstimates))
	workers := optimalParallelism(len(jobs), maxParallelSizeEstimates)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobCh:
					if !ok {
						return
					}

					size, known, err := estimateTargetSize(ctx, req, job.target)
					result := sizeEstimateResult{
						size:        size,
						known:       known,
						occurrences: job.occurrences,
						err:         err,
					}

					if err != nil {
						select {
						case resultCh <- result:
						case <-ctx.Done():
						}
						return
					}

					select {
					case resultCh <- result:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			select {
			case <-ctx.Done():
				return
			case jobCh <- job:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return resultCh
}

func estimateTargetSize(ctx context.Context, req DownloadRequest, target ParsedTarget) (int64, bool, error) {
	probe, err := ProbeMediaContext(ctx, target)
	if err != nil {
		if err := estimateTargetContextError(ctx, err); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	info, err := videoQualityInfoFromProbe(probe)
	if err != nil {
		if err := estimateTargetContextError(ctx, err); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}

	switch req.Profile.Mode {
	case ModeAudio:
		if info.audioSize > 0 {
			return info.audioSize, true, nil
		}
		return 0, false, nil
	case ModeThumbnail:
		return 0, false, nil
	default:
		size, ok := estimateVideoProfileSize(info, req.Profile)
		return size, ok, nil
	}
}

func estimateTargetContextError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

func estimateVideoProfileSize(info videoQualityInfo, profile OutputProfile) (int64, bool) {
	formats := profile.VideoFmtChain
	if len(formats) == 0 {
		formats = []string{"bestvideo+bestaudio/best"}
	}

	for _, format := range formats {
		if size, ok := estimateFormatSize(info, format); ok {
			return size, true
		}
	}
	return 0, false
}

func estimateFormatSize(info videoQualityInfo, format string) (int64, bool) {
	if heights := extractExactHeights(format); len(heights) > 0 {
		return estimateVideoSize(info, heights)
	}

	if maxHeight := extractMaxHeight(format); maxHeight > 0 {
		heights := heightsAtMost(info.heights, maxHeight)
		if len(heights) > 0 {
			return estimateVideoSize(info, heights)
		}
	}

	switch {
	case strings.Contains(format, "worst"):
		heights := slices.Clone(info.heights)
		slices.Reverse(heights)
		return estimateVideoSize(info, heights)
	case strings.Contains(format, "best"):
		return estimateVideoSize(info, info.heights)
	default:
		return 0, false
	}
}

func extractExactHeights(format string) []int {
	matches := heightExactRE.FindAllStringSubmatch(format, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int]bool, len(matches))
	heights := make([]int, 0, len(matches))
	for _, match := range matches {
		height, ok := parseDigits(match[1])
		if !ok || height <= 0 || seen[height] {
			continue
		}
		seen[height] = true
		heights = append(heights, height)
	}
	return heights
}

func extractMaxHeight(format string) int {
	matches := heightMaxRE.FindAllStringSubmatch(format, -1)
	maxHeight := 0
	for _, match := range matches {
		if height, ok := parseDigits(match[1]); ok {
			maxHeight = max(maxHeight, height)
		}
	}
	return maxHeight
}

func heightsAtMost(heights []int, maxHeight int) []int {
	filtered := make([]int, 0, len(heights))
	for _, height := range heights {
		if height <= maxHeight {
			filtered = append(filtered, height)
		}
	}
	return filtered
}
