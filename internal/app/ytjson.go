package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func scanYTDLPJSONLines(ctx context.Context, timeout time.Duration, args []string, handle func(map[string]any)) error {
	cmd, stdout, runCtx, cancel, err := startYTDLPMergedOutputCommand(ctx, timeout, args...)
	if err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}
	defer cancel()
	defer stdout.Close()

	if err := readCommandLines(stdout, func(line []byte) error {
		if len(line) == 0 {
			return nil
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil
		}
		if handle != nil {
			handle(entry)
		}
		return nil
	}); err != nil {
		cancel()
		_ = waitCommand(cmd, runCtx)
		return fmt.Errorf("yt-dlp output: %w", err)
	}

	if err := waitCommand(cmd, runCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("yt-dlp: %w", err)
	}
	return nil
}
