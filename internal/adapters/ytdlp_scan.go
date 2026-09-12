package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"volvid/internal/core"
)

func flatPlaylistScanArgs(target string) []string {
	return []string{
		"--flat-playlist",
		"--dump-json",
		"--quiet",
		"--ignore-errors",
		target,
	}
}

func flatScanError(err error, count int, timeoutErr error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return timeoutErr
	}
	if count > 0 {
		return fmt.Errorf("%w (%d)", err, count)
	}
	return err
}

var (
	// errYtdlpStart marks failures to start the yt-dlp process.
	errYtdlpStart = errors.New("yt-dlp start")
	// errYtdlpOutput marks failures while reading the merged output stream.
	errYtdlpOutput = errors.New("yt-dlp output")
)

// runYtdlpLines starts yt-dlp with merged stderr, feeds every non-empty line
// to handle and returns the process result. Start/read failures are wrapped
// with the sentinels above; the process exit error is returned untouched.
func runYtdlpLines(
	ctx context.Context,
	timeout time.Duration,
	deps core.CheckDepsResult,
	args []string,
	handle func(line []byte) error,
) error {
	cmd, stdout, runCtx, cancel, err := startYTDLPMergedOutputCommand(ctx, timeout, deps, args...)
	if err != nil {
		return fmt.Errorf("%w: %w", errYtdlpStart, err)
	}
	defer cancel()
	defer stdout.Close()

	if err := readCommandLines(stdout, handle); err != nil {
		cancel()
		if waitErr := waitCommand(cmd, runCtx); waitErr != nil {
			err = errors.Join(err, waitErr)
		}
		return fmt.Errorf("%w: %w", errYtdlpOutput, err)
	}
	return waitCommand(cmd, runCtx)
}

func scanYTDLPJSONLines(env *Env, ctx context.Context, timeout time.Duration, args []string, handle func(map[string]any)) error {
	var firstErrorLine string
	err := runYtdlpLines(ctx, timeout, resolveRuntimeDeps(env), args, func(line []byte) error {
		if len(line) == 0 {
			return nil
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			if firstErrorLine == "" {
				firstErrorLine = ytdlpErrorLine(string(line))
			}
			return nil
		}
		handle(entry)
		return nil
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, errYtdlpStart) || errors.Is(err, errYtdlpOutput) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	if firstErrorLine != "" {
		return fmt.Errorf("yt-dlp: %w: %s", err, firstErrorLine)
	}
	return fmt.Errorf("yt-dlp: %w", err)
}

func ytdlpErrorLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if len(line) > maxYtdlpErrorLine {
		line = line[:maxYtdlpErrorLine]
		for len(line) > 0 && !utf8.ValidString(line) {
			line = line[:len(line)-1]
		}
	}
	return line
}

// mediaEntryFromMap parses a flat yt-dlp JSON entry into title/URL/duration.
// Single source of truth for playlist and search results.
func mediaEntryFromMap(entry map[string]any, index int, titleFmt string) (title, url string, duration int, ok bool) {
	url = mediaEntryURL(entry)
	if url == "" {
		return "", "", 0, false
	}
	defaultTitle := fmt.Sprintf(titleFmt, index)
	title = strings.TrimSpace(core.MapString(entry, "title", core.MapString(entry, "id", defaultTitle)))
	if title == "" {
		title = defaultTitle
	}
	return title, url, int(core.MapFloat(entry, "duration")), true
}
