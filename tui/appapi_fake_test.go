package tui

import (
	"context"
	"testing"

	"volvid/internal/adapters"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

// stubAPI is a scriptable AppAPI fake: no processes, no network, no fs.
// Env-dependent behavior is canned via fields; pure parsing delegates to
// core, labels to i18n.
type stubAPI struct {
	AppAPI // nil embedded; overridden methods below take precedence

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
	return &stubAPI{locale: core.LocaleEN, deps: testDeps(true, true), dir: "/tmp/volvid-test"}
}

func (s *stubAPI) ParseTarget(raw string) (core.ParsedTarget, error) {
	return core.ParseTarget(raw)
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

func (s *stubAPI) DefaultVideoProfile(l core.Locale) core.OutputProfile {
	return i18n.DefaultVideoProfile(l)
}

func (s *stubAPI) DefaultProfileForMode(mode core.DownloadMode, l core.Locale) core.OutputProfile {
	return i18n.DefaultProfileForMode(mode, l)
}

func (s *stubAPI) DownloadsDir() string { return s.dir }

func (s *stubAPI) DownloadsDirLocked() bool { return false }

func (s *stubAPI) IsWindows() bool { return false }

func (s *stubAPI) LoadLocale() core.Locale { return s.locale }

func (s *stubAPI) SaveLocale(l core.Locale) error { s.locale = l; return nil }

func (s *stubAPI) NextLocale(l core.Locale) core.Locale { return core.NextLocale(l) }

func (s *stubAPI) Strings(l core.Locale) *i18n.UIStrings { return i18n.StringsFor(l) }

func (s *stubAPI) ParseFragment(raw string, d int) (core.DownloadFragment, error) {
	return core.ParseBoundedFragmentRange(raw, d)
}

func (s *stubAPI) ValidateFragment(f core.DownloadFragment, d int) error {
	return core.ValidateFragmentDuration(f, d)
}

func (s *stubAPI) ParseSelection(raw string, maxIdx int, l core.Locale) ([]int, error) {
	return adapters.ParseSelectionFor(raw, maxIdx, l)
}

func (s *stubAPI) ResolveSubtitles(_ context.Context, _ string) ([]core.SubtitleTrack, error) {
	return []core.SubtitleTrack{{Lang: "en"}, {Lang: "ru", Auto: true}}, nil
}

func newStubModel(stub *stubAPI) Model {
	return newModelWithAPI(context.Background(), stub)
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

func TestNewWithDepsUsesInjectedAPI(t *testing.T) {
	stub := newStubAPI()
	stub.locale = core.LocaleRU
	got := NewWithDeps(context.Background(), stub)
	m, ok := got.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", got)
	}
	if m.locale != core.LocaleRU {
		t.Fatalf("expected injected locale ru, got %v", m.locale)
	}
	if m.api == nil {
		t.Fatal("expected api to be set")
	}
}
