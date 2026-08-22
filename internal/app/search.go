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

func SearchYouTubeContext(env *Env, ctx context.Context, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is empty")
	}

	results := make([]SearchResult, 0, 5)
	err := scanYTDLPJSONLines(env, ctx, searchTimeout, []string{
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
		result, ok := searchResultFromMap(entry, len(results)+1)
		if !ok {
			return
		}
		results = append(results, result)
	})

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("yt-dlp: search timeout")
		}
		if len(results) > 0 {
			return nil, fmt.Errorf("%w (%d)", err, len(results))
		}
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("search returned no results")
	}
	return results, nil
}
