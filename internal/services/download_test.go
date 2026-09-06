package services

import (
	"errors"
	"testing"
	"volvid/internal/core"
)

func testDeps(ytAvail, ffAvail bool) core.CheckDepsResult {
	return core.CheckDepsResult{
		YTDLP:  core.DependencyInfo{Name: "yt-dlp", Available: ytAvail},
		FFmpeg: core.DependencyInfo{Name: "ffmpeg", Available: ffAvail},
	}
}

func okPrepare(req core.DownloadRequest, _ core.CheckDepsResult) (core.DownloadRequest, error) {
	return req, nil
}

func TestPlanDownloadMissingYtDlp(t *testing.T) {
	_, err := PlanDownload(testDeps(false, true),
		core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"},
		core.OutputProfile{Mode: core.ModeVideo}, nil, 0, true, nil, nil, 1, "/tmp", core.LocaleEN, okPrepare)
	var missing *MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingDependencyError, got %v", err)
	}
	if missing.Name != "yt-dlp" {
		t.Fatalf("expected yt-dlp, got %q", missing.Name)
	}
}

func TestPlanDownloadMissingFFmpegForAudio(t *testing.T) {
	_, err := PlanDownload(testDeps(true, false),
		core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"},
		core.OutputProfile{Mode: core.ModeAudio}, nil, 0, true, nil, nil, 1, "/tmp", core.LocaleEN, okPrepare)
	var missing *MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingDependencyError, got %v", err)
	}
	if missing.Name != "ffmpeg" {
		t.Fatalf("expected ffmpeg, got %q", missing.Name)
	}
}

func TestPlanDownloadNoFFmpegNeededForPlainVideo(t *testing.T) {
	plan, err := PlanDownload(testDeps(true, false),
		core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"},
		core.OutputProfile{Mode: core.ModeVideo}, nil, 0, true, nil, nil, 1, "/tmp", core.LocaleEN, okPrepare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Workers != 1 {
		t.Fatalf("expected 1 worker, got %d", plan.Workers)
	}
}

func TestPlanDownloadValidationErrorPassthrough(t *testing.T) {
	want := errors.New("bad request")
	prepare := func(core.DownloadRequest, core.CheckDepsResult) (core.DownloadRequest, error) {
		return core.DownloadRequest{}, want
	}
	_, err := PlanDownload(testDeps(true, true),
		core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"},
		core.OutputProfile{Mode: core.ModeVideo}, nil, 0, true, nil, nil, 1, "/tmp", core.LocaleEN, prepare)
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestWorkerCount(t *testing.T) {
	if got := WorkerCount(5, 0); got != 1 {
		t.Fatalf("empty entries: expected 1, got %d", got)
	}
	if got := WorkerCount(0, 3); got != 1 {
		t.Fatalf("zero workers: expected 1, got %d", got)
	}
	if got := WorkerCount(3, 10); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}
