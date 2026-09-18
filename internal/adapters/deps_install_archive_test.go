package adapters

import (
	"archive/tar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func makeTar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractArchiveBinariesWithTar(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}
	archive := filepath.Join(t.TempDir(), "tool.tar")
	makeTar(t, archive, map[string]string{
		"pkg/bin/tool":  "TOOL",
		"pkg/bin/other": "OTHER",
	})

	dest := filepath.Join(t.TempDir(), "tool")
	if err := extractArchiveBinariesWithTar(t.Context(), archive, map[string]string{"tool": dest}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "TOOL" {
		t.Fatalf("dest content = %q, %v", data, err)
	}
}

func TestExtractArchiveBinariesWithTarMissingTarget(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}
	archive := filepath.Join(t.TempDir(), "tool.tar")
	makeTar(t, archive, map[string]string{"pkg/bin/other": "OTHER"})

	dest := filepath.Join(t.TempDir(), "tool")
	if err := extractArchiveBinariesWithTar(t.Context(), archive, map[string]string{"tool": dest}); err == nil {
		t.Fatal("expected error for a target missing from the archive")
	}
}

func TestExtractArchiveBinariesWithTarRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink extraction needs privileges on Windows")
	}
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}

	archive := filepath.Join(t.TempDir(), "tool.tar")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	if err := tw.WriteHeader(&tar.Header{Name: "pkg/bin/tool", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "tool")
	if err := extractArchiveBinariesWithTar(t.Context(), archive, map[string]string{"tool": dest}); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
}
