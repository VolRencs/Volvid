package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"volvid/internal/core"
)

func rawMap(langs ...string) map[string]json.RawMessage {
	m := make(map[string]json.RawMessage, len(langs))
	for _, lang := range langs {
		m[lang] = json.RawMessage(`[{"ext":"vtt"}]`)
	}
	return m
}

func TestSubtitleTracksFromPayload(t *testing.T) {
	tracks := subtitleTracksFromPayload(rawMap("ru", "en"), rawMap("en", "uk"))
	want := []core.SubtitleTrack{{Lang: "en"}, {Lang: "ru"}, {Lang: "uk", Auto: true}}
	if len(tracks) != len(want) {
		t.Fatalf("got %v, want %v", tracks, want)
	}
	for i := range want {
		if tracks[i] != want[i] {
			t.Fatalf("got %v, want %v", tracks, want)
		}
	}
}

func TestSubtitleDownloadArgs(t *testing.T) {
	off := core.OutputProfile{Mode: core.ModeVideo}
	if args := subtitleDownloadArgs(off); len(args) != 0 {
		t.Fatalf("expected no args when off, got %v", args)
	}
	on := core.OutputProfile{Mode: core.ModeVideo, SubMode: core.SubEmbed, SubLangs: []string{"en", "ru"}}
	args := subtitleDownloadArgs(on)
	joined := ""
	for _, a := range args {
		joined += a + " "
	}
	for _, want := range []string{"--write-subs", "--write-auto-subs", "--sub-langs", "en,ru", "--sub-format", "srt", "--convert-subs", "srt", "--embed-subs"} {
		found := false
		for _, a := range args {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in %v (joined %q)", want, args, joined)
		}
	}
	// Audio mode never embeds even if the fields are set.
	audio := core.OutputProfile{Mode: core.ModeAudio, SubMode: core.SubEmbed, SubLangs: []string{"en"}}
	if args := subtitleDownloadArgs(audio); len(args) != 0 {
		t.Fatalf("expected no args for audio mode, got %v", args)
	}
}

func TestSubtitleTranscodeArgs(t *testing.T) {
	off := core.OutputProfile{Mode: core.ModeVideo}
	args := subtitleTranscodeArgs(off)
	if len(args) != 1 || args[0] != "-sn" {
		t.Fatalf("expected [-sn] when off, got %v", args)
	}
	mp4 := core.OutputProfile{Mode: core.ModeVideo, VideoContainer: "mp4", SubMode: core.SubEmbed, SubLangs: []string{"en"}}
	args = subtitleTranscodeArgs(mp4)
	want := []string{"-map", "0:s?", "-c:s", "mov_text"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %v, want %v", args, want)
		}
	}
	mkv := core.OutputProfile{Mode: core.ModeVideo, VideoContainer: "mkv", SubMode: core.SubEmbed, SubLangs: []string{"en"}}
	args = subtitleTranscodeArgs(mkv)
	if args[3] != "srt" {
		t.Fatalf("expected srt codec for mkv, got %v", args)
	}
}

func TestCleanupSubtitleSidecars(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sidecars := []string{"video.en.srt", "video.ru.srt", "video.en.vtt"}
	keep := []string{"video.mp4", "video.particular.mp4", "other.en.srt", "notes.txt"}
	for _, name := range append(append([]string{}, sidecars...), keep...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cleanupSubtitleSidecars(video, []string{"en", "ru"})

	for _, name := range sidecars {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed", name)
		}
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s kept, err %v", name, err)
		}
	}
}

func TestCleanupSubtitleSidecarsEmpty(t *testing.T) {
	// No langs or empty path must be a safe no-op.
	cleanupSubtitleSidecars("", []string{"en"})
	cleanupSubtitleSidecars(filepath.Join(t.TempDir(), "video.mp4"), nil)
}
