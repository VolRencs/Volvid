// Package services holds application use-cases extracted from tui/flow.go.
//
// Flow logic such as "DetectDeps -> require ffmpeg -> PrepareDownloadRequest
// -> StartDownloadRequestContext" used to live in the Bubble Tea layer,
// making it untestable without a live Env and real processes. Services are
// pure w.r.t. TUI: they take explicit inputs, return explicit decisions, and
// depend only on internal/core domain types.
package services

import (
	"volvid/internal/core"
)

// DownloadPlan is the validated decision to start a download.
type DownloadPlan struct {
	Request core.DownloadRequest
	Deps    core.CheckDepsResult
	Workers int
	Total   int
}

// MissingDependencyError signals that the UI must open the dependency
// screen instead of starting the download.
type MissingDependencyError struct {
	Name string
}

func (e *MissingDependencyError) Error() string { return "missing dependency: " + e.Name }

// PlanDownload validates prerequisites and builds the download request.
// It replaces the first half of tui Model.startDownload: dep checks,
// request normalization/validation and worker-count policy.
func PlanDownload(
	deps core.CheckDepsResult,
	target core.ParsedTarget,
	profile core.OutputProfile,
	fragment *core.DownloadFragment,
	mediaDuration int,
	forceSingle bool,
	plInfo *core.PlaylistInfo,
	entries []core.PlaylistEntry,
	numWorkers int,
	outputDir string,
	locale core.Locale,
	prepare func(core.DownloadRequest, core.CheckDepsResult) (core.DownloadRequest, error),
) (DownloadPlan, error) {
	if !deps.YTDLP.Available {
		return DownloadPlan{}, &MissingDependencyError{Name: deps.YTDLP.Name}
	}
	if profile.NeedsFFmpeg(fragment) && !deps.FFmpeg.Available {
		return DownloadPlan{}, &MissingDependencyError{Name: deps.FFmpeg.Name}
	}

	req := core.DownloadRequest{
		Target:        target,
		Profile:       profile,
		Fragment:      fragment,
		MediaDuration: mediaDuration,
		ForceSingle:   forceSingle,
		PlaylistInfo:  plInfo,
		Entries:       entries,
		Workers:       max(numWorkers, 1),
		OutputDir:     outputDir,
		Locale:        locale,
	}
	prepared, err := prepare(req, deps)
	if err != nil {
		return DownloadPlan{}, err
	}

	workers := max(numWorkers, 1)
	if len(entries) == 0 {
		workers = 1
	}
	prepared.Workers = workers

	return DownloadPlan{
		Request: prepared,
		Deps:    deps,
		Workers: workers,
		Total:   len(entries),
	}, nil
}
