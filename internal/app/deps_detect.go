package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
