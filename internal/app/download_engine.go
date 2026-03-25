package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type QualityConfig struct {
	Locale    Locale
	Label     string
	FmtChain  []string
	FmtLabels []string
}

var qualityChains = [3][]string{
	{"bestvideo+bestaudio/best", "bestvideo+bestaudio", "best"},
	{"bestvideo[height<=360]+bestaudio/best[height<=360]", "best[height<=360]", "worst"},
	nil,
}

func QualityChainAt(idx int) []string {
	if idx < 0 || idx >= len(qualityChains) {
		return nil
	}
	return slices.Clone(qualityChains[idx])
}

func profileFromQualityConfig(cfg QualityConfig) OutputProfile {
	return OutputProfile{
		Key:            cfg.Label,
		Label:          cfg.Label,
		Mode:           ModeVideo,
		VideoFmtChain:  slices.Clone(cfg.FmtChain),
		VideoFmtLabels: slices.Clone(cfg.FmtLabels),
	}
}

var (
	dlRE    = regexp.MustCompile(`(?i)\[download\]\s+(?P<pct>[\d.]+)%\s+of\s+~?\s*(?P<size>[\d.]+)\s*(?P<unit>[KMGTkmgt]i?[Bb])`)
	speedRE = regexp.MustCompile(`(?i)at\s+(?P<speed>[\d.]+\s*[KMGTkmgt]i?[Bb]/s)`)
	destRE  = regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`)
	procRE  = regexp.MustCompile(`(?i)^\s*\[(Merger|ExtractAudio|Thumbnails?Convertor)\]`)
	numRE   = regexp.MustCompile(`^\d+\s*[-–]\s*`)
)

type DlEventType uint8

const (
	EvStart DlEventType = iota
	EvDest
	EvProgress
	EvProc
	EvDone
	EvReset
	EvFallback
	EvClosed
)

type DlUpdate struct {
	Type   DlEventType
	Slot   int
	Text   string
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	OK     bool
}

func subexp(re *regexp.Regexp, m []string, name string) string {
	if i := re.SubexpIndex(name); i >= 0 && i < len(m) {
		return m[i]
	}
	return ""
}

func ffmpegArgs() []string {
	if FFmpegResolved != "" {
		return []string{"--ffmpeg-location", FFmpegResolved}
	}
	return nil
}

func streamYtdlp(ctx context.Context, slot int, l Locale, args []string, ch chan<- DlUpdate) bool {
	cmd, pr, runCtx, cancel, err := startMergedOutputCommand(
		ctx,
		0,
		YtdlpBin,
		slices.Concat([]string{"--newline", "--no-warnings"}, args)...,
	)
	if err != nil {
		return false
	}
	defer cancel()
	defer pr.Close()

	loc := StringsFor(l)

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case destRE.MatchString(line):
			m := destRE.FindStringSubmatch(line)
			stem := strings.TrimSpace(m[1])
			stem = numRE.ReplaceAllString(
				filepath.Base(strings.TrimSuffix(stem, filepath.Ext(stem))), "",
			)
			if r := []rune(stem); len(r) > 58 {
				stem = string(r[:58])
			}
			ch <- DlUpdate{Type: EvDest, Slot: slot, Text: stem}

		case dlRE.MatchString(line):
			m := dlRE.FindStringSubmatch(line)
			pct, _ := strconv.ParseFloat(subexp(dlRE, m, "pct"), 64)
			size, _ := strconv.ParseFloat(subexp(dlRE, m, "size"), 64)
			totalB := int64(size * float64(unitToMult(subexp(dlRE, m, "unit"))))
			speed := ""
			if sm := speedRE.FindStringSubmatch(line); sm != nil {
				speed = subexp(speedRE, sm, "speed")
			}
			ch <- DlUpdate{
				Type:   EvProgress,
				Slot:   slot,
				Pct:    pct,
				DoneB:  int64(float64(totalB) * pct / 100),
				TotalB: totalB,
				Speed:  speed,
			}

		case procRE.MatchString(line):
			m := procRE.FindStringSubmatch(line)
			label := loc.MergeProc
			switch lower := strings.ToLower(m[1]); {
			case strings.Contains(lower, "audio"):
				label = loc.MP3Proc
			case strings.Contains(lower, "thumbnail"):
				label = loc.ThumbProc
			}
			ch <- DlUpdate{Type: EvProc, Slot: slot, Text: label}
		}
	}
	if err := sc.Err(); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = waitCommand(cmd, runCtx)
		return false
	}
	if err := waitCommand(cmd, runCtx); err != nil {
		return false
	}
	if runCtx != nil && runCtx.Err() != nil {
		return false
	}
	return true
}

func runDownloadRequest(ctx context.Context, slot int, req DownloadRequest, url, tmpl string, extra []string, ch chan<- DlUpdate) bool {
	req = NormalizeDownloadRequest(req)
	strs := StringsFor(req.Locale)

	formats := []string{""}
	labels := []string{""}
	if req.Profile.Mode == ModeVideo {
		formats = req.Profile.VideoFmtChain
		labels = req.Profile.VideoFmtLabels
		if len(formats) == 0 {
			formats = []string{"bestvideo+bestaudio/best"}
		}
	}

	for i, format := range formats {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
		if req.Profile.Mode == ModeVideo && i > 0 {
			label := format
			if i < len(labels) && labels[i] != "" {
				label = labels[i]
			}
			ch <- DlUpdate{
				Type: EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(strs.FallbackFmt, i, label),
			}
		}

		spec, err := BuildCommandSpec(req, url, tmpl, format, extra)
		if err != nil {
			return false
		}
		if streamYtdlp(ctx, slot, req.Locale, spec.Args, ch) {
			return true
		}
	}
	return false
}

func normalizeWorkerCount(workers, jobs int) int {
	if jobs <= 0 {
		return 1
	}
	workers = max(workers, 1)
	return min(workers, jobs)
}

func StartDownload(
	cfg QualityConfig,
	url string,
	forceSingle bool,
	plInfo *PlaylistInfo,
	entries []PlaylistEntry,
	workers int,
	ch chan<- DlUpdate,
) {
	target, err := ParseTarget(url)
	if err != nil {
		go func() {
			defer close(ch)
			ch <- DlUpdate{Type: EvDone, OK: false}
		}()
		return
	}
	StartDownloadRequest(DownloadRequest{
		Target:       target,
		Profile:      profileFromQualityConfig(cfg),
		ForceSingle:  forceSingle,
		PlaylistInfo: plInfo,
		Entries:      entries,
		Workers:      workers,
		OutputDir:    DlDir,
		Locale:       cfg.Locale,
	}, ch)
}

func StartDownloadContext(
	ctx context.Context,
	cfg QualityConfig,
	url string,
	forceSingle bool,
	plInfo *PlaylistInfo,
	entries []PlaylistEntry,
	workers int,
	ch chan<- DlUpdate,
) {
	target, err := ParseTarget(url)
	if err != nil {
		go func() {
			defer close(ch)
			ch <- DlUpdate{Type: EvDone, OK: false}
		}()
		return
	}
	StartDownloadRequestContext(ctx, DownloadRequest{
		Target:       target,
		Profile:      profileFromQualityConfig(cfg),
		ForceSingle:  forceSingle,
		PlaylistInfo: plInfo,
		Entries:      entries,
		Workers:      workers,
		OutputDir:    DlDir,
		Locale:       cfg.Locale,
	}, ch)
}

func StartDownloadRequest(req DownloadRequest, ch chan<- DlUpdate) {
	StartDownloadRequestContext(context.Background(), req, ch)
}

func StartDownloadRequestContext(ctx context.Context, req DownloadRequest, ch chan<- DlUpdate) {
	go func() {
		var wg sync.WaitGroup
		defer func() { wg.Wait(); close(ch) }()
		if ctx == nil {
			ctx = context.Background()
		}

		req = NormalizeDownloadRequest(req)
		if err := ValidateDownloadRequest(req); err != nil {
			ch <- DlUpdate{Type: EvDone, OK: false}
			return
		}

		if req.PlaylistInfo == nil || req.ForceSingle || len(req.Entries) == 0 {
			ok := runDownloadRequest(ctx, 0, req, req.Target.DownloadURL(req.ForceSingle),
				filepath.Join(req.OutputDir, "%(title)s.%(ext)s"),
				[]string{"--no-playlist"}, ch,
			)
			ch <- DlUpdate{Type: EvDone, OK: ok}
			return
		}

		plDir := filepath.Join(req.OutputDir, SanitizeDirname(req.PlaylistInfo.Title))
		if err := os.MkdirAll(plDir, 0o755); err != nil {
			plDir = filepath.Join(req.OutputDir, "playlist")
			_ = os.MkdirAll(plDir, 0o755)
		}

		workerCount := normalizeWorkerCount(req.Workers, len(req.Entries))
		jobs := make(chan PlaylistEntry)
		go func() {
			defer close(jobs)
			for _, e := range req.Entries {
				select {
				case <-ctx.Done():
					return
				case jobs <- e:
				}
			}
		}()

		for slot := 0; slot < workerCount; slot++ {
			wg.Add(1)
			go func(slot int) {
				defer wg.Done()
				for e := range jobs {
					if ctx.Err() != nil {
						return
					}
					func() {
						defer func() {
							time.Sleep(slotResetDelay)
							ch <- DlUpdate{Type: EvReset, Slot: slot}
						}()
						ch <- DlUpdate{Type: EvStart, Slot: slot, Text: e.Title}
						tmpl := filepath.Join(plDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", e.Index))
						ok := runDownloadRequest(ctx, slot, req, e.URL, tmpl, []string{"--no-playlist"}, ch)
						ch <- DlUpdate{Type: EvDone, Slot: slot, OK: ok}
					}()
				}
			}(slot)
		}
	}()
}

func StartDownloadContextInDir(
	ctx context.Context,
	baseDir string,
	cfg QualityConfig,
	url string,
	forceSingle bool,
	plInfo *PlaylistInfo,
	entries []PlaylistEntry,
	workers int,
	ch chan<- DlUpdate,
) {
	target, err := ParseTarget(url)
	if err != nil {
		go func() {
			defer close(ch)
			ch <- DlUpdate{Type: EvDone, OK: false}
		}()
		return
	}
	StartDownloadRequestContext(ctx, DownloadRequest{
		Target:       target,
		Profile:      profileFromQualityConfig(cfg),
		ForceSingle:  forceSingle,
		PlaylistInfo: plInfo,
		Entries:      entries,
		Workers:      workers,
		OutputDir:    baseDir,
		Locale:       cfg.Locale,
	}, ch)
}
