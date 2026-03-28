package app

import (
	"errors"
	"slices"
	"sync"
)

type qualityScanResult struct {
	info videoQualityInfo
	err  error
}

func autoQualityScanWorkers(items int) int {
	return optimalParallelism(items, maxParallelQualityScans)
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

func runQualityScan(urls []string) <-chan qualityScanResult {
	jobs := make(chan string, min(len(urls), maxParallelQualityScans))
	results := make(chan qualityScanResult, len(urls))
	workers := autoQualityScanWorkers(len(urls))

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
		for _, h := range res.info.heights {
			counts[h]++
			seen[h] = true
		}
	}

	heights := make([]int, 0, len(seen))
	for h := range seen {
		heights = append(heights, h)
	}
	return heights, counts, videos, scanned, firstErr
}
