package app

import (
	"bufio"
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
	Label    string
	FmtChain []string
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

func streamYtdlp(slot int, args []string, ch chan<- DlUpdate) bool {
	cmd := exec.Command(YtdlpBin, slices.Concat([]string{"--newline", "--no-warnings"}, args)...)
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
	return cmd.Wait() == nil
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

func runWithFallback(slot int, cfg QualityConfig, url, tmpl string, extra []string, ch chan<- DlUpdate) bool {
	if len(cfg.FmtChain) == 0 {
		return streamYtdlp(slot, buildArgs(cfg, url, tmpl, "", extra), ch)
	}
	for i, format := range cfg.FmtChain {
		if i > 0 {
			ch <- DlUpdate{
				Type: EvFallback,
				Slot: slot,
				Text: fmt.Sprintf(Loc.FallbackFmt, i, format),
			}
		}
		if streamYtdlp(slot, buildArgs(cfg, url, tmpl, format, extra), ch) {
			return true
		}
	}
	return false
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
	go func() {
		var wg sync.WaitGroup
		defer func() { wg.Wait(); close(ch) }()

		if plInfo == nil || forceSingle || len(entries) == 0 {
			ok := runWithFallback(0, cfg, url,
				filepath.Join(DlDir, "%(title)s.%(ext)s"),
				[]string{"--no-playlist"}, ch,
			)
			ch <- DlUpdate{Type: EvDone, OK: ok}
			return
		}

		plDir := filepath.Join(DlDir, SanitizeDirname(plInfo.Title))
		if err := os.MkdirAll(plDir, 0o755); err != nil {
			plDir = filepath.Join(DlDir, "playlist")
			_ = os.MkdirAll(plDir, 0o755)
		}

		slotCh := make(chan int, workers)
		for i := range workers {
			slotCh <- i
		}
		jobs := make(chan PlaylistEntry)
		go func() {
			for _, e := range entries {
				jobs <- e
			}
			close(jobs)
		}()
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for e := range jobs {
					slot := <-slotCh
					func() {
						defer func() {
							time.Sleep(slotResetDelay)
							ch <- DlUpdate{Type: EvReset, Slot: slot}
							slotCh <- slot
						}()
						ch <- DlUpdate{Type: EvStart, Slot: slot, Text: e.Title}
						tmpl := filepath.Join(plDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", e.Index))
						ok := runWithFallback(slot, cfg, e.URL, tmpl, []string{"--no-playlist"}, ch)
						ch <- DlUpdate{Type: EvDone, Slot: slot, OK: ok}
					}()
				}
			}()
		}
	}()
}
