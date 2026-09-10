package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"volvid/internal/core"
)

func InstallDependencyFor(env *Env, ctx context.Context, key string, l core.Locale, ch chan<- core.FileProgress) error {
	var err error
	switch strings.TrimSpace(key) {
	case "ytdlp":
		err = installYtDlpFor(env, ctx, l, ch)
	case "ffmpeg":
		err = installFFmpegFor(env, ctx, l, ch)
	case "node":
		err = installNodeFor(env, ctx, l, ch)
	default:
		return fmt.Errorf("unsupported dependency: %s", key)
	}
	if err != nil {
		return fmt.Errorf("install dependency %s: %w", key, err)
	}
	invalidateDepsCache(env)
	return nil
}
func ensureDepsDir(env *Env) error {
	if _, err := prepareDir(env.DepsDir); err != nil {
		return fmt.Errorf("create DepsDir: %w", err)
	}
	return nil
}
func requireStagedBinary(ctx context.Context, name string, spec depSpec) error {
	if detectExecutableDependency(ctx, spec, true).Version == "" {
		return fmt.Errorf("binary %s downloaded but does not run", name)
	}
	return nil
}
func requireTargetsFound(targets map[string]string, found map[string]bool) error {
	for _, name := range slices.Sorted(maps.Keys(targets)) {
		if !found[name] {
			return fmt.Errorf("%s not found in archive", name)
		}
	}
	return nil
}
func installYtDlpFor(env *Env, ctx context.Context, l core.Locale, ch chan<- core.FileProgress) error {
	if err := ensureDepsDir(env); err != nil {
		return err
	}

	url, _, checksum, err := ytdlpDownloadAsset(ctx, env)
	if err != nil {
		return fmt.Errorf("yt-dlp asset metadata: %w", err)
	}
	staging, err := os.MkdirTemp(env.DepsDir, ".ytdlp-*")
	if err != nil {
		return fmt.Errorf("create install staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	stagedYtdlp := filepath.Join(staging, binaryBaseName(env.YtdlpBin))
	if err := downloadFileContext(env, ctx, url, stagedYtdlp, l, ch); err != nil {
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
	if err := requireStagedBinary(ctx, "yt-dlp", depSpec{Key: "ytdlp", Name: "yt-dlp", ManagedPath: stagedYtdlp, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}); err != nil {
		return err
	}
	if err := replaceFilesWithBackup(map[string]string{stagedYtdlp: env.YtdlpBin}); err != nil {
		return fmt.Errorf("replace yt-dlp binary: %w", err)
	}
	return nil
}
func installFFmpegFor(env *Env, ctx context.Context, l core.Locale, ch chan<- core.FileProgress) error {
	if err := ensureDepsDir(env); err != nil {
		return err
	}

	stagingParent, err := stageDownloadDir(env)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingParent)

	archiveURL, archiveName, err := ffmpegArchiveAsset()
	if err != nil {
		return fmt.Errorf("ffmpeg asset metadata: %w", err)
	}
	archive := filepath.Join(stagingParent, archiveName)
	if err := downloadFileContext(env, ctx, archiveURL, archive, l, ch); err != nil {
		return fmt.Errorf("download ffmpeg archive: %w", err)
	}
	// BtbN does not publish a machine-readable SHA manifest like yt-dlp/node,
	// so the checksum is verified only when set explicitly. This still closes
	// the supply chain for users who pin a hash.
	if expected := strings.TrimSpace(os.Getenv("VOLVID_FFMPEG_SHA256")); expected != "" {
		if err := verifyFileSHA256(archive, expected); err != nil {
			return fmt.Errorf("verify ffmpeg checksum: %w", err)
		}
	}

	staging, err := os.MkdirTemp(env.DepsDir, ".ffmpeg-*")
	if err != nil {
		return fmt.Errorf("create install staging dir: %w", err)
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

	if err := extractBinaries(ctx, archive, targets); err != nil {
		return fmt.Errorf("extract ffmpeg archive: %w", err)
	}

	if err := requireStagedBinary(ctx, "ffmpeg", depSpec{Key: "ffmpeg", Name: "ffmpeg", ManagedPath: stagedFFmpeg, VersionArgs: []string{"-version"}, ParseVersion: ffmpegVersionFromLine}); err != nil {
		return err
	}
	if err := requireStagedBinary(ctx, "ffprobe", depSpec{ManagedPath: stagedFFprobe, VersionArgs: []string{"-version"}, ParseVersion: firstNonEmptyLine}); err != nil {
		return err
	}
	if err := replaceFilesWithBackup(map[string]string{
		stagedFFmpeg:  env.FFmpegBin,
		stagedFFprobe: env.FFprobeBin,
	}); err != nil {
		return fmt.Errorf("replace ffmpeg binaries: %w", err)
	}
	return nil
}
func installNodeFor(env *Env, ctx context.Context, l core.Locale, ch chan<- core.FileProgress) error {
	url, filename, checksum, err := nodeDownloadAsset(ctx, env)
	if err != nil {
		return fmt.Errorf("node asset metadata: %w", err)
	}
	if err := ensureDepsDir(env); err != nil {
		return err
	}

	stagingParent, err := stageDownloadDir(env)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingParent)

	archive := filepath.Join(stagingParent, filename)
	if err := downloadFileContext(env, ctx, url, archive, l, ch); err != nil {
		return fmt.Errorf("download node archive: %w", err)
	}
	if checksum != "" {
		if err := verifyFileSHA256(archive, checksum); err != nil {
			return fmt.Errorf("verify node checksum: %w", err)
		}
	}

	staging, err := os.MkdirTemp(env.DepsDir, ".node-*")
	if err != nil {
		return fmt.Errorf("create install staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	nodeName := binaryBaseName(env.NodeBin)
	stagedNode := filepath.Join(staging, nodeName)
	if err := extractBinaries(ctx, archive, map[string]string{nodeName: stagedNode}); err != nil {
		return fmt.Errorf("extract node archive: %w", err)
	}

	if err := requireStagedBinary(ctx, "node", depSpec{Key: "node", Name: "node", ManagedPath: stagedNode, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}); err != nil {
		return err
	}
	if err := replaceFilesWithBackup(map[string]string{stagedNode: env.NodeBin}); err != nil {
		return fmt.Errorf("replace node binary: %w", err)
	}
	return nil
}
func extractBinaries(ctx context.Context, archive string, targets map[string]string) error {
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return extractZipBinaries(archive, targets)
	}
	return extractArchiveBinariesWithTar(ctx, archive, targets)
}
func stageDownloadDir(env *Env) (string, error) {
	dir, err := os.MkdirTemp(env.DepsDir, ".dl-*")
	if err != nil {
		return "", fmt.Errorf("create download staging dir: %w", err)
	}
	return dir, nil
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
func nodeDownloadAsset(ctx context.Context, env *Env) (string, string, string, error) {
	filename, checksum, err := nodeAssetFilename(ctx, env)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve node asset: %w", err)
	}
	return nodeLatestURL + filename, filename, checksum, nil
}
func ytdlpDownloadAsset(ctx context.Context, env *Env) (string, string, string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", "", "", fmt.Errorf("detect platform: %w", err)
	}
	asset := strings.TrimSpace(platform.YTDLPAsset)
	if asset == "" {
		return "", "", "", fmt.Errorf("yt-dlp asset name is empty")
	}
	checksum, err := ytdlpAssetChecksum(env, ctx, asset)
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
func nodeAssetFilename(ctx context.Context, env *Env) (string, string, error) {
	manifest, err := downloadText(env, ctx, nodeLatestURL+"SHASUMS256.txt")
	if err != nil {
		return "", "", fmt.Errorf("node manifest: %w", err)
	}

	suffix, err := nodeAssetSuffix()
	if err != nil {
		return "", "", fmt.Errorf("resolve node suffix: %w", err)
	}
	return nodeAssetFromManifest(manifest, suffix)
}
func downloadText(env *Env, ctx context.Context, url string) (string, error) {
	ctx = resolveContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, manifestFetchTimeout)
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

	data, err := io.ReadAll(io.LimitReader(resp.Body, manifestMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > manifestMaxBytes {
		return "", fmt.Errorf("manifest exceeds %d bytes", manifestMaxBytes)
	}
	return string(data), nil
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
	name = filepath.Base(strings.TrimSpace(name))
	if !validAssetFilename(name) {
		return "", "", fmt.Errorf("node asset filename is invalid: %q", name)
	}
	return name, checksum, nil
}

func validAssetFilename(name string) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '+':
		default:
			return false
		}
	}
	return true
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
