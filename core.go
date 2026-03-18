package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Version    = "3.3.0"
	GithubRepo = "VolRencs/YouTubeDownloader"

	ffmpegWinURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ytdlpBase    = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"
)

var (
	IsWindows      = runtime.GOOS == "windows"
	Arch           = strings.ToLower(runtime.GOARCH)
	DepsDir        string
	DlDir          string
	YtdlpBin       string
	FFmpegResolved string
)

func init() {
	exe, err := os.Executable()
	if err != nil {
		exe, _ = filepath.Abs(os.Args[0])
	}
	base    := filepath.Dir(exe)
	DepsDir  = filepath.Join(base, "_deps")
	DlDir    = filepath.Join(base, "downloads")
	if IsWindows {
		YtdlpBin = filepath.Join(DepsDir, "yt-dlp.exe")
	} else {
		YtdlpBin = filepath.Join(DepsDir, "yt-dlp")
	}
}

func FmtBytes(n int64) string {
	switch {
	case n >= 1_073_741_824:
		return fmt.Sprintf("%.2f ГБ", float64(n)/1_073_741_824)
	case n >= 1_048_576:
		return fmt.Sprintf("%.1f МБ", float64(n)/1_048_576)
	case n >= 1_024:
		return fmt.Sprintf("%d КБ", n/1_024)
	default:
		return fmt.Sprintf("%d Б", n)
	}
}

func FmtDuration(secs int) string {
	if secs <= 0 {
		return "??:??"
	}
	h, m, s := secs/3600, (secs%3600)/60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

var invalidFilenameRE = regexp.MustCompile(`[<>:"/\\|?*]`)

func SanitizeDirname(name string) string {
	name = strings.TrimRight(invalidFilenameRE.ReplaceAllString(strings.TrimSpace(name), "_"), " .")
	if r := []rune(name); len(r) > 180 {
		name = string(r[:180])
	}
	if name == "" {
		return "playlist"
	}
	return name
}

func unitToMult(unit string) int64 {
	u := strings.ToUpper(unit)
	u, _ = strings.CutSuffix(u, "IB")
	u, _ = strings.CutSuffix(u, "B")
	switch u {
	case "K": return 1_024
	case "M": return 1_048_576
	case "G": return 1_073_741_824
	case "T": return 1_099_511_627_776
	default:  return 1
	}
}

type FileProgress struct {
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	Done   bool
	Err    error
}

func DownloadFile(url, dest string, ch chan<- FileProgress) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := max(resp.ContentLength, 0)
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	var (
		done, lastDone int64
		lastTime       = time.Now()
		nextEmit       = time.Now()
		speedStr       string
	)
	emit := func(fin bool, e error) {
		if ch == nil {
			return
		}
		now := time.Now()
		if elapsed := now.Sub(lastTime).Seconds(); elapsed > 0 {
			speedStr = FmtBytes(int64(float64(done-lastDone)/elapsed)) + "/с"
			lastDone, lastTime = done, now
		}
		pct := 0.0
		if total > 0 {
			pct = float64(done) / float64(total) * 100
		}
		select {
		case ch <- FileProgress{Pct: pct, DoneB: done, TotalB: total, Speed: speedStr, Done: fin, Err: e}:
		default:
		}
	}

	buf := make([]byte, 32_768)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				emit(true, werr)
				return werr
			}
			done += int64(n)
			if time.Now().After(nextEmit) {
				emit(false, nil)
				nextEmit = time.Now().Add(100 * time.Millisecond)
			}
		}
		if err == io.EOF {
			emit(true, nil)
			return nil
		}
		if err != nil {
			emit(true, err)
			return err
		}
	}
}

type CheckDepsResult struct {
	YtdlpVer      string
	FFmpegMissing bool
}

func DetectDeps() CheckDepsResult {
	r := CheckDepsResult{YtdlpVer: YtdlpVersion()}
	if IsWindows {
		bin := filepath.Join(DepsDir, "ffmpeg.exe")
		if _, err := os.Stat(bin); err == nil {
			FFmpegResolved = bin
		} else {
			r.FFmpegMissing = true
		}
	} else if path, err := exec.LookPath("ffmpeg"); err == nil {
		FFmpegResolved = path
	}
	return r
}

func YtdlpVersion() string {
	if _, err := os.Stat(YtdlpBin); err != nil {
		return ""
	}
	out, err := exec.Command(YtdlpBin, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func YtdlpURL() string {
	switch {
	case IsWindows:
		return ytdlpBase + "yt-dlp.exe"
	case Arch == "arm64" || Arch == "aarch64":
		return ytdlpBase + "yt-dlp_linux_aarch64"
	default:
		return ytdlpBase + "yt-dlp_linux"
	}
}

func InstallYtDlp(ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return err
	}
	if err := DownloadFile(YtdlpURL(), YtdlpBin, ch); err != nil {
		return err
	}
	if !IsWindows {
		os.Chmod(YtdlpBin, 0o755)
	}
	if YtdlpVersion() == "" {
		return fmt.Errorf("бинарник скачан, но не запускается")
	}
	return nil
}

func InstallFFmpeg(ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "ffmpeg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, "ffmpeg.zip")
	if err := DownloadFile(ffmpegWinURL, archive, ch); err != nil {
		return err
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()

	targets := []string{"ffmpeg.exe", "ffprobe.exe"}
	found := false
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		if !slices.Contains(targets, name) {
			continue
		}
		rc, _ := zf.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if os.WriteFile(filepath.Join(DepsDir, name), data, 0o755) == nil && name == "ffmpeg.exe" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("ffmpeg.exe не найден в архиве")
	}
	FFmpegResolved = filepath.Join(DepsDir, "ffmpeg.exe")
	return nil
}

type PlaylistEntry struct {
	Index    int
	Title    string
	URL      string
	Duration int
}

type PlaylistInfo struct {
	Title   string
	Entries []PlaylistEntry
}

func FetchPlaylistInfo(url string) (*PlaylistInfo, error) {
	out, err := exec.Command(YtdlpBin,
		"--flat-playlist", "--dump-json",
		"--quiet", "--ignore-errors", "--no-warnings", url,
	).Output()
	if err != nil {
		return nil, err
	}

	var entries []PlaylistEntry
	var first map[string]any
	idx := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		idx++
		if first == nil {
			first = e
		}
		videoURL := strVal(e, "url", strVal(e, "webpage_url", ""))
		if videoURL == "" {
			if id := strVal(e, "id", ""); id != "" {
				videoURL = "https://youtu.be/" + id
			} else {
				continue
			}
		}
		dur := 0
		if d, ok := e["duration"].(float64); ok {
			dur = int(d)
		}
		entries = append(entries, PlaylistEntry{
			Index:    idx,
			Title:    strVal(e, "title", strVal(e, "id", fmt.Sprintf("Видео %d", idx))),
			URL:      videoURL,
			Duration: dur,
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("плейлист пуст или недоступен")
	}
	title := "playlist"
	if first != nil {
		title = strVal(first, "playlist_title", strVal(first, "playlist", "playlist"))
	}
	return &PlaylistInfo{Title: title, Entries: entries}, nil
}

func strVal(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

var (
	sepRE    = regexp.MustCompile(`[,;\s]+`)
	rangeRE2 = regexp.MustCompile(`^(\d+)\s*[-–]\s*(\d+)$`)
	singleRE = regexp.MustCompile(`^\d+$`)
)

func ParseSelection(raw string, maxIdx int) ([]int, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return nil, fmt.Errorf("выбор не может быть пустым")
	}
	if slices.Contains([]string{"а", "a", "all", "все", "всё", "*"}, raw) {
		r := make([]int, maxIdx)
		for i := range r {
			r[i] = i + 1
		}
		return r, nil
	}
	seen := make(map[int]bool, maxIdx)
	for _, part := range sepRE.Split(raw, -1) {
		if part == "" {
			continue
		}
		if m := rangeRE2.FindStringSubmatch(part); m != nil {
			a, _ := strconv.Atoi(m[1])
			b, _ := strconv.Atoi(m[2])
			if a > b {
				a, b = b, a
			}
			if a < 1 || b > maxIdx {
				return nil, fmt.Errorf("диапазон %d-%d вне 1–%d", a, b, maxIdx)
			}
			for n := a; n <= b; n++ {
				seen[n] = true
			}
		} else if singleRE.MatchString(part) {
			n, _ := strconv.Atoi(part)
			if n < 1 || n > maxIdx {
				return nil, fmt.Errorf("номер %d вне 1–%d", n, maxIdx)
			}
			seen[n] = true
		} else {
			return nil, fmt.Errorf("непонятный ввод: «%s»", part)
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("ничего не выбрано")
	}
	result := make([]int, 0, len(seen))
	for n := range seen {
		result = append(result, n)
	}
	slices.Sort(result)
	return result, nil
}

type QualityConfig struct {
	Label    string
	FmtChain []string
}

var Qualities = [3]QualityConfig{
	{"Лучшее качество", []string{"bestvideo+bestaudio/best", "bestvideo+bestaudio", "best"}},
	{"360p", []string{"bestvideo[height<=360]+bestaudio/best[height<=360]", "best[height<=360]", "worst"}},
	{"MP3", nil},
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
	EvStart    DlEventType = iota
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
	Stem   string
	Label  string
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	OK     bool
}

func group(re *regexp.Regexp, m []string, name string) string {
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
	cmd := exec.Command(YtdlpBin, append([]string{"--newline", "--no-warnings"}, args...)...)
	pipe, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err != nil || cmd.Start() != nil {
		return false
	}
	sc := bufio.NewScanner(pipe)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case destRE.MatchString(line):
			m := destRE.FindStringSubmatch(line)
			stem := strings.TrimSpace(m[1])
			stem = numRE.ReplaceAllString(filepath.Base(strings.TrimSuffix(stem, filepath.Ext(stem))), "")
			if r := []rune(stem); len(r) > 58 {
				stem = string(r[:58])
			}
			ch <- DlUpdate{Type: EvDest, Slot: slot, Stem: stem}
		case dlRE.MatchString(line):
			m := dlRE.FindStringSubmatch(line)
			pct, _ := strconv.ParseFloat(group(dlRE, m, "pct"), 64)
			size, _ := strconv.ParseFloat(group(dlRE, m, "size"), 64)
			totalB := int64(size * float64(unitToMult(group(dlRE, m, "unit"))))
			speed := ""
			if sm := speedRE.FindStringSubmatch(line); sm != nil {
				speed = group(speedRE, sm, "speed")
			}
			ch <- DlUpdate{Type: EvProgress, Slot: slot,
				Pct: pct, DoneB: int64(float64(totalB) * pct / 100), TotalB: totalB, Speed: speed}
		case procRE.MatchString(line):
			m := procRE.FindStringSubmatch(line)
			label := "слияние видео+аудио (ffmpeg)…"
			if strings.Contains(strings.ToLower(m[1]), "audio") {
				label = "конвертация в MP3 (ffmpeg)…"
			}
			ch <- DlUpdate{Type: EvProc, Slot: slot, Label: label}
		}
	}
	cmd.Wait()
	return cmd.ProcessState.ExitCode() == 0
}

func buildArgs(cfg QualityConfig, url, tmpl, fmt_ string, extra []string) []string {
	args := ffmpegArgs()
	if len(cfg.FmtChain) == 0 {
		args = append(args, "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0")
	} else {
		f := fmt_
		if f == "" {
			f = cfg.FmtChain[0]
		}
		args = append(args, "-f", f, "--merge-output-format", "mp4")
	}
	return append(append(args, "-o", tmpl, "--windows-filenames"), append(extra, url)...)
}

func runWithFallback(slot int, cfg QualityConfig, url, tmpl string, extra []string, ch chan<- DlUpdate) bool {
	if len(cfg.FmtChain) == 0 {
		return streamYtdlp(slot, buildArgs(cfg, url, tmpl, "", extra), ch)
	}
	for i, fmt_ := range cfg.FmtChain {
		if i > 0 {
			ch <- DlUpdate{Type: EvFallback, Slot: slot,
				Label: fmt.Sprintf("Запасной формат #%d: %s", i, fmt_)}
		}
		if streamYtdlp(slot, buildArgs(cfg, url, tmpl, fmt_, extra), ch) {
			return true
		}
	}
	return false
}

func StartDownload(cfg QualityConfig, url string, forceSingle bool,
	plInfo *PlaylistInfo, entries []PlaylistEntry, workers int, ch chan<- DlUpdate) {

	go func() {
		var wg sync.WaitGroup
		defer func() { wg.Wait(); close(ch) }()

		if plInfo == nil || forceSingle || len(entries) == 0 {
			ok := runWithFallback(0, cfg, url,
				filepath.Join(DlDir, "%(title)s.%(ext)s"),
				[]string{"--no-playlist"}, ch)
			ch <- DlUpdate{Type: EvDone, OK: ok}
			return
		}

		plDir := filepath.Join(DlDir, SanitizeDirname(plInfo.Title))
		if err := os.MkdirAll(plDir, 0o755); err != nil {
			plDir = filepath.Join(DlDir, "playlist")
			os.MkdirAll(plDir, 0o755)
		}

		slotCh := make(chan int, workers)
		for i := range workers {
			slotCh <- i
		}
		for _, e := range entries {
			wg.Add(1)
			go func() {
				defer wg.Done()
				slot := <-slotCh
				defer func() {
					time.Sleep(300 * time.Millisecond)
					ch <- DlUpdate{Type: EvReset, Slot: slot}
					slotCh <- slot
				}()
				ch <- DlUpdate{Type: EvStart, Slot: slot, Stem: e.Title}
				tmpl := filepath.Join(plDir, fmt.Sprintf("%03d - %%(title)s.%%(ext)s", e.Index))
				ok := runWithFallback(slot, cfg, e.URL, tmpl, []string{"--no-playlist"}, ch)
				ch <- DlUpdate{Type: EvDone, Slot: slot, OK: ok}
			}()
		}
	}()
}

type UpdateInfo struct {
	Latest string
	DlURL  string
}

func assetName() string {
	switch {
	case IsWindows:
		return "VolRenDownloader.exe"
	case Arch == "arm64" || Arch == "aarch64":
		return "VolRenDownloader_linux_arm64"
	default:
		return "VolRenDownloader_linux_amd64"
	}
}

func CheckUpdate() *UpdateInfo {
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET",
		"https://api.github.com/repos/"+GithubRepo+"/releases/latest", nil)
	req.Header.Set("User-Agent", "VolRenDownloader/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var data map[string]any
	if json.NewDecoder(resp.Body).Decode(&data) != nil {
		return nil
	}
	latest := strings.TrimPrefix(strVal(data, "tag_name", ""), "v")
	if latest == "" || !versionGT(latest, Version) {
		return nil
	}
	assets, _ := data["assets"].([]any)
	want := assetName()
	for _, a := range assets {
		if asset, ok := a.(map[string]any); ok {
			if strVal(asset, "name", "") == want {
				return &UpdateInfo{Latest: latest, DlURL: strVal(asset, "browser_download_url", "")}
			}
		}
	}
	return nil
}

func ApplyUpdate(info *UpdateInfo, ch chan<- FileProgress) error {
	exe, _ := os.Executable()
	dest, _ := filepath.Abs(exe)
	var tmp string
	if IsWindows {
		tmp = strings.TrimSuffix(dest, ".exe") + ".new.exe"
	} else {
		tmp = dest + ".new"
	}
	if err := DownloadFile(info.DlURL, tmp, ch); err != nil {
		os.Remove(tmp)
		return err
	}
	return applyUpdatePlatform(tmp, dest)
}

func versionGT(a, b string) bool {
	parse := func(s string) [3]int {
		var v [3]int
		for i, p := range strings.SplitN(s, ".", 3) {
			v[i], _ = strconv.Atoi(p)
		}
		return v
	}
	av, bv := parse(a), parse(b)
	for i := range 3 {
		if av[i] != bv[i] {
			return av[i] > bv[i]
		}
	}
	return false
}

type SessionItem struct{ Label, URL string; OK bool }

type Session struct {
	Success int
	Failed  int
	Items   []SessionItem
}

func (s *Session) Record(label, url string, ok bool) {
	if ok {
		s.Success++
	} else {
		s.Failed++
	}
	s.Items = append(s.Items, SessionItem{label, url, ok})
}

var (
	YtRE              = regexp.MustCompile(`(?i)(youtube\.com/(watch\?.*v=|shorts/|live/|playlist\?list=)|youtu\.be/)[\w\-]+`)
	PlaylistRE        = regexp.MustCompile(`(?i)(youtube\.com/playlist\?|[?&]list=[\w\-]{10,})`)
	VideoInPlaylistRE = regexp.MustCompile(`(?i)youtube\.com/watch\?.*v=[\w\-]{11}.*[?&]list=`)
)

func IsPlaylistURL(url string) bool { return PlaylistRE.MatchString(url) }
