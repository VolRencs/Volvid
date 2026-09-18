package adapters

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"volvid/internal/core"
)

// Download artifact suffixes: yt-dlp --part fragments, our transcode
// tmp files and replacement backups. Only exact artifacts are removed,
// never stem heuristics (so user video.particular.mp4 files survive).
const (
	artifactPartSuffix      = ".part"
	artifactYtdlSuffix      = ".ytdl"
	artifactPartFragPrefix  = ".part-"
	artifactTranscodePrefix = ".transcode-"
	artifactBackupPrefix    = ".bak-"
)

func sendUpdate(ctx context.Context, ch chan<- core.DlUpdate, u core.DlUpdate) bool {
	if ch == nil {
		return false
	}
	if ctx == nil {
		select {
		case ch <- u:
			return true
		default:
			return false
		}
	}
	select {
	case ch <- u:
		return true
	case <-ctx.Done():
		return false
	}
}

type downloadCleanup struct {
	mu    sync.Mutex
	root  string
	paths map[string]struct{}
}

func newDownloadCleanup() *downloadCleanup {
	return &downloadCleanup{paths: map[string]struct{}{}}
}
func (c *downloadCleanup) setRoot(root string) {
	if c == nil {
		return
	}
	root = cleanAbsPath(root)
	if root == "" {
		return
	}
	c.mu.Lock()
	c.root = root
	c.mu.Unlock()
}
func normalizeCleanupPath(c *downloadCleanup, path string) (string, bool) {
	if c == nil {
		return "", false
	}
	abs := cleanAbsPath(path)
	if abs == "" || !filepath.IsAbs(abs) {
		return "", false
	}
	c.mu.Lock()
	root := c.root
	c.mu.Unlock()
	if root != "" {
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "", false
		}
	}
	return abs, true
}

func (c *downloadCleanup) add(path string) {
	abs, ok := normalizeCleanupPath(c, path)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths[abs] = struct{}{}
}
func (c *downloadCleanup) forget(path string) {
	abs, ok := normalizeCleanupPath(c, path)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.paths, abs)
}
func (c *downloadCleanup) cleanup() {
	if c == nil {
		return
	}
	c.mu.Lock()
	rootPath := c.root
	paths := make([]string, 0, len(c.paths))
	for p := range c.paths {
		paths = append(paths, p)
	}
	c.mu.Unlock()
	if rootPath == "" {
		return
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return
	}
	defer root.Close()

	byDir := make(map[string][]string)
	for _, p := range paths {
		rel, ok := cleanupRelativePath(rootPath, p)
		if !ok {
			continue
		}
		dir := filepath.Dir(rel)
		byDir[dir] = append(byDir[dir], rel)
	}
	for dir, rels := range byDir {
		for _, rel := range rels {
			deleteDownloadArtifacts(root, rel)
		}
		removeMatchingArtifacts(root, dir, rels)
	}
}
func cleanupRelativePath(rootPath, path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	rel, err := filepath.Rel(rootPath, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return rel, true
}
func deleteDownloadArtifacts(root *os.Root, rel string) {
	_ = root.Remove(rel)
	_ = root.Remove(rel + artifactPartSuffix)
	_ = root.Remove(rel + artifactYtdlSuffix)
}

// Subtitle sidecar extensions yt-dlp may leave next to an embedded video.
var embeddedSubtitleExts = []string{".srt", ".vtt"}

// cleanupSubtitleSidecars removes "<video base>.<lang>.srt/vtt" files left
// by yt-dlp after embedding. Only exact "<base>.<lang><ext>" matches are
// removed, so unrelated user files are never touched.
func cleanupSubtitleSidecars(videoPath string, langs []string) {
	videoPath = strings.TrimSpace(videoPath)
	if videoPath == "" || len(langs) == 0 {
		return
	}
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	if base == "" || base == "." {
		return
	}
	names := make([]string, 0, len(langs)*len(embeddedSubtitleExts))
	if slices.Contains(langs, "all") {
		matches, _ := filepath.Glob(filepath.Join(dir, base+".*.srt"))
		for _, match := range matches {
			names = append(names, filepath.Base(match))
		}
		matches, _ = filepath.Glob(filepath.Join(dir, base+".*.vtt"))
		for _, match := range matches {
			names = append(names, filepath.Base(match))
		}
	} else {
		for _, lang := range langs {
			lang = strings.TrimSpace(lang)
			if lang == "" {
				continue
			}
			for _, ext := range embeddedSubtitleExts {
				names = append(names, base+"."+lang+ext)
			}
		}
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		// Never delete the video itself or anything outside plain files.
		if name == filepath.Base(videoPath) {
			continue
		}
		full := filepath.Join(dir, name)
		if info, err := os.Lstat(full); err != nil || !info.Mode().IsRegular() {
			continue
		}
		_ = os.Remove(full)
	}
}
func removeMatchingArtifacts(root *os.Root, relDir string, rels []string) {
	bases := make([]string, 0, len(rels))
	for _, rel := range rels {
		if base := filepath.Base(rel); base != "" && base != "." {
			bases = append(bases, base)
		}
	}
	dir, err := root.Open(relDir)
	if err != nil {
		return
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, base := range bases {
			if strings.HasPrefix(name, base+artifactPartFragPrefix) ||
				strings.HasPrefix(name, "."+base+artifactTranscodePrefix) ||
				strings.HasPrefix(name, "."+base+artifactBackupPrefix) {
				_ = root.Remove(filepath.Join(relDir, name))
				break
			}
		}
	}
}
