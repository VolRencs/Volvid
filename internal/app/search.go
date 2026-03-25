package app

import (
	"bufio"
	"context"
	"encoding/json"
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

	cmd, stdout, runCtx, cancel, err := startMergedOutputCommand(
		ctx,
		searchTimeout,
		YtdlpBin,
		"--flat-playlist",
		"--dump-json",
		"--quiet",
		"--ignore-errors",
		"--no-warnings",
		"ytsearch5:"+query,
	)
	if err != nil {
		return nil, fmt.Errorf("yt-dlp start: %w", err)
	}
	defer cancel()
	defer stdout.Close()

	results := make([]SearchResult, 0, 5)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			continue
		}
		url := strVal(entry, "url", strVal(entry, "webpage_url", ""))
		if url == "" {
			if id := strVal(entry, "id", ""); id != "" {
				url = "https://youtu.be/" + id
			}
		}
		if url == "" {
			continue
		}
		result := SearchResult{
			Title:    strings.TrimSpace(strVal(entry, "title", fmt.Sprintf("Video %d", len(results)+1))),
			URL:      url,
			Duration: int(floatVal(entry, "duration")),
		}
		results = append(results, result)
		if len(results) == 5 {
			break
		}
	}

	scanErr := sc.Err()
	waitErr := waitCommand(cmd, runCtx)
	if len(results) == 0 {
		switch {
		case scanErr != nil:
			return nil, fmt.Errorf("yt-dlp output: %w", scanErr)
		case waitErr != nil:
			if errors.Is(waitErr, context.DeadlineExceeded) {
				return nil, errors.New("yt-dlp: search timeout")
			}
			return nil, fmt.Errorf("yt-dlp: %w", waitErr)
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
