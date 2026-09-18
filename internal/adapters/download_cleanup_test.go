package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadCleanupRemovesTrackedArtifacts(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	tracked := []string{
		video,
		video + ".part",
		video + ".ytdl",
		filepath.Join(dir, "video.mp4.part-Frag1"),
		filepath.Join(dir, ".video.mp4.transcode-abc.mkv"),
		filepath.Join(dir, ".video.mp4.bak-abc"),
	}
	for _, p := range tracked {
		writeTestFile(t, p)
	}
	keep := filepath.Join(dir, "keep.txt")
	writeTestFile(t, keep)

	c := newDownloadCleanup()
	c.setRoot(dir)
	c.add(video)
	c.cleanup()

	for _, p := range tracked {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s must be removed, stat err = %v", p, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated file must survive: %v", err)
	}
}

func TestDownloadCleanupForgetKeepsFile(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	writeTestFile(t, video)

	c := newDownloadCleanup()
	c.setRoot(dir)
	c.add(video)
	c.forget(video)
	c.cleanup()

	if _, err := os.Stat(video); err != nil {
		t.Fatalf("forgotten file must survive: %v", err)
	}
}

func TestDownloadCleanupWithoutRootIsNoop(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	writeTestFile(t, video)

	c := newDownloadCleanup()
	c.add(video)
	c.cleanup()

	if _, err := os.Stat(video); err != nil {
		t.Fatalf("file must survive without a root: %v", err)
	}
}

func TestDownloadCleanupIgnoresOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	writeTestFile(t, outside)

	c := newDownloadCleanup()
	c.setRoot(root)
	c.add(outside)
	c.cleanup()

	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside the root must survive: %v", err)
	}
}
