package adapters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"volvid/internal/core"
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
func (c *downloadCleanup) add(path string) {
	if c == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != "" {
		rel, err := filepath.Rel(c.root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}
	}
	c.paths[abs] = struct{}{}
}
func (c *downloadCleanup) forget(path string) {
	if c == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
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
	_ = os.Remove(path + ".part")
	_ = os.Remove(path + ".ytdl")
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
			// Только точные артефакты: yt-dlp --part фрагменты,
			// наши tmp транскода и backup замены. Без stem-эвристик,
			// чтобы не сносить пользовательские video.particular.mp4.
			if strings.HasPrefix(name, base+".part-") ||
				strings.HasPrefix(name, "."+base+".transcode-") ||
				strings.HasPrefix(name, "."+base+".bak-") {
				_ = os.Remove(filepath.Join(dir, name))
				break
			}
		}
	}
}
