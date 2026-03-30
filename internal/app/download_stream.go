package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
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

func ffmpegArgs(deps CheckDepsResult) []string {
	bin := strings.TrimSpace(deps.FFmpeg.Path)
	if bin == "" {
		return nil
	}
	return []string{"--ffmpeg-location", bin}
}

func streamYtdlp(ctx context.Context, slot int, l Locale, deps CheckDepsResult, args []string, ch chan<- DlUpdate) downloadResult {
	cmd, pr, runCtx, cancel, err := startYTDLPMergedOutputCommandFor(
		ctx,
		0,
		deps,
		slices.Concat(streamProtocolArgs(), []string{"--no-warnings"}, args)...,
	)
	if err != nil {
		return failedDownload(err)
	}
	defer cancel()
	defer pr.Close()

	result := downloadResult{}
	lastTitle := ""

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if parseMovedOutputPath(line, &result) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "error:") {
			setDownloadErrorText(&result, strings.TrimSpace(line[6:]))
			continue
		}

		switch {
		case strings.HasPrefix(line, ytdlpLineStart):
			title := parseJSONStringWithPrefix(line, ytdlpLineStart)
			if title != "" {
				lastTitle = title
				ch <- DlUpdate{Type: EvStart, Slot: slot, Text: title}
			}

		case strings.HasPrefix(line, ytdlpLineProgress):
			update, title, ok := parseProgressUpdate(line, slot, l)
			if !ok {
				continue
			}
			if title != "" && title != lastTitle {
				lastTitle = title
				ch <- DlUpdate{Type: EvDest, Slot: slot, Text: title}
			}
			ch <- update

		case strings.HasPrefix(line, ytdlpLinePost):
			label := postprocessLabel(line, l)
			if label != "" {
				ch <- DlUpdate{Type: EvProc, Slot: slot, Text: label}
			}
		}
	}

	if err := sc.Err(); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = waitCommand(cmd, runCtx)
		setDownloadError(&result, err)
		return result
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		setDownloadError(&result, err)
		return result
	}
	if runCtx != nil && runCtx.Err() != nil {
		setDownloadError(&result, runCtx.Err())
		return result
	}
	return result
}

func streamProtocolArgs() []string {
	return []string{
		"--newline",
		"--progress",
		"--print", "before_dl:" + ytdlpLineStart + "%(title|)j",
		"--print", "after_move:" + ytdlpLineMoved + "%(filepath)j",
		"--progress-template", "download:" + ytdlpLineProgress + "%(progress.downloaded_bytes|0)s\t%(progress.total_bytes|0)s\t%(progress.total_bytes_estimate|0)s\t%(progress.speed|0)s\t%(progress._percent_str|0)s\t%(info.title|)j",
		"--progress-template", "postprocess:" + ytdlpLinePost + "%(progress.postprocessor|)s",
	}
}

func parseMovedOutputPath(line string, result *downloadResult) bool {
	if result == nil || !strings.HasPrefix(line, ytdlpLineMoved) {
		return false
	}
	result.OutputPath = parseJSONStringWithPrefix(line, ytdlpLineMoved)
	return strings.TrimSpace(result.OutputPath) != ""
}

func parseJSONStringWithPrefix(line, prefix string) string {
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if raw == "" {
		return ""
	}

	var value string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func parseProgressUpdate(line string, slot int, l Locale) (DlUpdate, string, bool) {
	payload := strings.TrimPrefix(line, ytdlpLineProgress)
	parts := strings.SplitN(payload, "\t", 6)
	if len(parts) != 6 {
		return DlUpdate{}, "", false
	}

	doneB := parseDownloadInt(parts[0])
	totalB := parseDownloadInt(parts[1])
	if totalB <= 0 {
		totalB = parseDownloadInt(parts[2])
	}
	speed := formatProgressSpeed(parts[3], l)
	pct := parseDownloadFloat(parts[4])
	if pct <= 0 && totalB > 0 && doneB > 0 {
		pct = float64(doneB) / float64(totalB) * 100
	}
	pct = clampProgressPercent(pct)
	title := parseJSONStringField(parts[5])

	return DlUpdate{
		Type:   EvProgress,
		Slot:   slot,
		Pct:    pct,
		DoneB:  doneB,
		TotalB: totalB,
		Speed:  speed,
	}, title, true
}

func parseJSONStringField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var value string
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func formatProgressSpeed(raw string, l Locale) string {
	value := int64(parseDownloadFloat(raw))
	if value <= 0 {
		return ""
	}
	suffix := "/s"
	if l == LocaleRU {
		suffix = "/с"
	}
	return FmtBytesFor(value, l) + suffix
}

func postprocessLabel(line string, l Locale) string {
	name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, ytdlpLinePost)))
	loc := StringsFor(l)
	switch {
	case strings.Contains(name, "audio"):
		return loc.MP3Proc
	case strings.Contains(name, "thumbnail"):
		return loc.ThumbProc
	default:
		return loc.MergeProc
	}
}

func failedDownload(err error) downloadResult {
	return downloadResult{Err: err, ErrText: commandErrorText(err)}
}

func setDownloadError(result *downloadResult, err error) {
	if result == nil || err == nil {
		return
	}
	result.Err = err
	setDownloadErrorText(result, commandErrorText(err))
}

func setDownloadErrorText(result *downloadResult, text string) {
	if result == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" || result.ErrText != "" {
		return
	}
	result.ErrText = text
}

func clampProgressPercent(pct float64) float64 {
	switch {
	case pct < 0:
		return 0
	case pct > 100:
		return 100
	default:
		return pct
	}
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
