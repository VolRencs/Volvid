package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

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
	if len(line) > 512 {
		line = line[:512]
	}
	return line
}
