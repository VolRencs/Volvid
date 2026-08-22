package app

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func InstallDependencyFor(ctx context.Context, key string, l Locale, ch chan<- FileProgress) error {
	var err error
	switch strings.TrimSpace(key) {
	case "ytdlp":
		err = InstallYtDlpFor(ctx, l, ch)
	case "ffmpeg":
		err = InstallFFmpegFor(ctx, l, ch)
	case "node":
		err = InstallNodeFor(ctx, l, ch)
	default:
		return fmt.Errorf("unsupported dependency: %s", key)
	}
	if err != nil {
		return err
	}
	InvalidateDepsCache()
	return nil
}

func InstallYtDlpFor(ctx context.Context, l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	url, _, checksum, err := ytdlpDownloadAsset(ctx)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(DepsDir, ".ytdlp-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	stagedYtdlp := filepath.Join(staging, binaryBaseName(YtdlpBin))
	if err := DownloadFileContext(ctx, url, stagedYtdlp, l, ch); err != nil {
		return err
	}
	if err := verifyFileSHA256(stagedYtdlp, checksum); err != nil {
		return err
	}
	if !IsWindows {
		if err := os.Chmod(stagedYtdlp, 0o755); err != nil {
			return fmt.Errorf("chmod yt-dlp: %w", err)
		}
	}
	if detectExecutableDependency("ytdlp", "yt-dlp", true, true, nil, stagedYtdlp, []string{"--version"}, firstNonEmptyLine, true).Version == "" {
		return fmt.Errorf("бинарник yt-dlp скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{stagedYtdlp: YtdlpBin}); err != nil {
		return err
	}
	return nil
}

func extractZipEntry(zf *zip.File, dest string) error {
	if zf == nil {
		return errors.New("zip entry is nil")
	}
	mode := zf.FileInfo().Mode()
	if zf.FileInfo().IsDir() || mode&os.ModeSymlink != 0 {
		return fmt.Errorf("unsupported zip entry type: %s", zf.Name)
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".extract-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, rc); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}

func InstallFFmpegFor(ctx context.Context, l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archiveURL, archiveName, err := ffmpegArchiveAsset()
	if err != nil {
		return err
	}
	archive := filepath.Join(tmp, archiveName)
	if err := DownloadFileContext(ctx, archiveURL, archive, l, ch); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(DepsDir, ".ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	ffmpegName := binaryBaseName(FFmpegBin)
	ffprobeName := binaryBaseName(FFprobeBin)
	stagedFFmpeg := filepath.Join(staging, ffmpegName)
	stagedFFprobe := filepath.Join(staging, ffprobeName)
	targets := map[string]string{
		ffmpegName:  stagedFFmpeg,
		ffprobeName: stagedFFprobe,
	}

	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(ctx, archive, targets); err != nil {
			return err
		}
	} else {
		if err := extractArchiveBinariesWithTar(ctx, archive, targets); err != nil {
			return err
		}
	}

	if detectExecutableDependency("ffmpeg", "ffmpeg", false, true, nil, stagedFFmpeg, []string{"-version"}, ffmpegVersionFromLine, true).Version == "" {
		return fmt.Errorf("бинарник ffmpeg скачан, но не запускается")
	}
	if commandVersionLine(stagedFFprobe, "-version") == "" {
		return fmt.Errorf("бинарник ffprobe скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{
		stagedFFmpeg:  FFmpegBin,
		stagedFFprobe: FFprobeBin,
	}); err != nil {
		return err
	}
	return nil
}

func InstallNodeFor(ctx context.Context, l Locale, ch chan<- FileProgress) error {
	url, filename, checksum, err := nodeDownloadAsset(ctx)
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
	if err := DownloadFileContext(ctx, url, archive, l, ch); err != nil {
		return err
	}
	if checksum != "" {
		if err := verifyFileSHA256(archive, checksum); err != nil {
			return err
		}
	}

	staging, err := os.MkdirTemp(DepsDir, ".node-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	nodeName := binaryBaseName(NodeBin)
	stagedNode := filepath.Join(staging, nodeName)
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(ctx, archive, map[string]string{
			nodeName: stagedNode,
		}); err != nil {
			return err
		}
	} else {
		if err := extractArchiveBinariesWithTar(ctx, archive, map[string]string{
			nodeName: stagedNode,
		}); err != nil {
			return err
		}
	}

	if detectExecutableDependency("node", "node", false, true, nil, stagedNode, []string{"--version"}, firstNonEmptyLine, true).Version == "" {
		return fmt.Errorf("бинарник node скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{stagedNode: NodeBin}); err != nil {
		return err
	}
	return nil
}

func ffmpegArchiveAsset() (string, string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", "", err
	}
	url := strings.TrimSpace(platform.FFmpegURL)
	if url == "" {
		return "", "", fmt.Errorf("ffmpeg asset URL is empty")
	}
	return url, filepath.Base(url), nil
}

func nodeDownloadAsset(ctx context.Context) (string, string, string, error) {
	filename, checksum, err := nodeAssetFilename(ctx)
	if err != nil {
		return "", "", "", err
	}
	return nodeLatestURL + filename, filename, checksum, nil
}

func ytdlpDownloadAsset(ctx context.Context) (string, string, string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", "", "", err
	}
	asset := strings.TrimSpace(platform.YTDLPAsset)
	if asset == "" {
		return "", "", "", fmt.Errorf("yt-dlp asset name is empty")
	}
	checksum, err := ytdlpAssetChecksum(ctx, asset)
	if err != nil {
		return "", "", "", err
	}
	return ytdlpBase + asset, asset, checksum, nil
}

func ytdlpAssetChecksum(ctx context.Context, asset string) (string, error) {
	manifest, err := downloadText(ctx, ytdlpBase+"SHA2-256SUMS")
	if err != nil {
		return "", fmt.Errorf("yt-dlp checksum manifest: %w", err)
	}
	checksum, err := checksumFromManifest(manifest, asset)
	if err != nil {
		return "", fmt.Errorf("yt-dlp checksum for %s: %w", asset, err)
	}
	return checksum, nil
}

func nodeAssetFilename(ctx context.Context) (string, string, error) {
	manifest, err := downloadText(ctx, nodeLatestURL+"SHASUMS256.txt")
	if err != nil {
		return "", "", fmt.Errorf("node manifest: %w", err)
	}

	suffix, err := nodeAssetSuffix()
	if err != nil {
		return "", "", err
	}
	return nodeAssetFromManifest(manifest, suffix)
}

func nodeAssetFromManifest(manifest, suffix string) (string, string, error) {
	for _, line := range strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		if strings.HasSuffix(name, suffix) {
			checksum, err := normalizeSHA256(fields[0])
			if err != nil {
				return "", "", fmt.Errorf("node checksum for %s: %w", name, err)
			}
			return name, checksum, nil
		}
	}

	return "", "", fmt.Errorf("node asset with suffix %s not found", suffix)
}

func checksumFromManifest(manifest, name string) (string, error) {
	name = strings.TrimSpace(name)
	for _, line := range strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[len(fields)-1] != name {
			continue
		}
		checksum, err := normalizeSHA256(fields[0])
		if err != nil {
			return "", err
		}
		return checksum, nil
	}
	return "", fmt.Errorf("asset %s not found", name)
}

func nodeAssetSuffix() (string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", err
	}
	if platform.NodeAssetSuffix == "" {
		return "", fmt.Errorf("node asset suffix is empty")
	}
	return platform.NodeAssetSuffix, nil
}

func downloadText(ctx context.Context, url string) (string, error) {
	ctx = resolveContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
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

func normalizeSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("expected %d hex chars", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", err
	}
	return value, nil
}

func verifyFileSHA256(path, expected string) error {
	expected, err := normalizeSHA256(expected)
	if err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func extractZipBinaries(ctx context.Context, archive string, targets map[string]string) error {
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

func extractArchiveBinariesWithTar(ctx context.Context, archive string, targets map[string]string) error {
	entries, err := listTarArchive(ctx, archive)
	if err != nil {
		return err
	}
	selected, err := selectTarBinaryEntries(entries, targets)
	if err != nil {
		return err
	}

	destDir, err := os.MkdirTemp(filepath.Dir(archive), "extract-*")
	if err != nil {
		return fmt.Errorf("временная директория распаковки: %w", err)
	}
	defer os.RemoveAll(destDir)

	if err := extractTarEntriesWithTar(ctx, archive, destDir, selected); err != nil {
		return err
	}
	return copyExtractedBinaries(destDir, targets)
}

func listTarArchive(ctx context.Context, archive string) ([]string, error) {
	output, err := commandCombinedOutput(resolveContext(ctx), tarCommandTimeout, "tar", "-tf", archive)
	if err != nil {
		return nil, tarCommandError(err, output)
	}
	return parseTarListOutput(string(output))
}

func parseTarListOutput(output string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	entries := make([]string, 0, len(lines))
	for _, line := range lines {
		entry, err := validateArchiveMemberPath(line)
		if err != nil {
			return nil, err
		}
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil, errors.New("archive is empty")
	}
	return entries, nil
}

func selectTarBinaryEntries(entries []string, targets map[string]string) ([]string, error) {
	selected := make(map[string]string, len(targets))
	for _, entry := range entries {
		name := path.Base(entry)
		if _, ok := targets[name]; !ok {
			continue
		}
		if current := selected[name]; current == "" || betterArchiveBinaryEntry(entry, current) {
			selected[name] = entry
		}
	}

	out := make([]string, 0, len(targets))
	for name := range targets {
		entry := selected[name]
		if entry == "" {
			return nil, fmt.Errorf("%s не найден в архиве", name)
		}
		out = append(out, entry)
	}
	return out, nil
}

func betterArchiveBinaryEntry(candidate, current string) bool {
	candidateBin := strings.Contains("/"+candidate, "/bin/")
	currentBin := strings.Contains("/"+current, "/bin/")
	switch {
	case candidateBin != currentBin:
		return candidateBin
	default:
		return len(candidate) < len(current)
	}
}

func validateArchiveMemberPath(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("unsafe absolute archive path: %s", raw)
	}
	clean := path.Clean(raw)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe archive path: %s", raw)
	}
	return clean, nil
}

func extractTarEntriesWithTar(ctx context.Context, archive, destDir string, entries []string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	args := []string{"-xf", archive, "-C", destDir, "--no-same-owner", "--no-same-permissions", "--"}
	args = append(args, entries...)
	output, err := commandCombinedOutput(resolveContext(ctx), tarCommandTimeout, "tar", args...)
	if err != nil {
		return tarCommandError(err, output)
	}
	return nil
}

func tarCommandError(err error, output []byte) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("tar is required to extract this archive")
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
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink from archive: %s", src)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular file from archive: %s", src)
	}

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

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func replaceInstalledBinaries(paths map[string]string) error {
	type rollback struct {
		origDest string
		backup   string
	}
	var rollbacks []rollback

	for src, dest := range paths {
		backup := replacementBackupPath(dest)
		hadDest := true
		if err := os.Rename(dest, backup); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				for _, r := range rollbacks {
					_ = os.Rename(r.backup, r.origDest)
				}
				return fmt.Errorf("установка %s: %w", filepath.Base(dest), err)
			}
			hadDest = false
		}
		if err := os.Rename(src, dest); err != nil {
			if hadDest {
				_ = os.Rename(backup, dest)
			}
			for _, r := range rollbacks {
				_ = os.Rename(r.backup, r.origDest)
			}
			return fmt.Errorf("установка %s: %w", filepath.Base(dest), err)
		}
		if hadDest {
			rollbacks = append(rollbacks, rollback{origDest: dest, backup: backup})
		}
	}

	for _, r := range rollbacks {
		_ = os.Remove(r.backup)
	}
	return nil
}

func binaryBaseName(path string) string {
	return filepath.Base(strings.TrimSpace(path))
}
