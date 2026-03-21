package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type CheckDepsResult struct {
	YtdlpVer      string
	FFmpegVer     string
	FFmpegMissing bool
}

func DetectDeps() CheckDepsResult {
	r := CheckDepsResult{YtdlpVer: YtdlpVersion()}
	if IsWindows {
		bin := filepath.Join(DepsDir, "ffmpeg.exe")
		if _, err := os.Stat(bin); err == nil {
			FFmpegResolved = bin
			r.FFmpegVer = FFmpegVersion()
		} else {
			r.FFmpegMissing = true
		}
	} else if path, err := exec.LookPath("ffmpeg"); err == nil {
		FFmpegResolved = path
		r.FFmpegVer = FFmpegVersion()
	}
	return r
}

func YtdlpVersion() string {
	if _, err := os.Stat(YtdlpBin); err != nil {
		return ""
	}
	out, err := exec.Command(YtdlpBin, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func FFmpegVersion() string {
	bin := FFmpegResolved
	if bin == "" {
		var err error
		bin, err = exec.LookPath("ffmpeg")
		if err != nil {
			return ""
		}
	}
	out, _ := exec.Command(bin, "-version").CombinedOutput()
	if len(out) == 0 {
		return ""
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "version" && i+1 < len(fields) {
			ver := fields[i+1]
			if idx := strings.IndexAny(ver, "-_"); idx > 0 {
				ver = ver[:idx]
			}
			return ver
		}
	}
	return ""
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
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}
	if err := DownloadFile(YtdlpURL(), YtdlpBin, ch); err != nil {
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
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}
	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, "ffmpeg.zip")
	if err := DownloadFile(ffmpegWinURL, archive, ch); err != nil {
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
	if err := InstallYtDlp(ch); err != nil {
		return err
	}
	if IsWindows {
		return InstallFFmpeg(ch)
	}
	return nil
}
