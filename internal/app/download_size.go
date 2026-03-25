package app

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
)

type SizeEstimate struct {
	TotalBytes   int64
	Known        bool
	KnownItems   int
	UnknownItems int
}

var (
	heightExactRE = regexp.MustCompile(`height=(\d+)`)
	heightMaxRE   = regexp.MustCompile(`height<=?(\d+)`)
)

func EstimateDownloadSize(req DownloadRequest) (SizeEstimate, error) {
	return EstimateDownloadSizeContext(context.Background(), req)
}

func EstimateDownloadSizeContext(ctx context.Context, req DownloadRequest) (SizeEstimate, error) {
	req = NormalizeDownloadRequest(req)
	if err := ValidateDownloadRequest(req); err != nil {
		return SizeEstimate{}, err
	}

	urls := requestSourceURLs(req)
	if len(urls) == 0 {
		return SizeEstimate{}, errors.New("download size: empty input")
	}

	estimate := SizeEstimate{Known: true}
	for _, rawURL := range urls {
		target, err := ParseTarget(rawURL)
		if err != nil || !target.IsVideo() {
			estimate.Known = false
			estimate.UnknownItems++
			continue
		}

		size, ok, err := estimateTargetSize(ctx, req, target)
		if err != nil {
			return estimate, err
		}
		if !ok {
			estimate.Known = false
			estimate.UnknownItems++
			continue
		}

		estimate.TotalBytes += size
		estimate.KnownItems++
	}
	return estimate, nil
}

func estimateTargetSize(ctx context.Context, req DownloadRequest, target ParsedTarget) (int64, bool, error) {
	probe, err := ProbeMediaContext(ctx, target)
	if err != nil {
		return 0, false, nil
	}
	info, err := videoQualityInfoFromProbe(probe)
	if err != nil {
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
		return estimateVideoSize(info, heights, false)
	}

	if maxHeight := extractMaxHeight(format); maxHeight > 0 {
		heights := make([]int, 0, len(info.heights))
		for _, height := range info.heights {
			if height <= maxHeight {
				heights = append(heights, height)
			}
		}
		if len(heights) > 0 {
			return estimateVideoSize(info, heights, false)
		}
	}

	switch {
	case strings.Contains(format, "worst"):
		heights := slices.Clone(info.heights)
		slices.Reverse(heights)
		return estimateVideoSize(info, heights, false)
	case strings.Contains(format, "best"):
		return estimateVideoSize(info, info.heights, false)
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
		height := atoi(match[1])
		if height <= 0 || seen[height] {
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
		maxHeight = max(maxHeight, atoi(match[1]))
	}
	return maxHeight
}

func atoi(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
