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
		videoURL := strVal(e, "url", strVal(e, "webpage_url", ""))
		if videoURL == "" {
			if id := strVal(e, "id", ""); id != "" {
				videoURL = "https://youtu.be/" + id
			} else {
				return
			}
		}
		n := len(entries) + 1
		dur, _ := e["duration"].(float64)
		defTitle := fmt.Sprintf(StringsFor(l).VideoTitleFmt, n)
		entries = append(entries, PlaylistEntry{
			Index:    n,
			Title:    strVal(e, "title", strVal(e, "id", defTitle)),
			URL:      videoURL,
			Duration: int(dur),
		})
	})

	if len(entries) == 0 {
		switch {
		case err != nil:
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, errors.New(StringsFor(l).PlTimeout)
			}
			return nil, err
		default:
			return nil, errors.New(StringsFor(l).PlEmptyPlaylist)
		}
	}
	title := "playlist"
	if first != nil {
		title = strVal(first, "playlist_title", strVal(first, "playlist", "playlist"))
	}
	return &PlaylistInfo{Title: title, Entries: entries}, nil
}

func strVal(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
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
