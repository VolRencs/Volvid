package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
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
	ctx, cancel := context.WithTimeout(context.Background(), playlistFetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, YtdlpBin,
		"--flat-playlist", "--dump-json",
		"--quiet", "--ignore-errors", "--no-warnings",
		url,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("yt-dlp start: %w", err)
	}

	var entries []PlaylistEntry
	var first map[string]any

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if first == nil {
			first = e
		}
		videoURL := strVal(e, "url", strVal(e, "webpage_url", ""))
		if videoURL == "" {
			if id := strVal(e, "id", ""); id != "" {
				videoURL = "https://youtu.be/" + id
			} else {
				continue
			}
		}
		n := len(entries) + 1
		dur, _ := e["duration"].(float64)
		defTitle := fmt.Sprintf(Loc.VideoTitleFmt, n)
		entries = append(entries, PlaylistEntry{
			Index:    n,
			Title:    strVal(e, "title", strVal(e, "id", defTitle)),
			URL:      videoURL,
			Duration: int(dur),
		})
	}
	scanErr := sc.Err()
	waitErr := cmd.Wait()
	if len(entries) == 0 {
		switch {
		case scanErr != nil:
			return nil, fmt.Errorf("yt-dlp output: %w", scanErr)
		case waitErr != nil:
			if ctx.Err() != nil {
				return nil, errors.New(Loc.PlTimeout)
			}
			return nil, fmt.Errorf("yt-dlp: %w", waitErr)
		default:
			return nil, errors.New(Loc.PlEmptyPlaylist)
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
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return nil, errors.New(Loc.PlParseEmpty)
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
				return nil, fmt.Errorf(Loc.PlParseRange, a, b, maxIdx)
			}
			for n := a; n <= b; n++ {
				seen[n] = true
			}
		} else if n, err := strconv.Atoi(part); err == nil {
			if n < 1 || n > maxIdx {
				return nil, fmt.Errorf(Loc.PlParseNum, n, maxIdx)
			}
			seen[n] = true
		} else {
			return nil, fmt.Errorf(Loc.PlParseBad, part)
		}
	}
	if len(seen) == 0 {
		return nil, errors.New(Loc.PlParseNone)
	}
	return slices.Sorted(maps.Keys(seen)), nil
}
