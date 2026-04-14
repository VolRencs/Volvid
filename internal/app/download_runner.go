package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func runDownloadRequest(ctx context.Context, slot int, req DownloadRequest, url, outputTemplate string, extra []string, ch chan<- DlUpdate) downloadResult {
	deps := resolveRuntimeDeps()

	strs := StringsFor(req.Locale)
	formats, labels := downloadFormats(req)
	result := failedDownload(errors.New("download failed"))

	for i, format := range formats {
		if ctx != nil && ctx.Err() != nil {
			return failedDownload(ctx.Err())
		}
		if req.Profile.Mode == ModeVideo && i > 0 {
			ch <- DlUpdate{
				Type: EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(strs.FallbackFmt, i, formatLabel(format, labels, i)),
			}
		}

		args, err := buildDownloadCommandArgs(req, deps, url, outputTemplate, format, extra)
		if err != nil {
			return failedDownload(err)
		}
		result = streamYtdlp(ctx, slot, req.Locale, deps, args, ch)
		if result.Err == nil {
			return result
		}
	}

	return result
}

func downloadFormats(req DownloadRequest) ([]string, []string) {
	if req.Profile.Mode != ModeVideo {
		return []string{""}, []string{""}
	}

	formats := req.Profile.VideoFmtChain
	if len(formats) == 0 {
		formats = []string{"bestvideo+bestaudio/best"}
	}
	return formats, req.Profile.VideoFmtLabels
}

func formatLabel(format string, labels []string, idx int) string {
	if idx >= 0 && idx < len(labels) && labels[idx] != "" {
		return labels[idx]
	}
	return format
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
			ch <- DlUpdate{Type: EvDone, OK: false, ErrText: err.Error()}
			return
		}
		req = preparedReq
		req.OutputDir, err = prepareDir(req.OutputDir)
		if err != nil {
			ch <- DlUpdate{Type: EvDone, OK: false, ErrText: err.Error()}
			return
		}

		if !downloadRequestUsesPlaylist(req) {
			result := runSingleDownload(ctx, req, ch)
			ch <- DlUpdate{Type: EvDone, OK: result.Err == nil, ErrText: result.ErrText}
			return
		}

		entries := downloadRequestEntries(req)
		runPlaylistDownloads(ctx, req, entries, ch, &wg)
	}()
}

func runSingleDownload(ctx context.Context, req DownloadRequest, ch chan<- DlUpdate) downloadResult {
	ch <- DlUpdate{Type: EvStart, Slot: 0, Text: StringsFor(req.Locale).Downloading}
	return runDownloadRequest(
		ctx,
		0,
		req,
		req.Target.DownloadURL(req.ForceSingle),
		filepath.Join(req.OutputDir, "%(title)s.%(ext)s"),
		[]string{"--no-playlist"},
		ch,
	)
}

func runPlaylistDownloads(ctx context.Context, req DownloadRequest, entries []PlaylistEntry, ch chan<- DlUpdate, wg *sync.WaitGroup) {
	if len(entries) == 0 {
		return
	}

	workerCount := normalizeWorkerCount(req.Workers, len(entries))
	jobs := enqueuePlaylistJobs(ctx, entries)
	outputDir := playlistOutputDir(req)
	for slot := 0; slot < workerCount; slot++ {
		wg.Add(1)
		go playlistWorker(ctx, slot, req, outputDir, jobs, ch, wg)
	}
}

func enqueuePlaylistJobs(ctx context.Context, entries []PlaylistEntry) <-chan PlaylistEntry {
	jobs := make(chan PlaylistEntry)
	go func() {
		defer close(jobs)
		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return
			case jobs <- entry:
			}
		}
	}()
	return jobs
}

func playlistWorker(
	ctx context.Context,
	slot int,
	req DownloadRequest,
	outputDir string,
	jobs <-chan PlaylistEntry,
	ch chan<- DlUpdate,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for entry := range jobs {
		if ctx.Err() != nil {
			return
		}
		runPlaylistEntry(ctx, slot, req, outputDir, entry, ch)
	}
}

func runPlaylistEntry(ctx context.Context, slot int, req DownloadRequest, outputDir string, entry PlaylistEntry, ch chan<- DlUpdate) {
	defer resetDownloadSlot(slot, ch)
	ch <- DlUpdate{Type: EvStart, Slot: slot, Text: entry.Title}
	result := runDownloadRequest(ctx, slot, req, entry.URL, playlistOutputTemplate(outputDir, entry), []string{"--no-playlist"}, ch)
	ch <- DlUpdate{Type: EvDone, Slot: slot, OK: result.Err == nil, ErrText: result.ErrText}
}

func resetDownloadSlot(slot int, ch chan<- DlUpdate) {
	time.Sleep(slotResetDelay)
	ch <- DlUpdate{Type: EvReset, Slot: slot}
}

func playlistOutputDir(req DownloadRequest) string {
	dir := filepath.Join(req.OutputDir, SanitizeDirname(req.PlaylistInfo.Title))
	if err := os.MkdirAll(dir, 0o755); err == nil {
		return dir
	}

	dir = filepath.Join(req.OutputDir, "playlist")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func playlistOutputTemplate(outputDir string, entry PlaylistEntry) string {
	return filepath.Join(outputDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", entry.Index))
}
