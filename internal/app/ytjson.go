package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func scanYTDLPJSONLines(ctx context.Context, timeout time.Duration, args []string, handle func(map[string]any)) error {
	cmd, stdout, runCtx, cancel, err := startMergedOutputCommand(ctx, timeout, YtdlpBin, args...)
	if err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}
	defer cancel()
	defer stdout.Close()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)

	for sc.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			continue
		}
		if handle != nil {
			handle(entry)
		}
	}

	if err := sc.Err(); err != nil {
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
