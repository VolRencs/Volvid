package app

import (
	"fmt"
	"strconv"
)

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
	total := int64(0)
	found := false
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
