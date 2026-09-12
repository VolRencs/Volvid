package tui

import (
	"context"
	"testing"

	"volvid/internal/adapters"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

// stubAPI is a scriptable AppAPI fake: no processes, no network, no fs.
// Env-dependent behavior is canned via fields; everything else falls back to
// the production adapter.
type stubAPI struct {
	AppAPI

	locale core.Locale
	deps   core.CheckDepsResult
	dir    string

	prepareErr error
	started    bool
}

func testDeps(ytAvail, ffAvail bool) core.CheckDepsResult {
	return core.CheckDepsResult{
		YTDLP:  core.DependencyInfo{Key: "ytdlp", Name: "yt-dlp", Available: ytAvail},
		FFmpeg: core.DependencyInfo{Key: "ffmpeg", Name: "ffmpeg", Available: ffAvail},
	}
}

func newStubAPI() *stubAPI {
	return &stubAPI{
		AppAPI: newAppAPI(adapters.NewEnv()),
		locale: core.LocaleEN,
		deps:   testDeps(true, true),
		dir:    "/tmp/volvid-test",
	}
}

func (s *stubAPI) DetectDeps() core.CheckDepsResult { return s.deps }

func (s *stubAPI) RefreshDeps() core.CheckDepsResult { return s.deps }

func (s *stubAPI) PrepareDownload(req core.DownloadRequest, _ core.CheckDepsResult) (core.DownloadRequest, error) {
	if s.prepareErr != nil {
		return core.DownloadRequest{}, s.prepareErr
	}
	return req, nil
}

func (s *stubAPI) StartDownload(_ context.Context, _ core.DownloadRequest, _ core.CheckDepsResult, _ chan<- core.DlUpdate) {
	s.started = true
}

func (s *stubAPI) DownloadsDir() string { return s.dir }

func (s *stubAPI) DownloadsDirLocked() bool { return false }

func (s *stubAPI) IsWindows() bool { return false }

func (s *stubAPI) LoadLocale() core.Locale { return s.locale }

func (s *stubAPI) SaveLocale(l core.Locale) error { s.locale = l; return nil }

func (s *stubAPI) ResolveSubtitles(_ context.Context, _ string) ([]core.SubtitleTrack, error) {
	return []core.SubtitleTrack{{Lang: "en"}, {Lang: "ru", Auto: true}}, nil
}

func (s *stubAPI) ResolveAudioTracks(_ context.Context, _ string) ([]core.AudioTrack, error) {
	return []core.AudioTrack{{Lang: "en"}, {Lang: "ru"}}, nil
}

func newStubModel(stub *stubAPI) Model {
	m := newModelWithAPI(context.Background(), stub)
	m.deps = stub.deps
	return m
}

func TestStartDownloadMissingYtDlpOpensDepScreen(t *testing.T) {
	stub := newStubAPI()
	stub.deps = testDeps(false, true)
	m := newStubModel(stub)
	m.target = core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"}
	m.profile = i18n.DefaultVideoProfile(core.LocaleEN)
	m.locale = core.LocaleEN

	model, _ := m.startDownload()
	got, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", model)
	}
	if got.screen != scrDepUpdate {
		t.Fatalf("expected dep screen, got %v", got.screen)
	}
	if stub.started {
		t.Fatal("download must not start without yt-dlp")
	}
}

func TestStartDownloadValidationErrorStaysOnConfig(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.target = core.ParsedTarget{} // invalid: Prepare would fail; force via stub
	stub.prepareErr = context.DeadlineExceeded
	m.profile = i18n.DefaultVideoProfile(core.LocaleEN)
	m.locale = core.LocaleEN
	m.screen = scrQuality

	model, _ := m.startDownload()
	got := model.(Model)
	if got.flowErr == "" {
		t.Fatal("expected flowErr to be set")
	}
	if stub.started {
		t.Fatal("download must not start on validation error")
	}
}
