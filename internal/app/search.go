package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const searchTimeout = 90 * time.Second

type SearchResult struct {
	Title    string
	URL      string
	Duration int
}

func SearchYouTube(query string) ([]SearchResult, error) {
	return SearchYouTubeContext(context.Background(), query)
}

func SearchYouTubeContext(ctx context.Context, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is empty")
	}

	results := make([]SearchResult, 0, 5)
	err := scanYTDLPJSONLines(ctx, searchTimeout, []string{
		"--flat-playlist",
		"--dump-json",
		"--quiet",
		"--ignore-errors",
		"--no-warnings",
		"ytsearch5:" + query,
	}, func(entry map[string]any) {
		if len(results) == cap(results) {
			return
		}
		url := strVal(entry, "url", strVal(entry, "webpage_url", ""))
		if url == "" {
			if id := strVal(entry, "id", ""); id != "" {
				url = "https://youtu.be/" + id
			}
		}
		if url == "" {
			return
		}
		result := SearchResult{
			Title:    strings.TrimSpace(strVal(entry, "title", fmt.Sprintf("Video %d", len(results)+1))),
			URL:      url,
			Duration: int(floatVal(entry, "duration")),
		}
		results = append(results, result)
	})

	if len(results) == 0 {
		switch {
		case err != nil:
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, errors.New("yt-dlp: search timeout")
			}
			return nil, err
		default:
			return nil, errors.New("search returned no results")
		}
	}
	return results, nil
}

func floatVal(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}
