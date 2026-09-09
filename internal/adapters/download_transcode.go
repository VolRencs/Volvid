package adapters

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

const (
	defaultHardwareCRF = "23"
	nvencPreset        = "p5"
)

func transcodeDownloadedVideo(
	env *Env,
	ctx context.Context,
	slot int,
	profile core.OutputProfile,
	l core.Locale,
	deps core.CheckDepsResult,
	outputPath string,
	ch chan<- core.DlUpdate,
) (string, error) {
	if !profile.NeedsVideoTranscode() {
		return outputPath, nil
	}

	ffmpeg := ffmpegBinFor(env, deps)
	if strings.TrimSpace(outputPath) == "" {
		return "", errors.New("video transcoding failed: downloaded file path is unknown")
	}

	if ch != nil {
		sendUpdate(ctx, ch, core.DlUpdate{Type: core.EvProc, Slot: slot, Text: i18n.StringsFor(l).VideoConvertProc})
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

func ffmpegVideoTranscodeCommands(env *Env, ctx context.Context, ffmpeg, inputPath string, profile core.OutputProfile) [][]string {
	commands := make([][]string, 0, 4)
	for _, encoder := range hardwareVideoEncodersFor(env, ctx, ffmpeg, profile.VideoCodec) {
		hwProfile := profile
		hwProfile.VideoCodec = encoder.Codec
		commands = append(commands, ffmpegVideoTranscodeArgs(inputPath, hwProfile, encoder.Family))
	}
	commands = append(commands, ffmpegVideoTranscodeArgs(inputPath, profile, ""))
	return commands
}
func ffmpegVideoTranscodeArgs(inputPath string, profile core.OutputProfile, hardwareFamily string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-dn",
	}
	args = append(args, subtitleTranscodeArgs(profile)...)

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

// subtitleTranscodeArgs keeps embedded subtitle tracks when the profile
// wants them (mp4 needs mov_text, mkv accepts srt copy).
func subtitleTranscodeArgs(profile core.OutputProfile) []string {
	if !profile.WantsSubtitles() {
		return []string{"-sn"}
	}
	codec := "mov_text"
	if strings.EqualFold(strings.TrimSpace(profile.VideoContainer), "mkv") {
		codec = "srt"
	}
	return []string{"-map", "0:s?", "-c:s", codec}
}
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
	if env.ffmpegEncodersFlight == nil {
		env.ffmpegEncodersFlight = map[string]*encoderFlight{}
	}
	if encoders, ok := env.ffmpegEncodersValue[ffmpeg]; ok {
		env.ffmpegEncodersMu.Unlock()
		return maps.Clone(encoders)
	}
	if flight, ok := env.ffmpegEncodersFlight[ffmpeg]; ok {
		env.ffmpegEncodersMu.Unlock()
		if ctx == nil {
			<-flight.done
			return maps.Clone(flight.encoders)
		}
		select {
		case <-flight.done:
			return maps.Clone(flight.encoders)
		case <-ctx.Done():
			return map[string]bool{}
		}
	}
	flight := &encoderFlight{done: make(chan struct{})}
	env.ffmpegEncodersFlight[ffmpeg] = flight
	env.ffmpegEncodersMu.Unlock()

	out, err := commandOutput(ctx, ffmpegEncodersTimeout, ffmpeg, "-hide_banner", "-encoders")
	encoders := map[string]bool{}
	if err == nil {
		encoders = parseFFmpegVideoEncoders(string(out))
	}

	env.ffmpegEncodersMu.Lock()
	env.ffmpegEncodersValue[ffmpeg] = encoders
	flight.encoders = encoders
	close(flight.done)
	delete(env.ffmpegEncodersFlight, ffmpeg)
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
