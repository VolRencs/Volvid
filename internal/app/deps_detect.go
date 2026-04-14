package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cookiesStatusActive          = "active"
	cookiesStatusNoProfile       = "browser found but no usable profile"
	cookiesStatusBrowserNotFound = "browser not found"
	jsRuntimeStatusActive        = "active"
	jsRuntimeStatusNotFound      = "not found"
)

const (
	ytdlpLineStart    = "__VRDL_START__"
	ytdlpLineProgress = "__VRDL_PROGRESS__"
	ytdlpLinePost     = "__VRDL_POST__"
	ytdlpLineMoved    = "__VRDL_MOVED__"
)

type cookieBrowserFamily string

const (
	cookieFamilyFirefox  cookieBrowserFamily = "firefox"
	cookieFamilyChromium cookieBrowserFamily = "chromium"
)

type cookieBrowserSpec struct {
	Browser          string
	Family           cookieBrowserFamily
	Roots            []string
	SupportsProfiles bool
}

type cookieCandidate struct {
	Browser    string
	Family     cookieBrowserFamily
	Profile    string
	CookiePath string
	ModTime    time.Time
}

type depsDetectCall struct {
	done       chan struct{}
	result     CheckDepsResult
	generation uint64
}

var (
	firefoxUserAgentOnce  sync.Once
	firefoxUserAgentCache string
	depsCacheMu           sync.Mutex
	depsCache             CheckDepsResult
	depsCacheReady        bool
	depsCacheFlight       *depsDetectCall
	depsCacheGeneration   uint64
)

func DetectDeps() CheckDepsResult {
	return loadDeps(false)
}

func RefreshDeps() CheckDepsResult {
	return loadDeps(true)
}

func loadDeps(force bool) CheckDepsResult {
	depsCacheMu.Lock()
	if depsCacheReady && !force {
		deps := depsCache
		depsCacheMu.Unlock()
		return deps
	}
	if call := depsCacheFlight; call != nil && (!force || call.generation == depsCacheGeneration) {
		depsCacheMu.Unlock()
		<-call.done
		return call.result
	}

	call := &depsDetectCall{
		done:       make(chan struct{}),
		generation: depsCacheGeneration,
	}
	depsCacheFlight = call
	depsCacheMu.Unlock()

	deps := detectDeps(true)

	depsCacheMu.Lock()
	if depsCacheFlight == call {
		depsCacheFlight = nil
	}
	if depsCacheGeneration == call.generation {
		depsCache = deps
		depsCacheReady = true
	}
	call.result = deps
	close(call.done)
	depsCacheMu.Unlock()
	return deps
}

func InvalidateDepsCache() {
	depsCacheMu.Lock()
	depsCache = CheckDepsResult{}
	depsCacheReady = false
	depsCacheGeneration++
	depsCacheMu.Unlock()
}

func detectDeps(withVersions bool) CheckDepsResult {
	ytdlp := detectExecutableDependency(
		"ytdlp",
		"yt-dlp",
		true,
		true,
		[]string{"yt-dlp"},
		YtdlpBin,
		[]string{"--version"},
		firstNonEmptyLine,
		withVersions,
	)
	ffmpeg := detectExecutableDependency(
		"ffmpeg",
		"ffmpeg",
		true,
		true,
		[]string{"ffmpeg"},
		FFmpegBin,
		[]string{"-version"},
		ffmpegVersionFromLine,
		withVersions,
	)
	node := detectExecutableDependency(
		"node",
		"node",
		false,
		true,
		[]string{"node"},
		NodeBin,
		[]string{"--version"},
		firstNonEmptyLine,
		withVersions,
	)

	deps := CheckDepsResult{
		YTDLP:  ytdlp,
		FFmpeg: ffmpeg,
		Node:   node,
	}
	deps.Cookies = detectBrowserCookies(currentUserHome(), currentGOOS())
	deps.Runtime = detectJSRuntime(node)
	return deps
}

func detectExecutableDependency(
	key, name string,
	required, downloadable bool,
	lookNames []string,
	managedPath string,
	versionArgs []string,
	parseVersion func(string) string,
	withVersion bool,
) DependencyInfo {
	dep := DependencyInfo{
		Key:          key,
		Name:         name,
		Required:     required,
		Downloadable: downloadable,
		Source:       DepMissing,
	}

	if path, ok := firstLookPath(lookNames...); ok {
		dep.Path = absoluteIfPossible(path)
		dep.Source = DepSystem
		dep.Available = true
	} else if pathExists(managedPath) {
		dep.Path = managedPath
		dep.Source = DepManaged
		dep.Managed = true
		dep.Available = true
	}

	if dep.Available && withVersion {
		line := commandVersionLine(dep.Path, versionArgs...)
		dep.Version = strings.TrimSpace(parseVersion(line))
	}

	return dep
}

func firstLookPath(names ...string) (string, bool) {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		path, err := exec.LookPath(name)
		if err == nil && strings.TrimSpace(path) != "" {
			return path, true
		}
	}
	return "", false
}

func currentUserHome() string {
	home, _ := os.UserHomeDir()
	return strings.TrimSpace(home)
}

func currentGOOS() string {
	return strings.ToLower(strings.TrimSpace(runtime.GOOS))
}

func detectBrowserCookies(home, goos string) BrowserCookiesInfo {
	candidates, foundRoots := collectCookieCandidates(cookieBrowserSpecs(home, goos))
	if len(candidates) > 0 {
		best := newestCookieCandidate(candidates)
		return BrowserCookiesInfo{
			Status:  cookiesStatusActive,
			Browser: best.Browser,
			Profile: best.Profile,
		}
	}
	if foundRoots {
		return BrowserCookiesInfo{Status: cookiesStatusNoProfile}
	}
	return BrowserCookiesInfo{Status: cookiesStatusBrowserNotFound}
}

func detectJSRuntime(node DependencyInfo) JSRuntimeInfo {
	if !node.Available {
		return JSRuntimeInfo{Status: jsRuntimeStatusNotFound}
	}
	return JSRuntimeInfo{
		Status: jsRuntimeStatusActive,
		Name:   "node",
		Path:   node.Path,
	}
}

func cookieBrowserSpecs(home, goos string) []cookieBrowserSpec {
	switch goos {
	case "linux":
		return linuxCookieBrowserSpecs(home)
	case "windows":
		return windowsCookieBrowserSpecs()
	default:
		return nil
	}
}

func collectCookieCandidates(specs []cookieBrowserSpec) ([]cookieCandidate, bool) {
	out := make([]cookieCandidate, 0, len(specs))
	seen := make(map[string]struct{}, len(specs)*2)
	foundRoots := false

	for _, spec := range specs {
		roots, ok := resolveCookieRoots(spec.Roots)
		if ok {
			foundRoots = true
		}
		for _, candidate := range browserCookieCandidates(spec, roots) {
			key := filepath.Clean(candidate.CookiePath)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, candidate)
		}
	}

	return out, foundRoots
}

func browserCookieCandidates(spec cookieBrowserSpec, roots []string) []cookieCandidate {
	switch spec.Family {
	case cookieFamilyFirefox:
		return firefoxCookieCandidates(spec.Browser, roots)
	case cookieFamilyChromium:
		return chromiumCookieCandidates(spec.Browser, roots, spec.SupportsProfiles)
	default:
		return nil
	}
}

func newestCookieCandidate(candidates []cookieCandidate) cookieCandidate {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.ModTime.After(best.ModTime) {
			best = candidate
		}
	}
	return best
}

func resolveCookieRoots(roots []string) ([]string, bool) {
	resolved := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	found := false

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		matches := expandCookieRoot(root)
		if len(matches) > 0 {
			found = true
		}
		for _, match := range matches {
			key := filepath.Clean(match)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			resolved = append(resolved, key)
		}
	}

	return resolved, found
}

func expandCookieRoot(root string) []string {
	if !strings.ContainsAny(root, "*?[") {
		if pathExists(root) {
			return []string{absoluteIfPossible(root)}
		}
		return nil
	}

	matches, err := filepath.Glob(root)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if pathExists(match) {
			out = append(out, absoluteIfPossible(match))
		}
	}
	return out
}

func firefoxCookieCandidates(browser string, roots []string) []cookieCandidate {
	out := make([]cookieCandidate, 0, len(roots))
	seen := make(map[string]struct{}, len(roots)*2)

	for _, root := range roots {
		for _, pattern := range []string{
			filepath.Join(root, "cookies.sqlite"),
			filepath.Join(root, "*", "cookies.sqlite"),
			filepath.Join(root, "Profiles", "*", "cookies.sqlite"),
		} {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, match := range matches {
				info, ok := readableCookieFile(match)
				if !ok {
					continue
				}
				key := filepath.Clean(match)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, cookieCandidate{
					Browser:    browser,
					Family:     cookieFamilyFirefox,
					Profile:    absoluteIfPossible(filepath.Dir(match)),
					CookiePath: absoluteIfPossible(match),
					ModTime:    info.ModTime(),
				})
			}
		}
	}

	return out
}

func chromiumCookieCandidates(browser string, roots []string, supportsProfiles bool) []cookieCandidate {
	out := make([]cookieCandidate, 0, len(roots))
	seen := make(map[string]struct{}, len(roots)*2)

	for _, root := range roots {
		for _, profileDir := range chromiumProfileDirs(root, supportsProfiles) {
			cookiePath, info, ok := chromiumCookieFile(profileDir)
			if !ok {
				continue
			}
			key := filepath.Clean(cookiePath)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, cookieCandidate{
				Browser:    browser,
				Family:     cookieFamilyChromium,
				Profile:    absoluteIfPossible(profileDir),
				CookiePath: absoluteIfPossible(cookiePath),
				ModTime:    info.ModTime(),
			})
		}
	}

	return out
}

func chromiumProfileDirs(root string, supportsProfiles bool) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	if !supportsProfiles {
		return []string{root}
	}

	out := []string{root}
	for _, fixed := range []string{"Default", "Guest Profile"} {
		out = append(out, filepath.Join(root, fixed))
	}
	if matches, err := filepath.Glob(filepath.Join(root, "Profile *")); err == nil {
		out = append(out, matches...)
	}
	return uniquePaths(out)
}

func chromiumCookieFile(profileDir string) (string, os.FileInfo, bool) {
	for _, path := range []string{
		filepath.Join(profileDir, "Network", "Cookies"),
		filepath.Join(profileDir, "Cookies"),
	} {
		if info, ok := readableCookieFile(path); ok {
			return path, info, true
		}
	}
	return "", nil, false
}

func readableCookieFile(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	_ = file.Close()
	return info, true
}

func linuxCookieBrowserSpecs(home string) []cookieBrowserSpec {
	configHome := configHomeDir(home)
	chromeConfigHome := chromeConfigHomeDir(home)
	chromeUserDataDir := chromeUserDataDir(home)

	return []cookieBrowserSpec{
		{
			Browser:          "firefox",
			Family:           cookieFamilyFirefox,
			SupportsProfiles: true,
			Roots: uniquePaths([]string{
				filepath.Join(configHome, "mozilla", "firefox"),
				filepath.Join(home, ".mozilla", "firefox"),
				filepath.Join(home, ".var", "app", "org.mozilla.firefox", "config", "mozilla", "firefox"),
				filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
				filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
			}),
		},
		{
			Browser:          "chrome",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots: uniquePaths([]string{
				chromeUserDataDir,
				filepath.Join(chromeConfigHome, "google-chrome"),
				filepath.Join(configHome, "google-chrome"),
				filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome"),
			}),
		},
		{
			Browser:          "chromium",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots: uniquePaths([]string{
				chromeUserDataDir,
				filepath.Join(chromeConfigHome, "chromium"),
				filepath.Join(configHome, "chromium"),
				filepath.Join(home, ".var", "app", "org.chromium.Chromium", "config", "chromium"),
			}),
		},
		{
			Browser:          "edge",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(configHome, "microsoft-edge")},
		},
		{
			Browser:          "brave",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots: uniquePaths([]string{
				filepath.Join(configHome, "BraveSoftware", "Brave-Browser"),
				filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser"),
			}),
		},
		{
			Browser:          "vivaldi",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(configHome, "vivaldi")},
		},
		{
			Browser:          "opera",
			Family:           cookieFamilyChromium,
			SupportsProfiles: false,
			Roots:            []string{filepath.Join(configHome, "opera")},
		},
	}
}

func windowsCookieBrowserSpecs() []cookieBrowserSpec {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))

	return []cookieBrowserSpec{
		{
			Browser:          "firefox",
			Family:           cookieFamilyFirefox,
			SupportsProfiles: true,
			Roots: uniquePaths([]string{
				filepath.Join(appData, "Mozilla", "Firefox", "Profiles"),
				filepath.Join(localAppData, "Packages", "Mozilla.Firefox_*", "LocalCache", "Roaming", "Mozilla", "Firefox", "Profiles"),
			}),
		},
		{
			Browser:          "chrome",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(localAppData, "Google", "Chrome", "User Data")},
		},
		{
			Browser:          "chromium",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(localAppData, "Chromium", "User Data")},
		},
		{
			Browser:          "edge",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(localAppData, "Microsoft", "Edge", "User Data")},
		},
		{
			Browser:          "brave",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")},
		},
		{
			Browser:          "vivaldi",
			Family:           cookieFamilyChromium,
			SupportsProfiles: true,
			Roots:            []string{filepath.Join(localAppData, "Vivaldi", "User Data")},
		},
		{
			Browser:          "opera",
			Family:           cookieFamilyChromium,
			SupportsProfiles: false,
			Roots:            []string{filepath.Join(appData, "Opera Software", "Opera Stable")},
		},
	}
}

func configHomeDir(home string) string {
	if dir := resolveUserPath(home, os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return dir
	}
	return filepath.Join(home, ".config")
}

func chromeConfigHomeDir(home string) string {
	if dir := resolveUserPath(home, os.Getenv("CHROME_CONFIG_HOME")); dir != "" {
		return dir
	}
	return configHomeDir(home)
}

func chromeUserDataDir(home string) string {
	return resolveUserPath(home, os.Getenv("CHROME_USER_DATA_DIR"))
}

func resolveUserPath(home, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~"+string(os.PathSeparator)) {
		return absoluteIfPossible(filepath.Join(home, strings.TrimPrefix(raw, "~"+string(os.PathSeparator))))
	}
	if filepath.IsAbs(raw) {
		return absoluteIfPossible(raw)
	}
	if home == "" {
		return absoluteIfPossible(raw)
	}
	return absoluteIfPossible(filepath.Join(home, raw))
}

func uniquePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := filepath.Clean(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}

	return out
}

func resolveRuntimeDeps() CheckDepsResult {
	return detectDeps(false)
}

func ytdlpCommandArgsFor(deps CheckDepsResult, base []string) []string {
	args := make([]string, 0, len(base)+6)
	if deps.Cookies.Status == cookiesStatusActive {
		cookieArg := strings.TrimSpace(deps.Cookies.Browser)
		if profile := cookiesProfileArg(deps.Cookies); profile != "" {
			cookieArg += ":" + profile
		}
		args = append(args, "--cookies-from-browser", cookieArg)
	}
	if deps.Runtime.Status == jsRuntimeStatusActive && strings.TrimSpace(deps.Runtime.Path) != "" {
		args = append(args, "--js-runtimes", "node:"+deps.Runtime.Path)
	}
	if ua := runtimeUserAgent(deps); ua != "" {
		args = append(args, "--user-agent", ua)
	}
	args = append(args, base...)
	return args
}

func cookiesProfileArg(info BrowserCookiesInfo) string {
	if currentGOOS() == "linux" && strings.EqualFold(strings.TrimSpace(info.Browser), "firefox") {
		return ""
	}
	profile := strings.TrimSpace(info.Profile)
	if profile == "" {
		return ""
	}
	if currentGOOS() == "windows" {
		return filepath.Base(profile)
	}
	return profile
}

func runtimeUserAgent(deps CheckDepsResult) string {
	if deps.Cookies.Status != cookiesStatusActive {
		return ""
	}
	if currentGOOS() != "linux" || !strings.EqualFold(strings.TrimSpace(deps.Cookies.Browser), "firefox") {
		return ""
	}
	return firefoxUserAgent()
}

func firefoxUserAgent() string {
	firefoxUserAgentOnce.Do(func() {
		firefoxUserAgentCache = buildFirefoxUserAgent()
	})
	return firefoxUserAgentCache
}

func buildFirefoxUserAgent() string {
	version := detectFirefoxVersion()
	if version == "" {
		version = "128.0"
	}
	platform := firefoxUAPlatform()
	return "Mozilla/5.0 (" + platform + "; rv:" + version + ") Gecko/20100101 Firefox/" + version
}

func detectFirefoxVersion() string {
	bin, ok := firstLookPath("firefox", "firefox-bin")
	if !ok {
		return ""
	}
	return firefoxVersionFromLine(commandVersionLine(bin, "--version"))
}

func firefoxVersionFromLine(line string) string {
	for _, field := range strings.Fields(strings.TrimSpace(line)) {
		field = strings.Trim(field, " \t\r\n,;:()[]{}\"'")
		if field == "" || field[0] < '0' || field[0] > '9' {
			continue
		}
		return field
	}
	return ""
}

func firefoxUAPlatform() string {
	switch Arch {
	case "arm64":
		return "X11; Linux aarch64"
	default:
		return "X11; Linux x86_64"
	}
}

func ytdlpOutput(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return ytdlpOutputFor(ctx, timeout, resolveRuntimeDeps(), args...)
}

func startYTDLPMergedOutputCommand(
	ctx context.Context,
	timeout time.Duration,
	args ...string,
) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	return startYTDLPMergedOutputCommandFor(ctx, timeout, resolveRuntimeDeps(), args...)
}

func ytdlpOutputFor(ctx context.Context, timeout time.Duration, deps CheckDepsResult, args ...string) ([]byte, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, fmt.Errorf("yt-dlp is required")
	}
	return commandOutput(ctx, timeout, bin, ytdlpCommandArgsFor(deps, args)...)
}

func startYTDLPMergedOutputCommandFor(
	ctx context.Context,
	timeout time.Duration,
	deps CheckDepsResult,
	args ...string,
) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, nil, nil, nil, fmt.Errorf("yt-dlp is required")
	}
	return startMergedOutputCommand(ctx, timeout, bin, ytdlpCommandArgsFor(deps, args)...)
}

func commandVersionLine(bin string, args ...string) string {
	if strings.TrimSpace(bin) == "" {
		return ""
	}
	out, _ := commandCombinedOutput(context.Background(), versionProbeTimeout, bin, args...)
	return firstNonEmptyLine(string(out))
}

func firstNonEmptyLine(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func ffmpegVersionFromLine(line string) string {
	const prefix = "ffmpeg version "

	lower := strings.ToLower(line)
	idx := strings.Index(lower, prefix)
	if idx < 0 {
		return strings.TrimSpace(line)
	}

	rest := strings.TrimSpace(line[idx+len(prefix):])
	if rest == "" {
		return ""
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return strings.TrimSpace(line)
	}

	return strings.Trim(fields[0], " \t\r\n,;:()[]{}\"'")
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func absoluteIfPossible(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func parseDownloadInt(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseDownloadFloat(raw string) float64 {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return n
}

func YtdlpURL() string {
	switch {
	case IsWindows:
		return ytdlpBase + "yt-dlp.exe"
	case Arch == "arm64":
		return ytdlpBase + "yt-dlp_linux_aarch64"
	default:
		return ytdlpBase + "yt-dlp_linux"
	}
}

func InstallYtDlpFor(l Locale, ch chan<- FileProgress) error {
	if err := os.MkdirAll(DepsDir, 0o755); err != nil {
		return fmt.Errorf("создание DepsDir: %w", err)
	}
	if err := DownloadFile(YtdlpURL(), YtdlpBin, l, ch); err != nil {
		return err
	}
	if !IsWindows {
		if err := os.Chmod(YtdlpBin, 0o755); err != nil {
			return fmt.Errorf("chmod yt-dlp: %w", err)
		}
	}
	if detectExecutableDependency("ytdlp", "yt-dlp", true, true, nil, YtdlpBin, []string{"--version"}, firstNonEmptyLine, true).Version == "" {
		return fmt.Errorf("бинарник yt-dlp скачан, но не запускается")
	}
	return nil
}
