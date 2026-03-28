package app

import (
	"bufio"
	"context"
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
	Type   DlEventType
	Slot   int
	Text   string
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	OK     bool
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

func streamYtdlp(ctx context.Context, slot int, l Locale, args []string, ch chan<- DlUpdate) bool {
	cmd, pr, runCtx, cancel, err := startMergedOutputCommand(
		ctx,
		0,
		YtdlpBin,
		slices.Concat([]string{"--newline", "--no-warnings"}, args)...,
	)
	if err != nil {
		return false
	}
	defer cancel()
	defer pr.Close()

	loc := StringsFor(l)

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
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
		return false
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		return false
	}
	if runCtx != nil && runCtx.Err() != nil {
		return false
	}
	return true
}
