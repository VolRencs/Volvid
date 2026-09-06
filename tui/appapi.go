package tui

import (
	"context"

	"volvid/internal/adapters"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

// AppAPI is the anti-corruption layer between the Bubble Tea UI and the
// domain. TUI code depends on this interface plus pure core/i18n helpers,
// never on adapters concretes (except this file and constructors).
//
// Production use: appAPIAdapter{env} (built by New/NewWithDeps).
// Tests: stubAPI with scripted responses, no processes/network/fs.
type AppAPI interface {
	// Target / playlist / search / probe / quality.
	ParseTarget(raw string) (core.ParsedTarget, error)
	DetectDeps() core.CheckDepsResult
	RefreshDeps() core.CheckDepsResult
	FetchPlaylist(ctx context.Context, url string, l core.Locale) (*core.PlaylistInfo, error)
	SearchYouTube(ctx context.Context, query string) ([]core.SearchResult, error)
	ResolveQuality(ctx context.Context, urls []string) ([]core.QualityChoice, error)
	ProbeDuration(ctx context.Context, target core.ParsedTarget) (int, error)

	// Download pipeline (use-case extracted from flow.go:startDownload).
	PrepareDownload(req core.DownloadRequest, deps core.CheckDepsResult) (core.DownloadRequest, error)
	StartDownload(ctx context.Context, req core.DownloadRequest, deps core.CheckDepsResult, ch chan<- core.DlUpdate)
	DefaultVideoProfile(l core.Locale) core.OutputProfile
	DefaultProfileForMode(mode core.DownloadMode, l core.Locale) core.OutputProfile

	// Fragment / playlist selection validation.
	ParseFragment(raw string, mediaDuration int) (core.DownloadFragment, error)
	ValidateFragment(fragment core.DownloadFragment, mediaDuration int) error
	ParseSelection(raw string, maxIdx int, l core.Locale) ([]int, error)

	// Directories / OS integration.
	DownloadsDir() string
	SetDownloadsDir(path string) error
	DownloadsDirLocked() bool
	PickDir(ctx context.Context, current string, l core.Locale) (string, error)
	IsPickerCancelled(err error) bool
	OpenFolder(path string) error

	// Deps install / update.
	InstallDependency(ctx context.Context, key string, l core.Locale, ch chan<- core.FileProgress) error
	CheckUpdate() *core.UpdateInfo
	ApplyUpdate(ctx context.Context, l core.Locale, info *core.UpdateInfo, ch chan<- core.FileProgress) error

	// Locale / version.
	LoadLocale() core.Locale
	SaveLocale(l core.Locale) error
	NextLocale(l core.Locale) core.Locale
	Strings(l core.Locale) *i18n.UIStrings
	IsWindows() bool
	AppVersion() string
}

// appAPIAdapter is the production AppAPI backed by internal/adapters.
type appAPIAdapter struct {
	env *adapters.Env
}

func newAppAPI(env *adapters.Env) AppAPI {
	if env == nil {
		env = adapters.NewEnv()
	}
	return appAPIAdapter{env: env}
}

func (a appAPIAdapter) ParseTarget(raw string) (core.ParsedTarget, error) {
	return core.ParseTarget(raw)
}

func (a appAPIAdapter) DetectDeps() core.CheckDepsResult { return adapters.DetectDeps(a.env) }

func (a appAPIAdapter) RefreshDeps() core.CheckDepsResult { return adapters.RefreshDeps(a.env) }

func (a appAPIAdapter) FetchPlaylist(ctx context.Context, url string, l core.Locale) (*core.PlaylistInfo, error) {
	return adapters.FetchPlaylistInfoFor(a.env, ctx, url, l)
}

func (a appAPIAdapter) SearchYouTube(ctx context.Context, query string) ([]core.SearchResult, error) {
	return adapters.SearchYouTubeContext(a.env, ctx, query)
}

func (a appAPIAdapter) ResolveQuality(ctx context.Context, urls []string) ([]core.QualityChoice, error) {
	return adapters.ResolveQualityChoicesContext(a.env, ctx, urls)
}

func (a appAPIAdapter) ProbeDuration(ctx context.Context, target core.ParsedTarget) (int, error) {
	return adapters.ProbeMediaDurationContext(a.env, ctx, target)
}

func (a appAPIAdapter) PrepareDownload(req core.DownloadRequest, deps core.CheckDepsResult) (core.DownloadRequest, error) {
	return adapters.PrepareDownloadRequestWithDeps(a.env, req, deps)
}

func (a appAPIAdapter) StartDownload(ctx context.Context, req core.DownloadRequest, deps core.CheckDepsResult, ch chan<- core.DlUpdate) {
	adapters.StartDownloadRequestContext(a.env, ctx, req, deps, ch)
}

func (a appAPIAdapter) DefaultVideoProfile(l core.Locale) core.OutputProfile {
	return i18n.DefaultVideoProfile(l)
}

func (a appAPIAdapter) DefaultProfileForMode(mode core.DownloadMode, l core.Locale) core.OutputProfile {
	return i18n.DefaultProfileForMode(mode, l)
}

func (a appAPIAdapter) ParseFragment(raw string, mediaDuration int) (core.DownloadFragment, error) {
	return core.ParseBoundedFragmentRange(raw, mediaDuration)
}

func (a appAPIAdapter) ValidateFragment(fragment core.DownloadFragment, mediaDuration int) error {
	return core.ValidateFragmentDuration(fragment, mediaDuration)
}

func (a appAPIAdapter) ParseSelection(raw string, maxIdx int, l core.Locale) ([]int, error) {
	return adapters.ParseSelectionFor(raw, maxIdx, l)
}

func (a appAPIAdapter) DownloadsDir() string { return a.env.DownloadsDir() }

func (a appAPIAdapter) SetDownloadsDir(path string) error {
	return adapters.SetDownloadsDir(a.env, path)
}

func (a appAPIAdapter) DownloadsDirLocked() bool { return adapters.DownloadsDirLocked() }

func (a appAPIAdapter) PickDir(ctx context.Context, current string, l core.Locale) (string, error) {
	return adapters.PickDownloadsDir(ctx, a.env, current, l)
}

func (a appAPIAdapter) IsPickerCancelled(err error) bool {
	return adapters.IsFolderPickerCancelled(err)
}

func (a appAPIAdapter) OpenFolder(path string) error { return adapters.OpenInFileManager(path) }

func (a appAPIAdapter) InstallDependency(ctx context.Context, key string, l core.Locale, ch chan<- core.FileProgress) error {
	return adapters.InstallDependencyFor(a.env, ctx, key, l, ch)
}

func (a appAPIAdapter) CheckUpdate() *core.UpdateInfo { return adapters.CheckUpdate(a.env) }

func (a appAPIAdapter) ApplyUpdate(ctx context.Context, l core.Locale, info *core.UpdateInfo, ch chan<- core.FileProgress) error {
	return adapters.ApplyUpdateFor(a.env, ctx, l, info, ch)
}

func (a appAPIAdapter) LoadLocale() core.Locale { return adapters.LoadLocale(a.env) }

func (a appAPIAdapter) SaveLocale(l core.Locale) error { return adapters.SaveLocale(a.env, l) }

func (a appAPIAdapter) NextLocale(l core.Locale) core.Locale { return core.NextLocale(l) }

func (a appAPIAdapter) Strings(l core.Locale) *i18n.UIStrings { return i18n.StringsFor(l) }

func (a appAPIAdapter) IsWindows() bool { return a.env.IsWindows }

func (a appAPIAdapter) AppVersion() string { return adapters.Version }
