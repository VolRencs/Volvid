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

func InstallDependencyFor(env *Env, ctx context.Context, key string, l Locale, ch chan<- FileProgress) error {
	var err error
	switch strings.TrimSpace(key) {
	case "ytdlp":
		err = InstallYtDlpFor(env, ctx, l, ch)
	case "ffmpeg":
		err = InstallFFmpegFor(env, ctx, l, ch)
	case "node":
		err = InstallNodeFor(env, ctx, l, ch)
	default:
		return fmt.Errorf("unsupported dependency: %s", key)
	}
	if err != nil {
		return fmt.Errorf("install dependency %s: %w", key, err)
	}
	InvalidateDepsCache(env)
	return nil
}

func InstallYtDlpFor(env *Env, ctx context.Context, l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(env.DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	url, _, checksum, err := ytdlpDownloadAsset(env)
	if err != nil {
		return fmt.Errorf("yt-dlp asset metadata: %w", err)
	}
	staging, err := os.MkdirTemp(env.DepsDir, ".ytdlp-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	stagedYtdlp := filepath.Join(staging, binaryBaseName(env.YtdlpBin))
	if err := DownloadFileContext(env, ctx, url, stagedYtdlp, l, ch); err != nil {
		return fmt.Errorf("download yt-dlp: %w", err)
	}
	if err := verifyFileSHA256(stagedYtdlp, checksum); err != nil {
		return fmt.Errorf("verify yt-dlp checksum: %w", err)
	}
	if !env.IsWindows {
		if err := os.Chmod(stagedYtdlp, 0o755); err != nil {
			return fmt.Errorf("chmod yt-dlp: %w", err)
		}
	}
	if detectExecutableDependency(depSpec{Key: "ytdlp", Name: "yt-dlp", ManagedPath: stagedYtdlp, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}, true).Version == "" {
		return fmt.Errorf("бинарник yt-dlp скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{stagedYtdlp: env.YtdlpBin}); err != nil {
		return fmt.Errorf("replace yt-dlp binary: %w", err)
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
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create directory for zip entry: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".extract-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, rc); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("extract zip data: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod extracted file: %w", err)
	}
	return os.Rename(tmpName, dest)
}

func InstallFFmpegFor(env *Env, ctx context.Context, l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(env.DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archiveURL, archiveName, err := ffmpegArchiveAsset()
	if err != nil {
		return fmt.Errorf("ffmpeg asset metadata: %w", err)
	}
	archive := filepath.Join(tmp, archiveName)
	if err := DownloadFileContext(env, ctx, archiveURL, archive, l, ch); err != nil {
		return fmt.Errorf("download ffmpeg archive: %w", err)
	}

	staging, err := os.MkdirTemp(env.DepsDir, ".ffmpeg-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	ffmpegName := binaryBaseName(env.FFmpegBin)
	ffprobeName := binaryBaseName(env.FFprobeBin)
	stagedFFmpeg := filepath.Join(staging, ffmpegName)
	stagedFFprobe := filepath.Join(staging, ffprobeName)
	targets := map[string]string{
		ffmpegName:  stagedFFmpeg,
		ffprobeName: stagedFFprobe,
	}

	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(ctx, archive, targets); err != nil {
			return fmt.Errorf("extract ffmpeg zip: %w", err)
		}
	} else {
		if err := extractArchiveBinariesWithTar(ctx, archive, targets); err != nil {
			return fmt.Errorf("extract ffmpeg archive: %w", err)
		}
	}

	if detectExecutableDependency(depSpec{Key: "ffmpeg", Name: "ffmpeg", ManagedPath: stagedFFmpeg, VersionArgs: []string{"-version"}, ParseVersion: ffmpegVersionFromLine}, true).Version == "" {
		return fmt.Errorf("бинарник ffmpeg скачан, но не запускается")
	}
	if commandVersionLine(stagedFFprobe, "-version") == "" {
		return fmt.Errorf("бинарник ffprobe скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{
		stagedFFmpeg:  env.FFmpegBin,
		stagedFFprobe: env.FFprobeBin,
	}); err != nil {
		return fmt.Errorf("replace ffmpeg binaries: %w", err)
	}
	return nil
}

func InstallNodeFor(env *Env, ctx context.Context, l Locale, ch chan<- FileProgress) error {
	url, filename, checksum, err := nodeDownloadAsset(env)
	if err != nil {
		return fmt.Errorf("node asset metadata: %w", err)
	}
	if err := os.MkdirAll(env.DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}

	tmp, err := os.MkdirTemp("", "node-*")
	if err != nil {
		return fmt.Errorf("временная директория: %w", err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, filename)
	if err := DownloadFileContext(env, ctx, url, archive, l, ch); err != nil {
		return fmt.Errorf("download node archive: %w", err)
	}
	if checksum != "" {
		if err := verifyFileSHA256(archive, checksum); err != nil {
			return fmt.Errorf("verify node checksum: %w", err)
		}
	}

	staging, err := os.MkdirTemp(env.DepsDir, ".node-*")
	if err != nil {
		return fmt.Errorf("временная директория установки: %w", err)
	}
	defer os.RemoveAll(staging)

	nodeName := binaryBaseName(env.NodeBin)
	stagedNode := filepath.Join(staging, nodeName)
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		if err := extractZipBinaries(ctx, archive, map[string]string{
			nodeName: stagedNode,
		}); err != nil {
			return fmt.Errorf("extract node zip: %w", err)
		}
	} else {
		if err := extractArchiveBinariesWithTar(ctx, archive, map[string]string{
			nodeName: stagedNode,
		}); err != nil {
			return fmt.Errorf("extract node archive: %w", err)
		}
	}

	if detectExecutableDependency(depSpec{Key: "node", Name: "node", ManagedPath: stagedNode, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}, true).Version == "" {
		return fmt.Errorf("бинарник node скачан, но не запускается")
	}
	if err := replaceInstalledBinaries(map[string]string{stagedNode: env.NodeBin}); err != nil {
		return fmt.Errorf("replace node binary: %w", err)
	}
	return nil
}

func ffmpegArchiveAsset() (string, string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", "", fmt.Errorf("detect platform: %w", err)
	}
	url := strings.TrimSpace(platform.FFmpegURL)
	if url == "" {
		return "", "", fmt.Errorf("ffmpeg asset URL is empty")
	}
	return url, filepath.Base(url), nil
}

func nodeDownloadAsset(env *Env) (string, string, string, error) {
	filename, checksum, err := nodeAssetFilename(env)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve node asset: %w", err)
	}
	return nodeLatestURL + filename, filename, checksum, nil
}

func ytdlpDownloadAsset(env *Env) (string, string, string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", "", "", fmt.Errorf("detect platform: %w", err)
	}
	asset := strings.TrimSpace(platform.YTDLPAsset)
	if asset == "" {
		return "", "", "", fmt.Errorf("yt-dlp asset name is empty")
	}
	checksum, err := ytdlpAssetChecksum(env, context.Background(), asset)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve yt-dlp checksum: %w", err)
	}
	return ytdlpBase + asset, asset, checksum, nil
}

func ytdlpAssetChecksum(env *Env, ctx context.Context, asset string) (string, error) {
	manifest, err := downloadText(env, ctx, ytdlpBase+"SHA2-256SUMS")
	if err != nil {
		return "", fmt.Errorf("yt-dlp checksum manifest: %w", err)
	}
	checksum, err := checksumFromManifest(manifest, asset)
	if err != nil {
		return "", fmt.Errorf("yt-dlp checksum for %s: %w", asset, err)
	}
	return checksum, nil
}

func nodeAssetFilename(env *Env) (string, string, error) {
	manifest, err := downloadText(env, context.Background(), nodeLatestURL+"SHASUMS256.txt")
	if err != nil {
		return "", "", fmt.Errorf("node manifest: %w", err)
	}

	suffix, err := nodeAssetSuffix()
	if err != nil {
		return "", "", fmt.Errorf("resolve node suffix: %w", err)
	}
	return nodeAssetFromManifest(manifest, suffix)
}

func scanChecksumManifest(manifest string, match func(asset string) bool) (string, string, error) {
	for _, line := range strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		asset := fields[len(fields)-1]
		if !match(asset) {
			continue
		}
		checksum, err := normalizeSHA256(fields[0])
		if err != nil {
			return "", "", fmt.Errorf("normalize checksum: %w", err)
		}
		return asset, checksum, nil
	}
	return "", "", nil
}

func nodeAssetFromManifest(manifest, suffix string) (string, string, error) {
	name, checksum, err := scanChecksumManifest(manifest, func(asset string) bool {
		return strings.HasSuffix(asset, suffix)
	})
	if err != nil {
		return "", "", fmt.Errorf("node checksum for %s: %w", name, err)
	}
	if name == "" {
		return "", "", fmt.Errorf("node asset with suffix %s not found", suffix)
	}
	return name, checksum, nil
}

func checksumFromManifest(manifest, name string) (string, error) {
	target := strings.TrimSpace(name)
	found, checksum, err := scanChecksumManifest(manifest, func(asset string) bool {
		return asset == target
	})
	if err != nil {
		return "", fmt.Errorf("scan checksum manifest: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("asset %s not found", name)
	}
	return checksum, nil
}

func nodeAssetSuffix() (string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", fmt.Errorf("detect platform: %w", err)
	}
	if platform.NodeAssetSuffix == "" {
		return "", fmt.Errorf("node asset suffix is empty")
	}
	return platform.NodeAssetSuffix, nil
}

func downloadText(env *Env, ctx context.Context, url string) (string, error) {
	ctx = resolveContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := newDownloadRequest(ctx, url)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := doSafeRequest(ctx, env.dlClient, req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if err := validateDownloadResponse(resp, url); err != nil {
		return "", fmt.Errorf("validate download response: %w", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	return string(data), nil
}

func normalizeSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("expected %d hex chars", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("decode hex string: %w", err)
	}
	return value, nil
}

func verifyFileSHA256(path, expected string) error {
	expected, err := normalizeSHA256(expected)
	if err != nil {
		return fmt.Errorf("normalize expected checksum: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return fmt.Errorf("read file for checksum: %w", err)
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
		return fmt.Errorf("list tar archive: %w", err)
	}
	selected, err := selectTarBinaryEntries(entries, targets)
	if err != nil {
		return fmt.Errorf("select tar entries: %w", err)
	}

	destDir, err := os.MkdirTemp(filepath.Dir(archive), "extract-*")
	if err != nil {
		return fmt.Errorf("временная директория распаковки: %w", err)
	}
	defer os.RemoveAll(destDir)

	if err := extractTarEntriesWithTar(ctx, archive, destDir, selected); err != nil {
		return fmt.Errorf("extract tar entries: %w", err)
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
			return nil, fmt.Errorf("validate archive path: %w", err)
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
		return fmt.Errorf("create extraction directory: %w", err)
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
	return fmt.Errorf("tar command failed: %w", err)
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
			return fmt.Errorf("copy extracted file: %w", err)
		}
		found[name] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk extracted directory: %w", err)
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
		return fmt.Errorf("stat extracted file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink from archive: %s", src)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular file from archive: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy file data: %w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("sync destination file: %w", err)
	}
	return out.Close()
}

func replaceInstalledBinaries(paths map[string]string) error {
	return replaceFilesWithBackup(paths)
}

func binaryBaseName(path string) string {
	return filepath.Base(strings.TrimSpace(path))
}
