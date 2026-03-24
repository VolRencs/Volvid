package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type QualityConfig struct {
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

var (
	dlRE    = regexp.MustCompile(`(?i)\[download\]\s+(?P<pct>[\d.]+)%\s+of\s+~?\s*(?P<size>[\d.]+)\s*(?P<unit>[KMGTkmgt]i?[Bb])`)
	speedRE = regexp.MustCompile(`(?i)at\s+(?P<speed>[\d.]+\s*[KMGTkmgt]i?[Bb]/s)`)
	destRE  = regexp.MustCompile(`\[download\]\s+Destination:\s+(.+)`)
	procRE  = regexp.MustCompile(`(?i)^\s*\[(Merger|ExtractAudio)\]`)
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

func streamYtdlp(ctx context.Context, slot int, args []string, ch chan<- DlUpdate) bool {
	cmd := exec.CommandContext(ctx, YtdlpBin, slices.Concat([]string{"--newline", "--no-warnings"}, args)...)
	pr, pw, err := os.Pipe()
	if err != nil {
		return false
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return false
	}
	pw.Close()

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
			label := Loc.MergeProc
			if strings.Contains(strings.ToLower(m[1]), "audio") {
				label = Loc.MP3Proc
			}
			ch <- DlUpdate{Type: EvProc, Slot: slot, Text: label}
		}
	}
	if err := sc.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = pr.Close()
		return false
	}
	_ = pr.Close()
	if err := cmd.Wait(); err != nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	return true
}

func buildArgs(cfg QualityConfig, url, tmpl, format string, extra []string) []string {
	args := ffmpegArgs()
	if len(cfg.FmtChain) == 0 {
		args = append(args, "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0")
	} else {
		args = append(args, "-f", format, "--merge-output-format", "mp4")
	}
	args = append(args, "-o", tmpl, "--windows-filenames")
	args = append(args, extra...)
	args = append(args, url)
	return args
}

func runWithFallback(ctx context.Context, slot int, cfg QualityConfig, url, tmpl string, extra []string, ch chan<- DlUpdate) bool {
	if len(cfg.FmtChain) == 0 {
		return streamYtdlp(ctx, slot, buildArgs(cfg, url, tmpl, "", extra), ch)
	}
	for i, format := range cfg.FmtChain {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
		if i > 0 {
			label := format
			if i < len(cfg.FmtLabels) && cfg.FmtLabels[i] != "" {
				label = cfg.FmtLabels[i]
			}
			ch <- DlUpdate{
				Type: EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(Loc.FallbackFmt, i, label),
			}
		}
		if streamYtdlp(ctx, slot, buildArgs(cfg, url, tmpl, format, extra), ch) {
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
	StartDownloadContextInDir(context.Background(), DlDir, cfg, url, forceSingle, plInfo, entries, workers, ch)
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
	StartDownloadContextInDir(ctx, DlDir, cfg, url, forceSingle, plInfo, entries, workers, ch)
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
	go func() {
		var wg sync.WaitGroup
		defer func() { wg.Wait(); close(ch) }()
		if ctx == nil {
			ctx = context.Background()
		}
		if strings.TrimSpace(baseDir) == "" {
			baseDir = DlDir
		}

		if plInfo == nil || forceSingle || len(entries) == 0 {
			ok := runWithFallback(ctx, 0, cfg, url,
				filepath.Join(baseDir, "%(title)s.%(ext)s"),
				[]string{"--no-playlist"}, ch,
			)
			ch <- DlUpdate{Type: EvDone, OK: ok}
			return
		}

		plDir := filepath.Join(baseDir, SanitizeDirname(plInfo.Title))
		if err := os.MkdirAll(plDir, 0o755); err != nil {
			plDir = filepath.Join(baseDir, "playlist")
			_ = os.MkdirAll(plDir, 0o755)
		}

		workerCount := normalizeWorkerCount(workers, len(entries))
		jobs := make(chan PlaylistEntry)
		go func() {
			defer close(jobs)
			for _, e := range entries {
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
						ok := runWithFallback(ctx, slot, cfg, e.URL, tmpl, []string{"--no-playlist"}, ch)
						ch <- DlUpdate{Type: EvDone, Slot: slot, OK: ok}
					}()
				}
			}(slot)
		}
	}()
}
