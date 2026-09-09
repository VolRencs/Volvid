package adapters

import (
	"context"
	"os"
	"path/filepath"
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
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	abs = filepath.Clean(abs)
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
	paths := make([]string, 0, len(c.paths))
	for p := range c.paths {
		paths = append(paths, p)
	}
	c.mu.Unlock()
	byDir := make(map[string][]string)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dir := filepath.Dir(p)
		byDir[dir] = append(byDir[dir], p)
	}
	for dir, ps := range byDir {
		for _, p := range ps {
			deleteDownloadArtifacts(p)
		}
		removeMatchingArtifacts(dir, ps)
	}
}
func deleteDownloadArtifacts(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + artifactPartSuffix)
	_ = os.Remove(path + artifactYtdlSuffix)
}
func removeMatchingArtifacts(dir string, paths []string) {
	bases := make([]string, 0, len(paths))
	for _, p := range paths {
		if base := filepath.Base(p); base != "" {
			bases = append(bases, base)
		}
	}
	entries, err := os.ReadDir(dir)
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
				_ = os.Remove(filepath.Join(dir, name))
				break
			}
		}
	}
}
