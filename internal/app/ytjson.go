package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const maxYtdlpErrorLine = 512

// flatPlaylistScanArgs builds the yt-dlp arguments shared by playlist
// fetching and search: one JSON object per line on stdout.
func flatPlaylistScanArgs(target string) []string {
	return []string{
		"--flat-playlist",
		"--dump-json",
		"--quiet",
		"--ignore-errors",
		target,
	}
}

// flatScanError maps a scan failure to a user-facing error, keeping partial
// results when at least one entry was collected.
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

func scanYTDLPJSONLines(env *Env, ctx context.Context, timeout time.Duration, args []string, handle func(map[string]any)) error {
	cmd, stdout, runCtx, cancel, err := startYTDLPMergedOutputCommand(env, ctx, timeout, resolveRuntimeDeps(env), args...)
	if err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}
	defer cancel()
	defer stdout.Close()

	var firstErrorLine string
	if err := readCommandLines(stdout, func(line []byte) error {
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
	}); err != nil {
		cancel()
		if waitErr := waitCommand(cmd, runCtx); waitErr != nil {
			err = errors.Join(err, waitErr)
		}
		return fmt.Errorf("yt-dlp output: %w", err)
	}

	if err := waitCommand(cmd, runCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		if firstErrorLine != "" {
			return fmt.Errorf("yt-dlp: %w: %s", err, firstErrorLine)
		}
		return fmt.Errorf("yt-dlp: %w", err)
	}
	return nil
}

func ytdlpErrorLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	// Truncate by bytes but never split a UTF-8 sequence (at most the
	// trailing 3 bytes of a 4-byte rune are trimmed).
	if len(line) > maxYtdlpErrorLine {
		line = line[:maxYtdlpErrorLine]
		for len(line) > 0 && !utf8.ValidString(line) {
			line = line[:len(line)-1]
		}
	}
	return line
}
