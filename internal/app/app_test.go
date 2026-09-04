package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testEnv(t *testing.T) *Env {
	t.Helper()
	t.Setenv("VOLVID_CONFIG_DIR", t.TempDir())
	t.Setenv("VOLVID_DATA_DIR", t.TempDir())
	env := NewEnv()
	if env.ConfigDir == "" || env.DepsDir == "" {
		t.Fatal("expected NewEnv to resolve config and deps dirs")
	}
	if env.YtdlpBin == "" || env.FFmpegBin == "" {
		t.Fatal("expected NewEnv to resolve binary paths")
	}
	return env
}

func TestNewEnvBinaryPathsPerPlatform(t *testing.T) {
	env := testEnv(t)
	want := "yt-dlp"
	if env.IsWindows {
		want = "yt-dlp.exe"
	}
	if filepath.Base(env.YtdlpBin) != want {
		t.Fatalf("unexpected yt-dlp path %q", env.YtdlpBin)
	}
}

func TestDownloadsDirRoundTripConcurrent(t *testing.T) {
	env := testEnv(t)

	dir := t.TempDir()
	if err := SetDownloadsDir(env, dir); err != nil {
		t.Fatalf("SetDownloadsDir: %v", err)
	}
	if got := env.DownloadsDir(); got != dir {
		t.Fatalf("expected %q, got %q", dir, got)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_ = env.DownloadsDir()
			}
		}()
	}
	wg.Wait()
}

func newBareEnv() *Env {
	return &Env{
		depsCache:        newFlightCache[struct{}, CheckDepsResult](),
		runtimeDepsCache: newFlightCache[struct{}, CheckDepsResult](),
		probeCache:       newFlightCache[string, *MediaProbe](),
	}
}

func TestScanQualityChoicesCancelledContext(t *testing.T) {
	env := newBareEnv()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	choices, err := scanQualityChoicesContext(env, ctx, []string{"https://youtu.be/dQw4w9WgXcQ"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (choices=%v)", err, choices)
	}
}

func TestScanQualityChoicesEmptyInput(t *testing.T) {
	env := newBareEnv()
	if _, err := scanQualityChoicesContext(env, context.Background(), nil); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestProbeOnBareEnvDoesNotPanic(t *testing.T) {
	env := newBareEnv()
	target, err := ParseTarget("https://youtu.be/dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	ProbeMediaDurationContext(env, context.Background(), target)
}

func TestScanChecksumManifest(t *testing.T) {
	const (
		shaYtdlp = "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae0"
		shaNode  = "b3a8e0e1f9ab1bfe3a36f231f676f78bb30a519d2b21e6c530c7eee9e63c6ab2"
	)
	manifest := strings.Join([]string{
		"# comment",
		shaYtdlp + "  yt-dlp_linux",
		"", // skipped: not enough fields
		shaNode + "  node-v24-linux-x64.tar.gz",
	}, "\n")

	checksum, err := checksumFromManifest(manifest, "yt-dlp_linux")
	if err != nil || checksum != shaYtdlp {
		t.Fatalf("checksumFromManifest = %q, %v", checksum, err)
	}

	name, checksum, err := nodeAssetFromManifest(manifest, "-linux-x64.tar.gz")
	if err != nil || name != "node-v24-linux-x64.tar.gz" || checksum != shaNode {
		t.Fatalf("nodeAssetFromManifest = %q/%q, %v", name, checksum, err)
	}

	if _, err := checksumFromManifest(manifest, "missing"); err == nil {
		t.Fatal("expected error for missing asset")
	}
	if _, _, err := nodeAssetFromManifest(manifest, "-win-x64.zip"); err == nil {
		t.Fatal("expected error for missing suffix")
	}
}

func TestReplaceFilesWithBackup(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	src := filepath.Join(srcDir, "new.bin")
	dest := filepath.Join(destDir, "old.bin")
	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceFilesWithBackup(map[string]string{src: dest}); err != nil {
		t.Fatalf("replaceFilesWithBackup: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "new" {
		t.Fatalf("dest content = %q, %v", data, err)
	}
	matches, _ := filepath.Glob(dest + ".bak.*")
	if len(matches) != 0 {
		t.Fatalf("expected backup cleanup, found %v", matches)
	}
}

func TestReplaceFilesWithBackupRollbackOnMissingSource(t *testing.T) {
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "old.bin")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(destDir, "does-not-exist.bin")
	err := replaceFilesWithBackup(map[string]string{missing: dest})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	data, readErr := os.ReadFile(dest)
	if readErr != nil || string(data) != "old" {
		t.Fatalf("rollback failed: dest = %q, %v", data, readErr)
	}
}

func TestFmtSpeedForLocaleSuffix(t *testing.T) {
	if got := fmtSpeedFor(2048, LocaleEN); !strings.HasSuffix(got, "/s") {
		t.Fatalf("EN speed %q lacks /s", got)
	}
	if got := fmtSpeedFor(2048, LocaleRU); !strings.HasSuffix(got, "/с") {
		t.Fatalf("RU speed %q lacks /с", got)
	}
}

func TestPrepareDownloadRequestRejectsUnknownTarget(t *testing.T) {
	env := testEnv(t)
	req := DownloadRequest{Profile: DefaultVideoProfile(LocaleEN)}
	if _, err := PrepareDownloadRequest(env, req); err == nil {
		t.Fatal("expected error for missing target")
	}
}

func TestQualityChainAtClones(t *testing.T) {
	chain := qualityChainAt(0)
	if len(chain) == 0 {
		t.Fatal("expected non-empty chain")
	}
	chain[0] = "mutated"
	if qualityChains[0][0] == "mutated" {
		t.Fatal("qualityChainAt must return a clone")
	}
	if qualityChainAt(-1) != nil || qualityChainAt(len(qualityChains)) != nil {
		t.Fatal("expected nil for out-of-range index")
	}
}
