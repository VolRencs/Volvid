package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	dlRE    = regexp.MustCompile(`(?i)\[download\]\s+(?P<pct>[\d.]+)%\s+of\s+~?\s*(?P<size>[\d.]+)\s*(?P<unit>[KMGTkmgt]i?[Bb])`)
	speedRE = regexp.MustCompile(`(?i)at\s+(?P<speed>[\d.]+\s*[KMGTkmgt]i?[Bb]/s)`)
	destRE  = regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`)
	procRE  = regexp.MustCompile(`(?i)^\s*\[(Merger|ExtractAudio|Thumbnails?Convertor)\]`)
	numRE   = regexp.MustCompile(`^\d+\s*[-–]\s*`)
)

type DlEventType uint8

const (
	EvStart DlEventType = iota
	EvDest
	EvProgress
	EvProc
	EvDone
	EvReset
	EvFallback
	EvClosed
)

type DlUpdate struct {
	Type    DlEventType
	Slot    int
	Text    string
	ErrText string
	Pct     float64
	DoneB   int64
	TotalB  int64
	Speed   string
	OK      bool
}

type downloadResult struct {
	OutputPath string
	ErrText    string
	Err        error
}

func subexp(re *regexp.Regexp, m []string, name string) string {
	if i := re.SubexpIndex(name); i >= 0 && i < len(m) {
		return m[i]
	}
	return ""
}

func ffmpegArgs() []string {
	if FFmpegResolved != "" {
		return []string{"--ffmpeg-location", FFmpegResolved}
	}
	return nil
}

func streamYtdlp(ctx context.Context, slot int, l Locale, args []string, ch chan<- DlUpdate) downloadResult {
	cmd, pr, runCtx, cancel, err := startMergedOutputCommand(
		ctx,
		0,
		YtdlpBin,
		slices.Concat([]string{"--newline", "--no-warnings"}, args)...,
	)
	if err != nil {
		return failedDownload(err)
	}
	defer cancel()
	defer pr.Close()

	loc := StringsFor(l)
	result := downloadResult{}

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			continue
		case parsePrintedOutputPath(line, &result):
			continue
		case strings.HasPrefix(strings.ToLower(line), "error:"):
			result.ErrText = strings.TrimSpace(line[6:])
		}

		switch {
		case destRE.MatchString(line):
			m := destRE.FindStringSubmatch(line)
			stem := strings.TrimSpace(m[1])
			stem = numRE.ReplaceAllString(
				filepath.Base(strings.TrimSuffix(stem, filepath.Ext(stem))), "",
			)
			if r := []rune(stem); len(r) > 58 {
				stem = string(r[:58])
			}
			ch <- DlUpdate{Type: EvDest, Slot: slot, Text: stem}

		case dlRE.MatchString(line):
			m := dlRE.FindStringSubmatch(line)
			pct, _ := strconv.ParseFloat(subexp(dlRE, m, "pct"), 64)
			size, _ := strconv.ParseFloat(subexp(dlRE, m, "size"), 64)
			totalB := int64(size * float64(unitToMult(subexp(dlRE, m, "unit"))))
			speed := ""
			if sm := speedRE.FindStringSubmatch(line); sm != nil {
				speed = subexp(speedRE, sm, "speed")
			}
			ch <- DlUpdate{
				Type:   EvProgress,
				Slot:   slot,
				Pct:    pct,
				DoneB:  int64(float64(totalB) * pct / 100),
				TotalB: totalB,
				Speed:  speed,
			}

		case procRE.MatchString(line):
			m := procRE.FindStringSubmatch(line)
			label := loc.MergeProc
			switch lower := strings.ToLower(m[1]); {
			case strings.Contains(lower, "audio"):
				label = loc.MP3Proc
			case strings.Contains(lower, "thumbnail"):
				label = loc.ThumbProc
			}
			ch <- DlUpdate{Type: EvProc, Slot: slot, Text: label}
		}
	}
	if err := sc.Err(); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = waitCommand(cmd, runCtx)
		result.Err = err
		if result.ErrText == "" {
			result.ErrText = commandErrorText(err)
		}
		return result
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		result.Err = err
		if result.ErrText == "" {
			result.ErrText = commandErrorText(err)
		}
		return result
	}
	if runCtx != nil && runCtx.Err() != nil {
		result.Err = runCtx.Err()
		if result.ErrText == "" {
			result.ErrText = commandErrorText(runCtx.Err())
		}
		return result
	}
	return result
}

func parsePrintedOutputPath(line string, result *downloadResult) bool {
	if result == nil || !strings.HasPrefix(line, `"`) {
		return false
	}

	var path string
	if err := json.Unmarshal([]byte(line), &path); err != nil {
		return false
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	result.OutputPath = path
	return true
}

func failedDownload(err error) downloadResult {
	return downloadResult{Err: err, ErrText: commandErrorText(err)}
}

func commandErrorText(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "operation cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "operation timed out"
	default:
		return err.Error()
	}
}
