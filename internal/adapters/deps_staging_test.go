package adapters

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"volvid/internal/core"
)

func TestRequireStagedBinaryProbesManagedPath(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not in PATH")
	}
	staged := filepath.Join(t.TempDir(), "staged-bin")
	if err := os.WriteFile(staged, []byte("not a binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	spec := depSpec{
		Key:         "ytdlp",
		Name:        "yt-dlp",
		LookNames:   []string{"go"},
		ManagedPath: staged,
		VersionArgs: []string{"version"},
	}
	if err := requireStagedBinary(context.Background(), spec); err == nil {
		t.Fatal("staged junk must fail validation even when a PATH lookalike works")
	}

	good := spec
	good.ManagedPath = goBin
	if err := requireStagedBinary(context.Background(), good); err != nil {
		t.Fatalf("working staged binary must pass validation: %v", err)
	}
}

func TestRequireStagedBinaryEmptyPath(t *testing.T) {
	if err := requireStagedBinary(context.Background(), depSpec{Name: "yt-dlp"}); err == nil {
		t.Fatal("empty staging path must fail validation")
	}
}

func TestProbeVersionMissingBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-bin")
	if got := probeVersion(context.Background(), missing, "--version"); got != "" {
		t.Fatalf("expected empty version for missing binary, got %q", got)
	}
}

func TestEnrichDepsSkipsUnavailable(t *testing.T) {
	env := testEnv(t)
	zeros := core.DependencyInfo{Available: false}
	deps := core.CheckDepsResult{YTDLP: zeros, FFmpeg: zeros, Node: zeros}

	got := EnrichDeps(env, context.Background(), deps)
	for _, dep := range got.Dependencies() {
		if dep.Version != "" {
			t.Fatalf("unavailable dependency must not be probed: %+v", dep)
		}
	}
}
