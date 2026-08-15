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
			if err := transcodeDownloadedVideo(ctx, slot, req.Profile, req.Locale, deps, result.OutputPath, ch); err != nil {
				return failedDownload(err)
			}
			return result
		}
	}

	return result
}

func transcodeDownloadedVideo(
	ctx context.Context,
	slot int,
	profile OutputProfile,
	l Locale,
	deps CheckDepsResult,
	outputPath string,
	ch chan<- DlUpdate,
) error {
	if !profile.NeedsVideoTranscode() {
		return nil
	}

	ffmpeg := strings.TrimSpace(deps.FFmpeg.Path)
	if ffmpeg == "" {
		ffmpeg = FFmpegBin
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("video transcoding failed: downloaded file path is unknown")
	}

	tmp, err := transcodeTempPath(outputPath, profile.VideoContainer)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	if ch != nil {
		ch <- DlUpdate{Type: EvProc, Slot: slot, Text: StringsFor(l).VideoConvertProc}
	}

	commands := ffmpegVideoTranscodeCommands(ctx, ffmpeg, outputPath, tmp, profile)
	var lastErr error
	var lastOut []byte
	for i, command := range commands {
		if i > 0 {
			_ = os.Remove(tmp)
		}
		out, err := commandCombinedOutput(ctx, 0, ffmpeg, command.Args...)
		if err == nil {
			if err := replaceFile(outputPath, tmp); err != nil {
				return fmt.Errorf("video transcoding failed: %w", err)
			}
			return nil
		}

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
	return fmt.Errorf("video transcoding failed: %s", text)
}

func transcodeTempPath(outputPath, container string) (string, error) {
	dir := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	ext := strings.TrimSpace(container)
	if ext == "" {
		ext = strings.TrimPrefix(filepath.Ext(base), ".")
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

type videoTranscodeCommand struct {
	Label string
	Args  []string
}

type hardwareVideoEncoder struct {
	Name   string
	Codec  string
	Family string
}

func ffmpegVideoTranscodeCommands(ctx context.Context, ffmpeg, inputPath, outputPath string, profile OutputProfile) []videoTranscodeCommand {
	commands := make([]videoTranscodeCommand, 0, 4)
	for _, encoder := range hardwareVideoEncodersFor(ctx, ffmpeg, profile.VideoCodec) {
		hwProfile := profile
		hwProfile.VideoCodec = encoder.Codec
		commands = append(commands, videoTranscodeCommand{
			Label: encoder.Name,
			Args:  ffmpegVideoTranscodeArgs(inputPath, outputPath, hwProfile, encoder.Family),
		})
	}
	commands = append(commands, videoTranscodeCommand{
		Label: "cpu",
		Args:  ffmpegVideoTranscodeArgs(inputPath, outputPath, profile, ""),
	})
	return commands
}

func ffmpegVideoTranscodeArgs(inputPath, outputPath string, profile OutputProfile, hardwareFamily string) []string {
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
	return append(args, outputPath)
}

func hardwareVideoQualityArgs(family, crf string) []string {
	crf = strings.TrimSpace(crf)
	if crf == "" {
		crf = "23"
	}

	switch family {
	case "nvenc":
		return []string{"-preset", "p5", "-rc", "vbr", "-cq", crf}
	case "qsv":
		return []string{"-global_quality", crf}
	default:
		return nil
	}
}

func hardwareVideoEncodersFor(ctx context.Context, ffmpeg, videoCodec string) []hardwareVideoEncoder {
	codec := normalizedVideoCodec(videoCodec)
	if codec == "" {
		return nil
	}

	encoders := detectFFmpegVideoEncoders(ctx, ffmpeg)
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
			{Name: "NVIDIA NVENC H.264", Codec: "h264_nvenc", Family: "nvenc"},
			{Name: "Intel Quick Sync H.264", Codec: "h264_qsv", Family: "qsv"},
			{Name: "AMD AMF H.264", Codec: "h264_amf", Family: "amf"},
		}
	case "hevc":
		return []hardwareVideoEncoder{
			{Name: "NVIDIA NVENC H.265", Codec: "hevc_nvenc", Family: "nvenc"},
			{Name: "Intel Quick Sync H.265", Codec: "hevc_qsv", Family: "qsv"},
			{Name: "AMD AMF H.265", Codec: "hevc_amf", Family: "amf"},
		}
	case "av1":
		return []hardwareVideoEncoder{
			{Name: "NVIDIA NVENC AV1", Codec: "av1_nvenc", Family: "nvenc"},
			{Name: "Intel Quick Sync AV1", Codec: "av1_qsv", Family: "qsv"},
			{Name: "AMD AMF AV1", Codec: "av1_amf", Family: "amf"},
		}
	default:
		return nil
	}
}

var (
	ffmpegEncodersMu    sync.Mutex
	ffmpegEncodersCache = map[string]map[string]bool{}
)

func detectFFmpegVideoEncoders(ctx context.Context, ffmpeg string) map[string]bool {
	ffmpeg = strings.TrimSpace(ffmpeg)
	if ffmpeg == "" {
		return nil
	}

	ffmpegEncodersMu.Lock()
	if encoders, ok := ffmpegEncodersCache[ffmpeg]; ok {
		ffmpegEncodersMu.Unlock()
		return encoders
	}
	ffmpegEncodersMu.Unlock()

	out, err := commandOutput(ctx, 3*time.Second, ffmpeg, "-hide_banner", "-encoders")
	encoders := parseFFmpegVideoEncoders(string(out))
	if err != nil {
		encoders = map[string]bool{}
	}

	ffmpegEncodersMu.Lock()
	ffmpegEncodersCache[ffmpeg] = encoders
	ffmpegEncodersMu.Unlock()
	return encoders
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

func replaceFile(dst, src string) error {
	backupFile, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".backup-*")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(backup)
		return err
	}
	_ = os.Remove(backup)

	if err := os.Rename(dst, backup); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(backup, dst)
		return err
	}
	return os.Remove(backup)
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
	defer resetDownloadSlot(ctx, slot, ch)
	ch <- DlUpdate{Type: EvStart, Slot: slot, Text: entry.Title}
	result := runDownloadRequest(ctx, slot, req, entry.URL, playlistOutputTemplate(outputDir, entry), []string{"--no-playlist"}, ch)
	ch <- DlUpdate{Type: EvDone, Slot: slot, OK: result.Err == nil, ErrText: result.ErrText}
}

func resetDownloadSlot(ctx context.Context, slot int, ch chan<- DlUpdate) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(slotResetDelay):
	}
	select {
	case ch <- DlUpdate{Type: EvReset, Slot: slot}:
	case <-ctx.Done():
	}
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
