package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func sendUpdate(ctx context.Context, ch chan<- DlUpdate, u DlUpdate) bool {
	if ch == nil {
		return false
	}
	if ctx == nil {
		select {
		case ch <- u:
			return true
		default:
			return false
		}
	}
	select {
	case ch <- u:
		return true
	case <-ctx.Done():
		return false
	}
}

type downloadCleanup struct {
	mu    sync.Mutex
	root  string
	paths map[string]struct{}
}

func newDownloadCleanup() *downloadCleanup {
	return &downloadCleanup{paths: map[string]struct{}{}}
}

func (c *downloadCleanup) setRoot(root string) {
	if c == nil {
		return
	}
	root = cleanAbsPath(root)
	if root == "" {
		return
	}
	c.mu.Lock()
	c.root = root
	c.mu.Unlock()
}

func (c *downloadCleanup) add(path string) {
	if c == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != "" {
		rel, err := filepath.Rel(c.root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}
	}
	c.paths[abs] = struct{}{}
}

func (c *downloadCleanup) forget(path string) {
	if c == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.paths, abs)
}

func (c *downloadCleanup) cleanup() {
	if c == nil {
		return
	}
	c.mu.Lock()
	paths := make([]string, 0, len(c.paths))
	for p := range c.paths {
		paths = append(paths, p)
	}
	c.mu.Unlock()
	byDir := make(map[string][]string)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dir := filepath.Dir(p)
		byDir[dir] = append(byDir[dir], p)
	}
	for dir, ps := range byDir {
		for _, p := range ps {
			deleteDownloadArtifacts(p)
		}
		removeMatchingArtifacts(dir, ps)
	}
}

func deleteDownloadArtifacts(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + ".part")
	_ = os.Remove(path + ".ytdl")
}

func removeMatchingArtifacts(dir string, paths []string) {
	bases := make([]string, 0, len(paths))
	for _, p := range paths {
		if base := filepath.Base(p); base != "" {
			bases = append(bases, base)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, base := range bases {
			// Только точные артефакты: yt-dlp --part фрагменты,
			// наши tmp транскода и backup замены. Без stem-эвристик,
			// чтобы не сносить пользовательские video.particular.mp4.
			if strings.HasPrefix(name, base+".part-") ||
				strings.HasPrefix(name, "."+base+".transcode-") ||
				strings.HasPrefix(name, "."+base+".bak-") {
				_ = os.Remove(filepath.Join(dir, name))
				break
			}
		}
	}
}

func runDownloadRequest(env *Env, ctx context.Context, slot int, req DownloadRequest, deps CheckDepsResult, url, outputTemplate string, extra []string, ch chan<- DlUpdate, cleanup *downloadCleanup) downloadResult {
	strs := StringsFor(req.Locale)
	formats, labels := downloadFormats(req)
	result := failedDownload(errors.New("download failed"))

	for i, format := range formats {
		if ctx != nil && ctx.Err() != nil {
			return canceledDownload(ctx)
		}
		if req.Profile.Mode == ModeVideo && i > 0 {
			if !sendUpdate(ctx, ch, DlUpdate{
				Type: EvFallback,
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

func transcodeDownloadedVideo(
	env *Env,
	ctx context.Context,
	slot int,
	profile OutputProfile,
	l Locale,
	deps CheckDepsResult,
	outputPath string,
	ch chan<- DlUpdate,
) (string, error) {
	if !profile.NeedsVideoTranscode() {
		return outputPath, nil
	}

	ffmpeg := ffmpegBinFor(env, deps)
	if strings.TrimSpace(outputPath) == "" {
		return "", errors.New("video transcoding failed: downloaded file path is unknown")
	}

	if ch != nil {
		sendUpdate(ctx, ch, DlUpdate{Type: EvProc, Slot: slot, Text: StringsFor(l).VideoConvertProc})
	}

	finalPath := transcodeFinalPath(outputPath, profile.VideoContainer)
	commands := ffmpegVideoTranscodeCommands(env, ctx, ffmpeg, outputPath, profile)
	var lastErr error
	var lastOut []byte
	for _, command := range commands {
		tmp, err := transcodeTempPath(outputPath, profile.VideoContainer)
		if err != nil {
			return "", err
		}
		args := injectOutputPath(command, tmp)
		out, err := commandCombinedOutput(ctx, 0, ffmpeg, args...)
		if err == nil {
			if err := os.Chmod(tmp, 0o644); err != nil {
				_ = os.Remove(tmp)
				return "", fmt.Errorf("video transcoding failed: %w", err)
			}
			if err := replaceDownloadedFile(tmp, finalPath); err != nil {
				_ = os.Remove(tmp)
				return "", fmt.Errorf("video transcoding failed: %w", err)
			}
			// Сценарий «удалить оригинал после конвертации»:
			// если контейнер сменил расширение, готовый файл — finalPath,
			// а исходник нужно удалить сразу, не дожидаясь deferred cleanup.
			if finalPath != outputPath {
				if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					// Не удалось удалить сразу — deferred cleanup добьёт,
					// но успешный finalPath уже не трогаем.
					_ = err
				}
			}
			return finalPath, nil
		}

		_ = os.Remove(tmp)
		lastErr = err
		lastOut = out
		if ctx != nil && ctx.Err() != nil {
			break
		}
	}

	text := strings.TrimSpace(string(lastOut))
	if text == "" {
		text = commandErrorText(lastErr)
	}
	if lastErr != nil {
		return "", fmt.Errorf("video transcoding failed: %s: %w", text, lastErr)
	}
	return "", fmt.Errorf("video transcoding failed: %s", text)
}

func transcodeFinalPath(outputPath, container string) string {
	container = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(container), "."))
	if container == "" {
		return outputPath
	}
	ext := strings.TrimPrefix(filepath.Ext(outputPath), ".")
	if strings.EqualFold(ext, container) || ext == "" {
		if ext == "" {
			return outputPath + "." + container
		}
		return outputPath
	}
	return strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "." + container
}

func injectOutputPath(args []string, output string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, args...)
	return append(out, output)
}

func transcodeTempPath(outputPath, container string) (string, error) {
	dir := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	ext := strings.TrimSpace(container)
	if ext == "" {
		ext, _ = strings.CutPrefix(filepath.Ext(base), ".")
	}
	if ext == "" {
		ext = "mp4"
	}
	tmp, err := os.CreateTemp(dir, "."+base+".transcode-*."+ext)
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

type hardwareVideoEncoder struct {
	Codec  string
	Family string
}

func ffmpegVideoTranscodeCommands(env *Env, ctx context.Context, ffmpeg, inputPath string, profile OutputProfile) [][]string {
	commands := make([][]string, 0, 4)
	for _, encoder := range hardwareVideoEncodersFor(env, ctx, ffmpeg, profile.VideoCodec) {
		hwProfile := profile
		hwProfile.VideoCodec = encoder.Codec
		commands = append(commands, ffmpegVideoTranscodeArgs(inputPath, hwProfile, encoder.Family))
	}
	commands = append(commands, ffmpegVideoTranscodeArgs(inputPath, profile, ""))
	return commands
}

func ffmpegVideoTranscodeArgs(inputPath string, profile OutputProfile, hardwareFamily string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-dn",
		"-sn",
	}

	videoCodec := strings.TrimSpace(profile.VideoCodec)
	if videoCodec == "" {
		videoCodec = "copy"
	}
	args = append(args, "-c:v", videoCodec)
	if videoCodec != "copy" {
		if hardwareFamily != "" {
			args = append(args, hardwareVideoQualityArgs(hardwareFamily, profile.VideoCRF)...)
		} else if crf := strings.TrimSpace(profile.VideoCRF); crf != "" {
			args = append(args, "-crf", crf)
		}
	}

	audioCodec := strings.TrimSpace(profile.AudioCodec)
	if audioCodec == "" {
		audioCodec = "copy"
	}
	args = append(args, "-c:a", audioCodec)
	if audioCodec != "copy" {
		if bitrate := strings.TrimSpace(profile.AudioBitrate); bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
	}

	if strings.EqualFold(strings.TrimSpace(profile.VideoContainer), "mp4") {
		args = append(args, "-movflags", "+faststart")
	}
	return args
}

const (
	defaultHardwareCRF = "23"
	nvencPreset        = "p5"
)

func hardwareVideoQualityArgs(family, crf string) []string {
	crf = strings.TrimSpace(crf)
	if crf == "" {
		crf = defaultHardwareCRF
	}

	switch family {
	case "nvenc":
		return []string{"-preset", nvencPreset, "-rc", "vbr", "-cq", crf}
	case "qsv":
		return []string{"-global_quality", crf}
	case "amf":
		return []string{"-rc", "cqp", "-qp", crf}
	default:
		return nil
	}
}

func hardwareVideoEncodersFor(env *Env, ctx context.Context, ffmpeg, videoCodec string) []hardwareVideoEncoder {
	codec := normalizedVideoCodec(videoCodec)
	if codec == "" {
		return nil
	}

	encoders := detectFFmpegVideoEncoders(env, ctx, ffmpeg)
	candidates := hardwareEncoderCandidates(codec)
	out := make([]hardwareVideoEncoder, 0, len(candidates))
	for _, candidate := range candidates {
		if encoders[candidate.Codec] {
			out = append(out, candidate)
		}
	}
	return out
}

func normalizedVideoCodec(codec string) string {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "libx264", "h264", "avc":
		return "h264"
	case "libx265", "h265", "hevc":
		return "hevc"
	case "libsvtav1", "libaom-av1", "av1":
		return "av1"
	default:
		return ""
	}
}

func hardwareEncoderCandidates(codec string) []hardwareVideoEncoder {
	switch codec {
	case "h264":
		return []hardwareVideoEncoder{
			{Codec: "h264_nvenc", Family: "nvenc"},
			{Codec: "h264_qsv", Family: "qsv"},
			{Codec: "h264_amf", Family: "amf"},
		}
	case "hevc":
		return []hardwareVideoEncoder{
			{Codec: "hevc_nvenc", Family: "nvenc"},
			{Codec: "hevc_qsv", Family: "qsv"},
			{Codec: "hevc_amf", Family: "amf"},
		}
	case "av1":
		return []hardwareVideoEncoder{
			{Codec: "av1_nvenc", Family: "nvenc"},
			{Codec: "av1_qsv", Family: "qsv"},
			{Codec: "av1_amf", Family: "amf"},
		}
	default:
		return nil
	}
}

func detectFFmpegVideoEncoders(env *Env, ctx context.Context, ffmpeg string) map[string]bool {
	ffmpeg = strings.TrimSpace(ffmpeg)
	if ffmpeg == "" {
		return nil
	}

	env.ffmpegEncodersMu.Lock()
	if env.ffmpegEncodersValue == nil {
		env.ffmpegEncodersValue = map[string]map[string]bool{}
	}
	if encoders, ok := env.ffmpegEncodersValue[ffmpeg]; ok {
		env.ffmpegEncodersMu.Unlock()
		return maps.Clone(encoders)
	}
	env.ffmpegEncodersMu.Unlock()

	out, err := commandOutput(ctx, ffmpegEncodersTimeout, ffmpeg, "-hide_banner", "-encoders")
	if err != nil {
		return map[string]bool{}
	}
	encoders := parseFFmpegVideoEncoders(string(out))

	env.ffmpegEncodersMu.Lock()
	env.ffmpegEncodersValue[ffmpeg] = encoders
	env.ffmpegEncodersMu.Unlock()
	return maps.Clone(encoders)
}

func parseFFmpegVideoEncoders(output string) map[string]bool {
	encoders := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(fields[0], "V") {
			continue
		}
		encoders[fields[1]] = true
	}
	return encoders
}

func downloadFormats(req DownloadRequest) ([]string, []string) {
	if req.Profile.Mode != ModeVideo {
		return []string{""}, []string{""}
	}

	formats := req.Profile.VideoFmtChain
	if len(formats) == 0 {
		formats = []string{ytdlpBestFormat}
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

func StartDownloadRequestContext(env *Env, ctx context.Context, req DownloadRequest, deps CheckDepsResult, ch chan<- DlUpdate) {
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
			sendUpdate(ctx, ch, DlUpdate{Type: EvDone, OK: false, ErrText: err.Error()})
			return
		}
		req = preparedReq
		req.OutputDir, err = prepareDir(req.OutputDir)
		if err != nil {
			sendUpdate(ctx, ch, DlUpdate{Type: EvDone, OK: false, ErrText: err.Error()})
			return
		}
		cleanup.setRoot(req.OutputDir)

		if !downloadRequestUsesPlaylist(req) {
			result := runSingleDownload(env, ctx, req, deps, ch, cleanup)
			sendUpdate(ctx, ch, DlUpdate{Type: EvDone, OK: result.Err == nil, ErrText: result.ErrText})
			return
		}

		runPlaylistDownloads(env, ctx, req, deps, append([]PlaylistEntry(nil), req.Entries...), ch, &wg, cleanup)
	}()
}

func runSingleDownload(env *Env, ctx context.Context, req DownloadRequest, deps CheckDepsResult, ch chan<- DlUpdate, cleanup *downloadCleanup) downloadResult {
	sendUpdate(ctx, ch, DlUpdate{Type: EvStart, Slot: 0, Text: StringsFor(req.Locale).Downloading})
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

func runPlaylistDownloads(env *Env, ctx context.Context, req DownloadRequest, deps CheckDepsResult, entries []PlaylistEntry, ch chan<- DlUpdate, wg *sync.WaitGroup, cleanup *downloadCleanup) {
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
	env *Env,
	ctx context.Context,
	slot int,
	req DownloadRequest,
	deps CheckDepsResult,
	outputDir string,
	jobs <-chan PlaylistEntry,
	ch chan<- DlUpdate,
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

func runPlaylistEntry(env *Env, ctx context.Context, slot int, req DownloadRequest, deps CheckDepsResult, outputDir string, entry PlaylistEntry, ch chan<- DlUpdate, cleanup *downloadCleanup) {
	defer resetDownloadSlot(ctx, slot, ch)
	if !sendUpdate(ctx, ch, DlUpdate{Type: EvStart, Slot: slot, Text: entry.Title}) {
		return
	}
	result := runDownloadRequest(env, ctx, slot, req, deps, entry.URL, playlistOutputTemplate(outputDir, entry), []string{"--no-playlist"}, ch, cleanup)
	sendUpdate(ctx, ch, DlUpdate{Type: EvDone, Slot: slot, OK: result.Err == nil, ErrText: result.ErrText})
}

func resetDownloadSlot(ctx context.Context, slot int, ch chan<- DlUpdate) {
	timer := time.NewTimer(slotResetDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	if ctx == nil {
		select {
		case ch <- DlUpdate{Type: EvReset, Slot: slot}:
		default:
		}
		return
	}
	select {
	case ch <- DlUpdate{Type: EvReset, Slot: slot}:
	case <-ctx.Done():
	default:
	}
}

func playlistOutputDir(req DownloadRequest) string {
	title := "playlist"
	if req.PlaylistInfo != nil {
		title = req.PlaylistInfo.Title
	}
	dir := filepath.Join(req.OutputDir, sanitizeDirname(title))
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
