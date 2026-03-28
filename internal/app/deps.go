package app

import (
	"fmt"
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
