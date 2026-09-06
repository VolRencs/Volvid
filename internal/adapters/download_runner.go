package adapters

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

func runDownloadRequest(env *Env, ctx context.Context, slot int, req core.DownloadRequest, deps core.CheckDepsResult, url, outputTemplate string, extra []string, ch chan<- core.DlUpdate, cleanup *downloadCleanup) downloadResult {
	strs := i18n.StringsFor(req.Locale)
	formats, labels := downloadFormats(req)
	result := failedDownload(errors.New("download failed"))

	for i, format := range formats {
		if ctx != nil && ctx.Err() != nil {
			return canceledDownload(ctx)
		}
		if req.Profile.Mode == core.ModeVideo && i > 0 {
			if !sendUpdate(ctx, ch, core.DlUpdate{
				Type: core.EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(strs.FallbackFmt, i, formatLabel(format, labels, i)),
			}) {
				return canceledDownload(ctx)
			}
		}

		args, err := buildDownloadCommandArgs(req, deps, url, outputTemplate, format, extra)
		if err != nil {
			return failedDownload(err)
		}
		result = streamYtdlp(env, ctx, slot, req.Locale, deps, args, ch, cleanup)
		if result.Err == nil {
			finalPath, err := transcodeDownloadedVideo(env, ctx, slot, req.Profile, req.Locale, deps, result.OutputPath, ch)
			if err != nil {
				if cleanup != nil && result.OutputPath != "" {
					cleanup.add(result.OutputPath)
				}
				return failedDownload(err)
			}
			if finalPath != "" {
				// Транскодинг мог сменить расширение (контейнер):
				// result.OutputPath — оригинал, finalPath — готовый файл.
				if finalPath != result.OutputPath {
					result.OutputPath = finalPath
				}
			}
			if cleanup != nil && result.OutputPath != "" {
				// Успешный файл нельзя удалять в deferred cleanup.
				cleanup.forget(result.OutputPath)
			}
			return result
		}
	}

	return result
}

const (
	defaultHardwareCRF = "23"
	nvencPreset        = "p5"
)

func downloadFormats(req core.DownloadRequest) ([]string, []string) {
	if req.Profile.Mode != core.ModeVideo {
		return []string{""}, []string{""}
	}

	formats := req.Profile.VideoFmtChain
	if len(formats) == 0 {
		formats = []string{core.YtdlpBestFormat}
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
	workers = max(workers, 1)
	return min(workers, jobs)
}
func StartDownloadRequestContext(env *Env, ctx context.Context, req core.DownloadRequest, deps core.CheckDepsResult, ch chan<- core.DlUpdate) {
	go func() {
		var wg sync.WaitGroup
		cleanup := newDownloadCleanup()
		defer func() {
			wg.Wait()
			cleanup.cleanup()
			close(ch)
		}()
		if ctx == nil {
			ctx = context.Background()
		}

		preparedReq, err := PrepareDownloadRequestWithDeps(env, req, deps)
		if err != nil {
			sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvDone, OK: false, ErrText: err.Error()})
			return
		}
		req = preparedReq
		req.OutputDir, err = prepareDir(req.OutputDir)
		if err != nil {
			sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvDone, OK: false, ErrText: err.Error()})
			return
		}
		cleanup.setRoot(req.OutputDir)

		if !downloadRequestUsesPlaylist(req) {
			result := runSingleDownload(env, ctx, req, deps, ch, cleanup)
			sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvDone, OK: result.Err == nil, ErrText: result.ErrText})
			return
		}

		runPlaylistDownloads(env, ctx, req, deps, append([]core.PlaylistEntry(nil), req.Entries...), ch, &wg, cleanup)
	}()
}
func runSingleDownload(env *Env, ctx context.Context, req core.DownloadRequest, deps core.CheckDepsResult, ch chan<- core.DlUpdate, cleanup *downloadCleanup) downloadResult {
	sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvStart, Slot: 0, Text: i18n.StringsFor(req.Locale).Downloading})
	return runDownloadRequest(
		env,
		ctx,
		0,
		req,
		deps,
		req.Target.DownloadURL(req.ForceSingle),
		filepath.Join(req.OutputDir, "%(title)s.%(ext)s"),
		[]string{"--no-playlist"},
		ch,
		cleanup,
	)
}
func runPlaylistDownloads(env *Env, ctx context.Context, req core.DownloadRequest, deps core.CheckDepsResult, entries []core.PlaylistEntry, ch chan<- core.DlUpdate, wg *sync.WaitGroup, cleanup *downloadCleanup) {
	if len(entries) == 0 {
		return
	}

	workerCount := normalizeWorkerCount(req.Workers, len(entries))
	jobs := enqueuePlaylistJobs(ctx, entries)
	outputDir := playlistOutputDir(req)
	for slot := range workerCount {
		wg.Add(1)
		go playlistWorker(env, ctx, slot, req, deps, outputDir, jobs, ch, wg, cleanup)
	}
}
func enqueuePlaylistJobs(ctx context.Context, entries []core.PlaylistEntry) <-chan core.PlaylistEntry {
	jobs := make(chan core.PlaylistEntry)
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
	env *Env,
	ctx context.Context,
	slot int,
	req core.DownloadRequest,
	deps core.CheckDepsResult,
	outputDir string,
	jobs <-chan core.PlaylistEntry,
	ch chan<- core.DlUpdate,
	wg *sync.WaitGroup,
	cleanup *downloadCleanup,
) {
	defer wg.Done()
	for entry := range jobs {
		if ctx.Err() != nil {
			return
		}
		runPlaylistEntry(env, ctx, slot, req, deps, outputDir, entry, ch, cleanup)
	}
}
func runPlaylistEntry(env *Env, ctx context.Context, slot int, req core.DownloadRequest, deps core.CheckDepsResult, outputDir string, entry core.PlaylistEntry, ch chan<- core.DlUpdate, cleanup *downloadCleanup) {
	defer resetDownloadSlot(ctx, slot, ch)
	if !sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvStart, Slot: slot, Text: entry.Title}) {
		return
	}
	result := runDownloadRequest(env, ctx, slot, req, deps, entry.URL, playlistOutputTemplate(outputDir, entry), []string{"--no-playlist"}, ch, cleanup)
	sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvDone, Slot: slot, OK: result.Err == nil, ErrText: result.ErrText})
}
func resetDownloadSlot(ctx context.Context, slot int, ch chan<- core.DlUpdate) {
	timer := time.NewTimer(slotResetDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	select {
	case ch <- core.DlUpdate{Type: core.EvReset, Slot: slot}:
	case <-ctx.Done():
	default:
	}
}
func playlistOutputDir(req core.DownloadRequest) string {
	title := "playlist"
	if req.PlaylistInfo != nil {
		title = req.PlaylistInfo.Title
	}
	dir := filepath.Join(req.OutputDir, core.SanitizeDirname(title))
	if err := os.MkdirAll(dir, 0o755); err == nil {
		return dir
	}

	dir = filepath.Join(req.OutputDir, "playlist")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
func playlistOutputTemplate(outputDir string, entry core.PlaylistEntry) string {
	return filepath.Join(outputDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", entry.Index))
}
