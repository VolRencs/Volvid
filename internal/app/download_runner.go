package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func runDownloadRequest(ctx context.Context, slot int, req DownloadRequest, url, tmpl string, extra []string, ch chan<- DlUpdate) bool {
	req = NormalizeDownloadRequest(req)
	strs := StringsFor(req.Locale)

	formats := []string{""}
	labels := []string{""}
	if req.Profile.Mode == ModeVideo {
		formats = req.Profile.VideoFmtChain
		labels = req.Profile.VideoFmtLabels
		if len(formats) == 0 {
			formats = []string{"bestvideo+bestaudio/best"}
		}
	}

	for i, format := range formats {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
		if req.Profile.Mode == ModeVideo && i > 0 {
			label := format
			if i < len(labels) && labels[i] != "" {
				label = labels[i]
			}
			ch <- DlUpdate{
				Type: EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(strs.FallbackFmt, i, label),
			}
		}

		spec, err := BuildCommandSpec(req, url, tmpl, format, extra)
		if err != nil {
			return false
		}
		if streamYtdlp(ctx, slot, req.Locale, spec.Args, ch) {
			return true
		}
	}
	return false
}

func normalizeWorkerCount(workers, jobs int) int {
	if jobs <= 0 {
		return 1
	}
	workers = max(workers, 1)
	return min(workers, jobs)
}

func StartDownloadRequest(req DownloadRequest, ch chan<- DlUpdate) {
	StartDownloadRequestContext(context.Background(), req, ch)
}

func StartDownloadRequestContext(ctx context.Context, req DownloadRequest, ch chan<- DlUpdate) {
	go func() {
		var wg sync.WaitGroup
		defer func() { wg.Wait(); close(ch) }()
		if ctx == nil {
			ctx = context.Background()
		}

		preparedReq, err := PrepareDownloadRequest(req)
		if err != nil {
			ch <- DlUpdate{Type: EvDone, OK: false}
			return
		}
		req = preparedReq

		if !requestUsesPlaylist(req) {
			ok := runDownloadRequest(ctx, 0, req, req.Target.DownloadURL(req.ForceSingle),
				filepath.Join(req.OutputDir, "%(title)s.%(ext)s"),
				[]string{"--no-playlist"}, ch,
			)
			ch <- DlUpdate{Type: EvDone, OK: ok}
			return
		}

		plDir := filepath.Join(req.OutputDir, SanitizeDirname(req.PlaylistInfo.Title))
		if err := os.MkdirAll(plDir, 0o755); err != nil {
			plDir = filepath.Join(req.OutputDir, "playlist")
			_ = os.MkdirAll(plDir, 0o755)
		}

		entries := requestEntries(req)
		workerCount := normalizeWorkerCount(req.Workers, len(entries))
		jobs := make(chan PlaylistEntry)
		go func() {
			defer close(jobs)
			for _, e := range entries {
				select {
				case <-ctx.Done():
					return
				case jobs <- e:
				}
			}
		}()

		for slot := 0; slot < workerCount; slot++ {
			wg.Add(1)
			go func(slot int) {
				defer wg.Done()
				for e := range jobs {
					if ctx.Err() != nil {
						return
					}
					func() {
						defer func() {
							time.Sleep(slotResetDelay)
							ch <- DlUpdate{Type: EvReset, Slot: slot}
						}()
						ch <- DlUpdate{Type: EvStart, Slot: slot, Text: e.Title}
						tmpl := filepath.Join(plDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", e.Index))
						ok := runDownloadRequest(ctx, slot, req, e.URL, tmpl, []string{"--no-playlist"}, ch)
						ch <- DlUpdate{Type: EvDone, Slot: slot, OK: ok}
					}()
				}
			}(slot)
		}
	}()
}
