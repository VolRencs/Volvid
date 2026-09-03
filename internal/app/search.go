package app

import (
	"context"
	"errors"
	"strings"
)

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
	err := scanYTDLPJSONLines(env, ctx, searchTimeout, flatPlaylistScanArgs("ytsearch5:"+query), func(entry map[string]any) {
		if len(results) == cap(results) {
			return
		}
		result, ok := searchResultFromMap(entry, len(results)+1)
		if !ok {
			return
		}
		results = append(results, result)
	})

	if scanErr := flatScanError(err, len(results), errors.New("yt-dlp: search timeout")); scanErr != nil {
		return nil, scanErr
	}
	if len(results) == 0 {
		return nil, errors.New("search returned no results")
	}
	return results, nil
}
