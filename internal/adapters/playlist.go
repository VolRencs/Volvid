package adapters

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

func FetchPlaylistInfoFor(env *Env, ctx context.Context, url string, l core.Locale) (*core.PlaylistInfo, error) {
	var (
		entries []core.PlaylistEntry
		first   map[string]any
		strs    = i18n.StringsFor(l)
	)

	err := scanYTDLPJSONLines(env, ctx, playlistFetchTimeout, flatPlaylistScanArgs(url), func(e map[string]any) {
		if first == nil {
			first = e
		}

		n := len(entries) + 1
		title, entryURL, duration, ok := mediaEntryFromMap(e, n, strs.VideoTitleFmt)
		if !ok {
			return
		}
		entries = append(entries, core.PlaylistEntry{Index: n, Title: title, URL: entryURL, Duration: duration})
	})

	if scanErr := flatScanError(err, len(entries), errors.New(strs.PlTimeout)); scanErr != nil {
		return nil, scanErr
	}
	if len(entries) == 0 {
		return nil, errors.New(strs.PlEmptyPlaylist)
	}
	title := "playlist"
	if first != nil {
		title = core.MapString(first, "playlist_title", core.MapString(first, "playlist", "playlist"))
	}
	return &core.PlaylistInfo{Title: title, Entries: entries}, nil
}

func mediaEntryURL(entry map[string]any) string {
	for _, key := range []string{"webpage_url", "original_url", "url"} {
		if value := normalizeMediaEntryURL(core.MapString(entry, key, "")); value != "" {
			return value
		}
	}
	if id := cleanMediaEntryID(core.MapString(entry, "id", "")); id != "" {
		return "https://youtu.be/" + url.QueryEscape(id)
	}
	return ""
}
func normalizeMediaEntryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if target, err := core.ParseTarget(raw); err == nil {
		if target.VideoID != "" {
			return target.VideoURL()
		}
		return target.CanonicalURL
	}
	if id := cleanMediaEntryID(raw); id != "" {
		return "https://youtu.be/" + url.QueryEscape(id)
	}
	return ""
}
func cleanMediaEntryID(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < 6 || len(raw) > 128 {
		return ""
	}
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return ""
		}
	}
	return raw
}
func searchResultFromMap(entry map[string]any, index int) (core.SearchResult, bool) {
	title, entryURL, duration, ok := mediaEntryFromMap(entry, index, "Video %d")
	if !ok {
		return core.SearchResult{}, false
	}
	return core.SearchResult{Title: title, URL: entryURL, Duration: duration}, true
}

var (
	sepRE   = regexp.MustCompile(`[,;\s]+`)
	rangeRE = regexp.MustCompile(`^(\d+)\s*[-–]\s*(\d+)$`)
)

func ParseSelectionFor(raw string, maxIdx int, l core.Locale) ([]int, error) {
	strs := i18n.StringsFor(l)
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

func SearchYouTubeContext(env *Env, ctx context.Context, query string) ([]core.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is empty")
	}

	results := make([]core.SearchResult, 0, 5)
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
