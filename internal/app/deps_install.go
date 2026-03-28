package app

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

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
