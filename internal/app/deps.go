package app

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CheckDepsResult struct {
	YtdlpVer      string
	FFmpegVer     string
	FFmpegMissing bool
}

const (
	versionProbeAttempts = 8
	versionProbeDelay    = 250 * time.Millisecond
	versionProbeTimeout  = 5 * time.Second
)

type DepsLogger func(format string, args ...any)

func DetectDeps() CheckDepsResult {
	r := CheckDepsResult{}
	r.YtdlpVer = YtdlpVersion()
	r.FFmpegVer, r.FFmpegMissing = detectFFmpegVersion()
	return r
}

func YtdlpVersion() string {
	if !pathExists(YtdlpBin) {
		return ""
	}
	return commandVersionLine(YtdlpBin, "--version")
}

func FFmpegVersion() string {
	ver, _ := detectFFmpegVersion()
	return ver
}

func detectFFmpegVersion() (string, bool) {
	bin, missing := ffmpegBinaryPath()
	if bin == "" {
		FFmpegResolved = ""
		return "", missing
	}
	FFmpegResolved = bin

	line := commandVersionLine(bin, "-version")
	if line == "" {
		return "", false
	}
	return ffmpegVersionFromLine(line), false
}

func ffmpegBinaryPath() (string, bool) {
	if IsWindows {
		bin := filepath.Join(DepsDir, "ffmpeg.exe")
		if pathExists(bin) {
			return bin, false
		}
		return "", true
	}

	if pathExists(FFmpegResolved) {
		return FFmpegResolved, false
	}

	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", false
	}
	return bin, false
}

func commandVersionLine(bin string, args ...string) string {
	for attempt := 0; attempt < versionProbeAttempts; attempt++ {
		out, _ := commandCombinedOutput(context.Background(), versionProbeTimeout, bin, args...)
		line := firstNonEmptyLine(string(out))
		if line != "" {
			return line
		}
		if attempt+1 < versionProbeAttempts {
			time.Sleep(versionProbeDelay)
		}
	}
	return ""
}

func firstNonEmptyLine(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func ffmpegVersionFromLine(line string) string {
	const prefix = "ffmpeg version "

	lower := strings.ToLower(line)
	idx := strings.Index(lower, prefix)
	if idx < 0 {
		return strings.TrimSpace(line)
	}

	rest := strings.TrimSpace(line[idx+len(prefix):])
	if rest == "" {
		return ""
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return strings.TrimSpace(line)
	}

	return strings.Trim(fields[0], " \t\r\n,;:()[]{}\"'")
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func YtdlpURL() string {
	switch {
	case IsWindows:
		return ytdlpBase + "yt-dlp.exe"
	case Arch == "arm64":
		return ytdlpBase + "yt-dlp_linux_aarch64"
	default:
		return ytdlpBase + "yt-dlp_linux"
	}
}

func InstallYtDlp(ch chan<- FileProgress) error {
	return InstallYtDlpFor(LoadLocale(), ch)
}

func InstallYtDlpFor(l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}
	if err := DownloadFile(YtdlpURL(), YtdlpBin, l, ch); err != nil {
		return err
	}
	if !IsWindows {
		if err := os.Chmod(YtdlpBin, 0o755); err != nil {
			return fmt.Errorf("chmod yt-dlp: %w", err)
		}
	}
	if YtdlpVersion() == "" {
		return fmt.Errorf("бинарник yt-dlp скачан, но не запускается")
	}
	return nil
}

func extractZipEntry(zf *zip.File, dest string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func InstallFFmpeg(ch chan<- FileProgress) error {
	return InstallFFmpegFor(LoadLocale(), ch)
}

func InstallFFmpegFor(l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}
	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, "ffmpeg.zip")
	if err := DownloadFile(ffmpegWinURL, archive, l, ch); err != nil {
		return err
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("открытие архива: %w", err)
	}
	defer zr.Close()

	targets := []string{"ffmpeg.exe", "ffprobe.exe"}
	foundMain := false
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		if !slices.Contains(targets, name) {
			continue
		}
		if err := extractZipEntry(zf, filepath.Join(DepsDir, name)); err != nil {
			if name == "ffmpeg.exe" {
				return fmt.Errorf("ошибка извлечения %s: %w", name, err)
			}
			continue
		}
		if name == "ffmpeg.exe" {
			foundMain = true
		}
	}
	if !foundMain {
		return fmt.Errorf("ffmpeg.exe не найден в архиве")
	}
	FFmpegResolved = filepath.Join(DepsDir, "ffmpeg.exe")
	return nil
}

func InstallAllDeps(ch chan<- FileProgress) error {
	return InstallAllDepsFor(LoadLocale(), ch)
}

func InstallAllDepsFor(l Locale, ch chan<- FileProgress) error {
	if err := InstallYtDlpFor(l, ch); err != nil {
		return err
	}
	if IsWindows {
		return InstallFFmpegFor(l, ch)
	}
	return nil
}

func EnsureRuntimeDeps(logf DepsLogger) (CheckDepsResult, error) {
	deps := DetectDeps()

	if deps.YtdlpVer == "" {
		if logf != nil {
			logf("Зависимости: yt-dlp не найден, скачиваю…")
		}
		if err := InstallYtDlpFor(LocaleEN, nil); err != nil {
			return DetectDeps(), fmt.Errorf("установка yt-dlp: %w", err)
		}
		deps = DetectDeps()
		if deps.YtdlpVer == "" {
			return deps, fmt.Errorf("yt-dlp скачан, но версия не определяется")
		}
		if logf != nil {
			logf("Зависимости: yt-dlp готов (%s)", deps.YtdlpVer)
		}
	}

	if IsWindows && deps.FFmpegMissing {
		if logf != nil {
			logf("Зависимости: ffmpeg не найден, скачиваю…")
		}
		if err := InstallFFmpegFor(LocaleEN, nil); err != nil {
			return DetectDeps(), fmt.Errorf("установка ffmpeg: %w", err)
		}
		deps = DetectDeps()
		if deps.FFmpegVer == "" {
			return deps, fmt.Errorf("ffmpeg скачан, но версия не определяется")
		}
		if logf != nil {
			logf("Зависимости: ffmpeg готов (%s)", deps.FFmpegVer)
		}
	}

	return deps, nil
}
