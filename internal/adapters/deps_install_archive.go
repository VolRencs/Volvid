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
	found := make(map[string]bool, len(targets))
	for _, name := range slices.Sorted(maps.Keys(targets)) {
		if entry := selected[name]; entry != "" {
			found[name] = true
			out = append(out, entry)
		}
	}
	if err := requireTargetsFound(targets, found); err != nil {
		return nil, err
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
		return fmt.Errorf("tar is required to extract this archive: %w", err)
	}
	if line := firstNonEmptyLine(string(output)); line != "" {
		return fmt.Errorf("%w: %s", err, line)
	}
	return fmt.Errorf("tar command failed: %w", err)
}
func copyExtractedBinaries(root string, targets map[string]string) error {
	found := make(map[string]bool, len(targets))

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Отказ от symlink-атак: любой symlink внутри распаковки удаляем,
		// наружу ничего не копируем. Внешний tar распаковывает только
		// allowlist-entries в свежий пустой destDir, поэтому escape возможен
		// только через symlink-entry — такие файлы отклоняем здесь.
		if d.Type()&os.ModeSymlink != 0 {
			_ = os.Remove(path)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		name := filepath.Base(path)
		dest, ok := targets[name]
		if !ok {
			return nil
		}
		if !d.Type().IsRegular() {
			_ = os.Remove(path)
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

	return requireTargetsFound(targets, found)
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

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	// Atomic staged write (was: direct O_TRUNC write exposed to umask races).
	if err := writeStagedFile(in, dest, 0o755, -1); err != nil {
		return fmt.Errorf("copy extracted file: %w", err)
	}
	return nil
}
