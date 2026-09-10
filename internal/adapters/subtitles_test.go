package adapters

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"volvid/internal/core"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func findBinary(name string) (string, error) {
	return exec.LookPath(name)
}

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

func TestAudioTracksFromFormats(t *testing.T) {
	formats := []core.MediaFormat{
		{Height: 1080, VCodec: "avc1", ACodec: "none"},
		{Height: 0, VCodec: "none", ACodec: "mp4a", Language: "en"},
		{Height: 0, VCodec: "none", ACodec: "mp4a", Language: "ru"},
		{Height: 0, VCodec: "none", ACodec: "mp4a", Language: "ru"},
		{Height: 0, VCodec: "none", ACodec: "mp4a"},
		{Height: 720, VCodec: "avc1", ACodec: "mp4a", Language: "en"},
	}
	tracks := audioTracksFromFormats(formats)
	if len(tracks) != 2 || tracks[0].Lang != "en" || tracks[1].Lang != "ru" {
		t.Fatalf("got %v, want [en ru]", tracks)
	}
	if got := audioTracksFromFormats(nil); len(got) != 0 {
		t.Fatalf("expected no tracks, got %v", got)
	}
}

func TestApplyAudioTrackSelector(t *testing.T) {
	profile := core.OutputProfile{Mode: core.ModeVideo, AudioLangs: []string{"en", "ru"}}
	format := "bestvideo[height=1080]+bestaudio/best[height=1080]"
	want := "bestvideo[height=1080]+bestaudio[language=en]+bestaudio[language=ru]/best[height=1080]"
	if got := applyAudioTrackSelector(format, profile); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	off := core.OutputProfile{Mode: core.ModeVideo}
	if got := applyAudioTrackSelector(format, off); got != format {
		t.Fatalf("expected unchanged format, got %q", got)
	}
	if got := applyAudioTrackSelector("best", profile); got != "best" {
		t.Fatalf("expected unchanged format without bestaudio, got %q", got)
	}
}

func TestVideoModeArgsAudioMultistreams(t *testing.T) {
	multi := core.OutputProfile{Mode: core.ModeVideo, AudioLangs: []string{"en", "ru"}}
	args := videoModeArgs(multi, "bestvideo+bestaudio/best")
	found := false
	for _, a := range args {
		if a == "--audio-multistreams" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --audio-multistreams in %v", args)
	}
	single := core.OutputProfile{Mode: core.ModeVideo, AudioLangs: []string{"en"}}
	for _, a := range videoModeArgs(single, "bestvideo+bestaudio/best") {
		if a == "--audio-multistreams" {
			t.Fatalf("unexpected --audio-multistreams for a single track")
		}
	}
	off := core.OutputProfile{Mode: core.ModeVideo}
	for _, a := range videoModeArgs(off, "bestvideo+bestaudio/best") {
		if a == "--audio-multistreams" {
			t.Fatalf("unexpected --audio-multistreams when off")
		}
	}
}

func TestAudioLangISO(t *testing.T) {
	cases := map[string]string{
		"ru": "rus", "en": "eng", "en-US": "eng", "zh-Hans": "zho",
		"de": "deu", "pt-BR": "por", "es": "spa",
	}
	for in, want := range cases {
		got, ok := audioLangISO(in)
		if !ok || got != want {
			t.Fatalf("audioLangISO(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "all", "e", "ru!", "1234"} {
		if _, ok := audioLangISO(bad); ok {
			t.Fatalf("audioLangISO(%q) unexpectedly mapped", bad)
		}
	}
	// Bare 3-letter codes pass through.
	if got, ok := audioLangISO("rus"); !ok || got != "rus" {
		t.Fatalf("audioLangISO(rus) = %q, %v", got, ok)
	}
}

func TestAudioRetagArgs(t *testing.T) {
	args := audioRetagArgs("in.mp4", "out.mp4", []string{"rus", "ger"}, true)
	want := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", "in.mp4",
		"-map", "0", "-c", "copy", "-movflags", "+faststart",
		"-metadata:s:a:0", "language=rus", "-metadata:s:a:1", "language=ger", "out.mp4"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %v, want %v", args, want)
		}
	}

	// An unmappable language is skipped without shifting later stream tags.
	args = audioRetagArgs("in.mp4", "out.mp4", []string{"", "ger"}, false)
	want = []string{"-y", "-hide_banner", "-loglevel", "error", "-i", "in.mp4",
		"-map", "0", "-c", "copy", "-metadata:s:a:1", "language=ger", "out.mp4"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %v, want %v", args, want)
		}
	}
}

func TestRetagAudioLanguagesIntegration(t *testing.T) {
	ffmpeg, err := findBinary("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	src := dir + "/src.mkv"
	// Two sine audio streams, no language tags.
	if out, err := commandCombinedOutput(testCtx(t), 0, ffmpeg,
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=880:duration=1",
		"-map", "0:a", "-map", "1:a", "-c:a", "aac", src); err != nil {
		t.Fatalf("setup failed: %s: %v", out, err)
	}
	if err := retagAudioLanguages(testCtx(t), ffmpeg, src, []string{"ru", "de"}); err != nil {
		t.Fatalf("retag failed: %v", err)
	}
	out, err := commandCombinedOutput(testCtx(t), 0, "ffprobe",
		"-v", "error", "-show_entries", "stream=index,codec_type:stream_tags=language",
		"-of", "csv", src)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "0,audio,rus") || !strings.Contains(text, "1,audio,deu") {
		t.Fatalf("unexpected stream tags:\n%s", text)
	}
}
