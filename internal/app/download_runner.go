package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func runDownloadRequest(ctx context.Context, slot int, req DownloadRequest, url, outputTemplate string, extra []string, ch chan<- DlUpdate) downloadResult {
	deps := resolveRuntimeDeps()
	if fragmentDownloadStrategy(req) == fragmentAudio {
		return runAudioFragmentDownload(ctx, slot, req, deps, url, outputTemplate, extra, ch)
	}

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

func runAudioFragmentDownload(
	ctx context.Context,
	slot int,
	req DownloadRequest,
	deps CheckDepsResult,
	url string,
	outputTemplate string,
	extra []string,
	ch chan<- DlUpdate,
) downloadResult {
	tempDir, err := os.MkdirTemp(filepath.Dir(outputTemplate), ".volren-audio-fragment-*")
	if err != nil {
		return failedDownload(err)
	}
	defer os.RemoveAll(tempDir)

	sourceResult := streamYtdlp(
		ctx,
		slot,
		req.Locale,
		deps,
		audioFragmentSourceArgs(deps, url, filepath.Join(tempDir, "%(title)s.%(ext)s"), extra),
		ch,
	)
	if sourceResult.Err != nil {
		return sourceResult
	}

	sourcePath, err := resolveDownloadedMediaPath(tempDir, sourceResult.OutputPath)
	if err != nil {
		return failedDownload(err)
	}

	finalPath, err := audioFragmentOutputPath(outputTemplate, sourcePath, req.Profile)
	if err != nil {
		return failedDownload(err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return failedDownload(err)
	}

	ch <- DlUpdate{Type: EvProc, Slot: slot, Text: StringsFor(req.Locale).MP3Proc}

	trimmedPath := filepath.Join(tempDir, "fragment."+audioOutputExtension(req.Profile))
	if err := trimAudioFragment(ctx, deps, sourcePath, trimmedPath, *req.Fragment, req.Profile); err != nil {
		return failedDownload(err)
	}
	if err := replaceFile(trimmedPath, finalPath); err != nil {
		return failedDownload(err)
	}

	return downloadResult{OutputPath: finalPath}
}

func audioFragmentSourceArgs(deps CheckDepsResult, url, outputTemplate string, extra []string) []string {
	args := make([]string, 0, 12+len(extra))
	args = append(args, ffmpegArgs(deps)...)
	args = append(args,
		"-f", "bestaudio/best",
		"-o", outputTemplate,
		"--windows-filenames",
	)
	args = append(args, extra...)
	args = append(args, url)
	return args
}

func resolveDownloadedMediaPath(dir, printedPath string) (string, error) {
	printedPath = strings.TrimSpace(printedPath)
	if pathExists(printedPath) {
		return printedPath, nil
	}
	if printedPath != "" {
		if directPath := filepath.Join(dir, filepath.Base(printedPath)); pathExists(directPath) {
			return directPath, nil
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry.Name()))
		switch {
		case name == "":
			continue
		case strings.HasSuffix(name, ".part"), strings.HasSuffix(name, ".ytdl"):
			continue
		}
		return filepath.Join(dir, entry.Name()), nil
	}

	return "", errors.New("downloaded media file not found")
}

func audioFragmentOutputPath(outputTemplate, sourcePath string, profile OutputProfile) (string, error) {
	stem := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	stem = strings.TrimSpace(stem)
	if stem == "" {
		return "", errors.New("downloaded media filename is empty")
	}

	return filepath.Join(filepath.Dir(outputTemplate), stem+"."+audioOutputExtension(profile)), nil
}

func trimAudioFragment(ctx context.Context, deps CheckDepsResult, sourcePath, outputPath string, fragment DownloadFragment, profile OutputProfile) error {
	args, err := audioFragmentFFmpegArgs(sourcePath, outputPath, fragment, profile)
	if err != nil {
		return err
	}

	ffmpegBin := strings.TrimSpace(deps.FFmpeg.Path)
	if ffmpegBin == "" {
		return errors.New("ffmpeg is required")
	}

	output, err := commandCombinedOutput(ctx, 0, ffmpegBin, args...)
	if err == nil {
		return nil
	}

	if line := firstNonEmptyLine(string(output)); line != "" {
		return errors.New(line)
	}
	return err
}

func audioFragmentFFmpegArgs(sourcePath, outputPath string, fragment DownloadFragment, profile OutputProfile) ([]string, error) {
	transcodeArgs, err := audioTranscodeArgs(profile)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-i", sourcePath,
		"-ss", FormatClockTimestamp(fragment.StartAt),
		"-map", "0:a:0?",
		"-vn",
		"-sn",
		"-dn",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
	}
	if fragment.EndAt != nil {
		args = append(args, "-t", FormatClockTimestamp(*fragment.EndAt-fragment.StartAt))
	}
	args = append(args, transcodeArgs...)
	args = append(args, outputPath)
	return args, nil
}

func audioTranscodeArgs(profile OutputProfile) ([]string, error) {
	switch ext := audioOutputExtension(profile); ext {
	case "mp3":
		args := []string{"-c:a", "libmp3lame"}
		if bitrate := normalizeAudioBitrate(profile.AudioQuality); bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
		return args, nil
	case "m4a":
		args := []string{"-c:a", "aac", "-movflags", "+faststart"}
		if bitrate := normalizeAudioBitrate(profile.AudioQuality); bitrate != "" {
			args = append(args, "-b:a", bitrate)
		} else {
			args = append(args, "-q:a", "2")
		}
		return args, nil
	case "opus":
		args := []string{"-c:a", "libopus", "-vbr", "on", "-compression_level", "10"}
		if bitrate := normalizeAudioBitrate(profile.AudioQuality); bitrate != "" {
			args = append(args, "-b:a", bitrate)
		} else {
			args = append(args, "-b:a", "160k")
		}
		return args, nil
	case "flac":
		return []string{"-c:a", "flac"}, nil
	default:
		return nil, fmt.Errorf("unsupported audio output format %q", profile.AudioFormat)
	}
}

func normalizeAudioBitrate(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasSuffix(value, "k") {
		return value
	}
	return ""
}

func replaceFile(src, dest string) error {
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(src, dest)
}
