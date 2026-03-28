package app

import (
	"errors"
	"slices"
)

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
