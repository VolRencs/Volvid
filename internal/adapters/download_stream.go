package adapters

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

const (
	ytdlpLineStart    = "VRDL_START"
	ytdlpLineProgress = "VRDL_PROGRESS"
	ytdlpLinePost     = "VRDL_POST"
	ytdlpLineMoved    = "VRDL_MOVED"
)

type downloadResult struct {
	OutputPath string
	ErrText    string
	Err        error
}

func ffmpegBinFor(env *Env, deps core.CheckDepsResult) string {
	if bin := strings.TrimSpace(deps.FFmpeg.Path); bin != "" {
		return bin
	}
	return strings.TrimSpace(env.FFmpegBin)
}
func streamYtdlp(ctx context.Context, slot int, l core.Locale, deps core.CheckDepsResult, args []string, ch chan<- core.DlUpdate, cleanup *downloadCleanup) downloadResult {
	result := downloadResult{}
	lastTitle := ""

	err := runYtdlpLines(ctx, 0, deps, slices.Concat(streamProtocolArgs(), args), func(raw []byte) error {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			return nil
		}

		if parseMovedOutputPath(line, &result) {
			cleanup.add(result.OutputPath)
			return nil
		}
		if strings.HasPrefix(strings.ToLower(line), "error:") {
			setDownloadErrorText(&result, strings.TrimSpace(line[6:]))
			return nil
		}

		switch {
		case strings.HasPrefix(line, ytdlpLineStart):
			title, filename := parseBeforeDownload(line)
			if title != "" {
				lastTitle = title
				if !sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvStart, Slot: slot, Text: title}) {
					return interruptErr(ctx)
				}
			}
			cleanup.add(filename)

		case strings.HasPrefix(line, ytdlpLineProgress):
			update, title, ok := parseProgressUpdate(line, slot, l)
			if !ok {
				return nil
			}
			if title != "" && title != lastTitle {
				lastTitle = title
				if !sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvDest, Slot: slot, Text: title}) {
					return interruptErr(ctx)
				}
			}
			if !sendUpdate(ctx, ch, update) {
				return interruptErr(ctx)
			}

		case strings.HasPrefix(line, ytdlpLinePost):
			label := postprocessLabel(line, l)
			if label != "" {
				if !sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvProc, Slot: slot, Text: label}) {
					return interruptErr(ctx)
				}
			}
		}
		return nil
	})
	if err != nil {
		setDownloadError(&result, err, l)
	}
	return result
}
func streamProtocolArgs() []string {
	return []string{
		"--newline",
		"--progress",
		"--print", "before_dl:" + ytdlpLineStart + "%(title|)j\t%(_filename|)j",
		"--print", "after_move:" + ytdlpLineMoved + "%(filepath)j",
		"--progress-template", "download:" + ytdlpLineProgress + "%(progress.downloaded_bytes|0)s\t%(progress.total_bytes|0)s\t%(progress.total_bytes_estimate|0)s\t%(progress.speed|0)s\t%(progress._percent_str|0)s\t%(info.title|)j",
		"--progress-template", "postprocess:" + ytdlpLinePost + "%(progress.postprocessor|)s",
	}
}
func parseMovedOutputPath(line string, result *downloadResult) bool {
	if result == nil {
		return false
	}
	rest, ok := strings.CutPrefix(line, ytdlpLineMoved)
	if !ok {
		return false
	}
	result.OutputPath = parseJSONStringField(rest)
	return strings.TrimSpace(result.OutputPath) != ""
}
func parseBeforeDownload(line string) (string, string) {
	payload := strings.TrimPrefix(line, ytdlpLineStart)
	parts := strings.SplitN(payload, "\t", 2)
	title := parseJSONStringField(parts[0])
	if len(parts) < 2 {
		return title, ""
	}
	return title, parseJSONStringField(parts[1])
}
func parseProgressUpdate(line string, slot int, l core.Locale) (core.DlUpdate, string, bool) {
	payload := strings.TrimPrefix(line, ytdlpLineProgress)
	parts := strings.SplitN(payload, "\t", 6)
	if len(parts) != 6 {
		return core.DlUpdate{}, "", false
	}

	doneB := core.ParseIntOrZero(parts[0])
	totalB := core.ParseIntOrZero(parts[1])
	if totalB <= 0 {
		totalB = core.ParseIntOrZero(parts[2])
	}
	speed := formatProgressSpeed(parts[3], l)
	pct := core.ParsePercentOrZero(parts[4])
	if pct <= 0 && totalB > 0 && doneB > 0 {
		pct = float64(doneB) / float64(totalB) * 100
	}
	pct = clampProgressPercent(pct)
	title := parseJSONStringField(parts[5])

	return core.DlUpdate{
		Type:   core.EvProgress,
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
	// yt-dlp %(..)j emits a JSON-quoted string: Unquote avoids a full
	// json.Unmarshal per progress line.
	if value, err := strconv.Unquote(raw); err == nil {
		return strings.TrimSpace(value)
	}
	return raw
}
func formatProgressSpeed(raw string, l core.Locale) string {
	value := int64(core.ParsePercentOrZero(raw))
	if value <= 0 {
		return ""
	}
	return i18n.FormatSpeed(value, l)
}

func postprocessLabel(line string, l core.Locale) string {
	name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, ytdlpLinePost)))
	loc := i18n.StringsFor(l)
	switch {
	case strings.Contains(name, "audio"):
		return loc.MP3Proc
	case strings.Contains(name, "thumbnail"):
		return loc.ThumbProc
	default:
		return loc.MergeProc
	}
}
func failedDownload(err error, l core.Locale) downloadResult {
	return downloadResult{Err: err, ErrText: commandErrorText(err, l)}
}
func interruptErr(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return context.Canceled
}
func canceledDownload(ctx context.Context, l core.Locale) downloadResult {
	return failedDownload(interruptErr(ctx), l)
}
func setDownloadError(result *downloadResult, err error, l core.Locale) {
	if result == nil || err == nil {
		return
	}
	result.Err = err
	setDownloadErrorText(result, commandErrorText(err, l))
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
	return min(100, max(0, pct))
}
func commandErrorText(err error, l core.Locale) string {
	if err == nil {
		return ""
	}

	loc := i18n.StringsFor(l)
	switch {
	case errors.Is(err, context.Canceled):
		return loc.ErrCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return loc.ErrTimedOut
	default:
		return err.Error()
	}
}
