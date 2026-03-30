package app

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func InstallDependencyFor(key string, l Locale, ch chan<- FileProgress) error {
	var err error
	switch strings.TrimSpace(key) {
	case "ytdlp":
		err = InstallYtDlpFor(l, ch)
	case "ffmpeg":
		err = InstallFFmpegFor(l, ch)
	case "node":
		err = InstallNodeFor(l, ch)
	default:
		return fmt.Errorf("unsupported dependency: %s", key)
	}
	if err != nil {
		return err
	}
	InvalidateDepsCache()
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

func InstallFFmpegFor(l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, ffmpegArchiveFilename())
	if err := DownloadFile(ffmpegArchiveURL(), archive, l, ch); err != nil {
		return err
	}

	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(archive, map[string]string{
			binaryBaseName(FFmpegBin):  FFmpegBin,
			binaryBaseName(FFprobeBin): FFprobeBin,
		}); err != nil {
			return err
		}
	} else {
		extractDir := filepath.Join(tmp, "extract")
		if err := extractArchiveWithTar(archive, extractDir); err != nil {
			return err
		}
		if err := copyExtractedBinaries(extractDir, map[string]string{
			binaryBaseName(FFmpegBin):  FFmpegBin,
			binaryBaseName(FFprobeBin): FFprobeBin,
		}); err != nil {
			return err
		}
	}

	if detectExecutableDependency("ffmpeg", "ffmpeg", false, true, nil, FFmpegBin, []string{"-version"}, ffmpegVersionFromLine, true).Version == "" {
		return fmt.Errorf("бинарник ffmpeg скачан, но не запускается")
	}
	return nil
}

func InstallNodeFor(l Locale, ch chan<- FileProgress) error {
	url, filename, err := nodeDownloadAsset()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "node-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, filename)
	if err := DownloadFile(url, archive, l, ch); err != nil {
		return err
	}

	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(archive, map[string]string{
			binaryBaseName(NodeBin): NodeBin,
		}); err != nil {
			return err
		}
	} else {
		extractDir := filepath.Join(tmp, "extract")
		if err := extractArchiveWithTar(archive, extractDir); err != nil {
			return err
		}
		if err := copyExtractedBinaries(extractDir, map[string]string{
			binaryBaseName(NodeBin): NodeBin,
		}); err != nil {
			return err
		}
	}

	if detectExecutableDependency("node", "node", false, true, nil, NodeBin, []string{"--version"}, firstNonEmptyLine, true).Version == "" {
		return fmt.Errorf("бинарник node скачан, но не запускается")
	}
	return nil
}

func ffmpegArchiveURL() string {
	switch {
	case IsWindows && Arch == "arm64":
		return ffmpegWinARM64URL
	case IsWindows:
		return ffmpegWinURL
	case Arch == "arm64":
		return ffmpegLinuxARM64URL
	default:
		return ffmpegLinuxAMD64URL
	}
}

func ffmpegArchiveFilename() string {
	url := ffmpegArchiveURL()
	return filepath.Base(strings.TrimSpace(url))
}

func nodeDownloadAsset() (string, string, error) {
	filename, err := nodeAssetFilename()
	if err != nil {
		return "", "", err
	}
	return nodeLatestV22URL + filename, filename, nil
}

func nodeAssetFilename() (string, error) {
	manifest, err := downloadText(nodeLatestV22URL + "SHASUMS256.txt")
	if err != nil {
		return "", fmt.Errorf("node manifest: %w", err)
	}

	suffixes := nodeAssetSuffixes()
	for _, line := range strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("node asset for %s/%s not found", runtime.GOOS, Arch)
}

func nodeAssetSuffixes() []string {
	switch {
	case IsWindows && Arch == "arm64":
		return []string{"-win-arm64.zip"}
	case IsWindows:
		return []string{"-win-x64.zip"}
	case Arch == "arm64":
		return []string{"-linux-arm64.tar.gz"}
	default:
		return []string{"-linux-x64.tar.gz"}
	}
}

func downloadText(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := newDownloadRequest(ctx, url)
	if err != nil {
		return "", err
	}

	resp, err := doSafeRequest(ctx, dlClient, req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if err := validateDownloadResponse(resp, url); err != nil {
		return "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func extractZipBinaries(archive string, targets map[string]string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("открытие архива: %w", err)
	}
	defer zr.Close()

	found := make(map[string]bool, len(targets))
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		dest, ok := targets[name]
		if !ok {
			continue
		}
		if err := extractZipEntry(zf, dest); err != nil {
			return fmt.Errorf("ошибка извлечения %s: %w", name, err)
		}
		found[name] = true
	}

	for name := range targets {
		if !found[name] {
			return fmt.Errorf("%s не найден в архиве", name)
		}
	}
	return nil
}

func extractArchiveWithTar(archive, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	output, err := commandCombinedOutput(context.Background(), 2*time.Minute, "tar", "-xf", archive, "-C", destDir)
	if err == nil {
		return nil
	}
	if line := firstNonEmptyLine(string(output)); line != "" {
		return errors.New(line)
	}
	return err
}

func copyExtractedBinaries(root string, targets map[string]string) error {
	found := make(map[string]bool, len(targets))

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		name := filepath.Base(path)
		dest, ok := targets[name]
		if !ok {
			return nil
		}
		if err := copyExtractedFile(path, dest); err != nil {
			return err
		}
		found[name] = true
		return nil
	})
	if err != nil {
		return err
	}

	for name := range targets {
		if !found[name] {
			return fmt.Errorf("%s не найден в архиве", name)
		}
	}
	return nil
}

func copyExtractedFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func binaryBaseName(path string) string {
	return filepath.Base(strings.TrimSpace(path))
}
