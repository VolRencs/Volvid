package adapters

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

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

	perm := zf.FileInfo().Mode().Perm() & 0o755
	if perm == 0 {
		perm = 0o755
	}
	if err := writeStagedFile(rc, dest, perm, maxExtractedFileSize); err != nil {
		return fmt.Errorf("extract zip data: %w", err)
	}
	return nil
}
func extractZipBinaries(archive string, targets map[string]string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
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
			return fmt.Errorf("extract %s: %w", name, err)
		}
		found[name] = true
	}

	return requireTargetsFound(targets, found)
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
		return fmt.Errorf("create extract temp dir: %w", err)
	}
	defer os.RemoveAll(destDir)

	if err := extractTarEntriesWithTar(ctx, archive, destDir, slices.Sorted(maps.Values(selected))); err != nil {
		return fmt.Errorf("extract tar entries: %w", err)
	}
	return copyExtractedBinaries(destDir, selected, targets)
}
func listTarArchive(ctx context.Context, archive string) ([]string, error) {
	output, err := commandCombinedOutput(resolveContext(ctx), tarCommandTimeout, "tar", "-tf", archive)
	if err != nil {
		return nil, tarCommandError(err, output)
	}
	return parseTarListOutput(string(output))
}
func parseTarListOutput(output string) ([]string, error) {
	entries := make([]string, 0, strings.Count(output, "\n")+1)
	for line := range strings.SplitSeq(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
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
func selectTarBinaryEntries(entries []string, targets map[string]string) (map[string]string, error) {
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

	found := make(map[string]bool, len(targets))
	for name := range targets {
		if selected[name] != "" {
			found[name] = true
		}
	}
	if err := requireTargetsFound(targets, found); err != nil {
		return nil, err
	}
	return selected, nil
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
		return fmt.Errorf("tar is required to extract this archive: %w", err)
	}
	if line := firstNonEmptyLine(string(output)); line != "" {
		return fmt.Errorf("%w: %s", err, line)
	}
	return fmt.Errorf("tar command failed: %w", err)
}
func copyExtractedBinaries(extractDir string, selected, targets map[string]string) error {
	root, err := os.OpenRoot(extractDir)
	if err != nil {
		return fmt.Errorf("open extraction root: %w", err)
	}
	defer root.Close()

	found := make(map[string]bool, len(targets))
	for _, name := range slices.Sorted(maps.Keys(targets)) {
		entry := filepath.FromSlash(selected[name])
		info, err := root.Lstat(entry)
		if err != nil {
			return fmt.Errorf("stat extracted %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular file from archive: %s", selected[name])
		}

		src, err := root.Open(entry)
		if err != nil {
			return fmt.Errorf("open extracted %s: %w", name, err)
		}
		copyErr := writeStagedFile(src, targets[name], 0o755, -1)
		_ = src.Close()
		if copyErr != nil {
			return fmt.Errorf("copy extracted %s: %w", name, copyErr)
		}
		found[name] = true
	}

	return requireTargetsFound(targets, found)
}
