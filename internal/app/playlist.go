package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const playlistFetchTimeout = 15 * time.Minute

type PlaylistEntry struct {
	Index    int
	Title    string
	URL      string
	Duration int
}

type PlaylistInfo struct {
	Title   string
	Entries []PlaylistEntry
}

func FetchPlaylistInfo(url string) (*PlaylistInfo, error) {
	return FetchPlaylistInfoFor(context.Background(), url, LoadLocale())
}

func FetchPlaylistInfoFor(ctx context.Context, url string, l Locale) (*PlaylistInfo, error) {
	var (
		entries []PlaylistEntry
		first   map[string]any
		strs    = StringsFor(l)
	)

	err := scanYTDLPJSONLines(ctx, playlistFetchTimeout, []string{
		"--flat-playlist",
		"--dump-json",
		"--quiet",
		"--ignore-errors",
		"--no-warnings",
		url,
	}, func(e map[string]any) {
		if first == nil {
			first = e
		}

		n := len(entries) + 1
		entry, ok := playlistEntryFromMap(e, n, strs.VideoTitleFmt)
		if !ok {
			return
		}
		entries = append(entries, entry)
	})

	if len(entries) == 0 {
		switch {
		case err != nil:
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, errors.New(strs.PlTimeout)
			}
			return nil, err
		default:
			return nil, errors.New(strs.PlEmptyPlaylist)
		}
	}
	title := "playlist"
	if first != nil {
		title = mapString(first, "playlist_title", mapString(first, "playlist", "playlist"))
	}
	return &PlaylistInfo{Title: title, Entries: entries}, nil
}

func mapString(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func mapFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}

func mediaEntryURL(entry map[string]any) string {
	if url := mapString(entry, "url", mapString(entry, "webpage_url", "")); url != "" {
		return url
	}
	if id := mapString(entry, "id", ""); id != "" {
		return "https://youtu.be/" + id
	}
	return ""
}

func playlistEntryFromMap(entry map[string]any, index int, titleFmt string) (PlaylistEntry, bool) {
	url := mediaEntryURL(entry)
	if url == "" {
		return PlaylistEntry{}, false
	}

	defaultTitle := fmt.Sprintf(titleFmt, index)
	return PlaylistEntry{
		Index:    index,
		Title:    mapString(entry, "title", mapString(entry, "id", defaultTitle)),
		URL:      url,
		Duration: int(mapFloat(entry, "duration")),
	}, true
}

func searchResultFromMap(entry map[string]any, index int) (SearchResult, bool) {
	url := mediaEntryURL(entry)
	if url == "" {
		return SearchResult{}, false
	}

	return SearchResult{
		Title:    strings.TrimSpace(mapString(entry, "title", fmt.Sprintf("Video %d", index))),
		URL:      url,
		Duration: int(mapFloat(entry, "duration")),
	}, true
}

var (
	sepRE   = regexp.MustCompile(`[,;\s]+`)
	rangeRE = regexp.MustCompile(`^(\d+)\s*[-–]\s*(\d+)$`)
)

func ParseSelection(raw string, maxIdx int) ([]int, error) {
	return ParseSelectionFor(raw, maxIdx, LoadLocale())
}

func ParseSelectionFor(raw string, maxIdx int, l Locale) ([]int, error) {
	strs := StringsFor(l)
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return nil, errors.New(strs.PlParseEmpty)
	}
	switch raw {
	case "а", "a", "all", "все", "всё", "*":
		r := make([]int, maxIdx)
		for i := range maxIdx {
			r[i] = i + 1
		}
		return r, nil
	}
	seen := make(map[int]bool, maxIdx)
	for _, part := range sepRE.Split(raw, -1) {
		if part == "" {
			continue
		}
		if m := rangeRE.FindStringSubmatch(part); m != nil {
			a, errA := strconv.Atoi(m[1])
			b, errB := strconv.Atoi(m[2])
			if err := errors.Join(errA, errB); err != nil {
				return nil, fmt.Errorf("range %q: %w", part, err)
			}
			if a > b {
				a, b = b, a
			}
			if a < 1 || b > maxIdx {
				return nil, fmt.Errorf(strs.PlParseRange, a, b, maxIdx)
			}
			for n := a; n <= b; n++ {
				seen[n] = true
			}
		} else if n, err := strconv.Atoi(part); err == nil {
			if n < 1 || n > maxIdx {
				return nil, fmt.Errorf(strs.PlParseNum, n, maxIdx)
			}
			seen[n] = true
		} else {
			return nil, fmt.Errorf(strs.PlParseBad, part)
		}
	}
	if len(seen) == 0 {
		return nil, errors.New(strs.PlParseNone)
	}
	return slices.Sorted(maps.Keys(seen)), nil
}
