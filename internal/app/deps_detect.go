package app

import (
	"bufio"
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

type browserProfile struct {
	Browser string
	Profile string
}

type firefoxProfile struct {
	Name     string
	Path     string
	Default  bool
	Relative bool
}

var (
	firefoxUserAgentOnce  sync.Once
	firefoxUserAgentCache string
)

func DetectDeps() CheckDepsResult {
	deps := detectDeps(true)
	applyDetectedPaths(deps)
	return deps
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

func applyDetectedPaths(deps CheckDepsResult) {
	YtdlpResolved = deps.YTDLP.Path
	FFmpegResolved = deps.FFmpeg.Path
	NodeResolved = deps.Node.Path
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
	switch goos {
	case "linux":
		if profile, ok := detectLinuxFirefoxProfile(home); ok {
			return BrowserCookiesInfo{
				Status:  cookiesStatusActive,
				Browser: profile.Browser,
				Profile: profile.Profile,
			}
		}
		if profile, ok := detectLinuxChromiumProfile(home); ok {
			return BrowserCookiesInfo{
				Status:  cookiesStatusActive,
				Browser: profile.Browser,
				Profile: profile.Profile,
			}
		}
		if linuxBrowserRootExists(home) {
			return BrowserCookiesInfo{Status: cookiesStatusNoProfile}
		}
	case "windows":
		if profile, ok := detectWindowsFirefoxProfile(); ok {
			return BrowserCookiesInfo{
				Status:  cookiesStatusActive,
				Browser: profile.Browser,
				Profile: profile.Profile,
			}
		}
		if profile, ok := detectWindowsChromiumProfile(); ok {
			return BrowserCookiesInfo{
				Status:  cookiesStatusActive,
				Browser: profile.Browser,
				Profile: profile.Profile,
			}
		}
		if windowsBrowserRootExists() {
			return BrowserCookiesInfo{Status: cookiesStatusNoProfile}
		}
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

func linuxBrowserRootExists(home string) bool {
	for _, root := range linuxFirefoxRoots(home) {
		if pathExists(root) {
			return true
		}
	}
	for _, candidate := range linuxChromiumCandidates(home) {
		if pathExists(candidate.Root) {
			return true
		}
	}
	return false
}

func windowsBrowserRootExists() bool {
	for _, root := range windowsFirefoxRoots() {
		if pathExists(root) {
			return true
		}
	}
	for _, candidate := range windowsChromiumCandidates() {
		if pathExists(candidate.Root) {
			return true
		}
	}
	return false
}

func detectLinuxFirefoxProfile(home string) (browserProfile, bool) {
	return detectFirefoxProfileInRoots("firefox", linuxFirefoxRoots(home))
}

func detectWindowsFirefoxProfile() (browserProfile, bool) {
	return detectFirefoxProfileInRoots("firefox", windowsFirefoxRoots())
}

func detectFirefoxProfileInRoots(browser string, roots []string) (browserProfile, bool) {
	foundRoot := false
	var newest browserProfile
	var newestTime time.Time

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" || !pathExists(root) {
			continue
		}
		foundRoot = true

		profiles := readFirefoxProfiles(root)
		for _, profile := range profiles {
			cookiesPath := filepath.Join(profile.Path, "cookies.sqlite")
			info, err := os.Stat(cookiesPath)
			if err != nil || info.IsDir() {
				continue
			}
			if profile.Default {
				return browserProfile{Browser: browser, Profile: profile.Path}, true
			}
			if newest.Profile == "" || info.ModTime().After(newestTime) {
				newest = browserProfile{Browser: browser, Profile: profile.Path}
				newestTime = info.ModTime()
			}
		}
	}

	if newest.Profile != "" {
		return newest, true
	}
	if foundRoot {
		return browserProfile{}, false
	}
	return browserProfile{}, false
}

func linuxFirefoxRoots(home string) []string {
	return []string{
		filepath.Join(home, ".mozilla", "firefox"),
		filepath.Join(home, ".config", "mozilla", "firefox"),
		filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
		filepath.Join(home, ".var", "app", "org.mozilla.firefox", "config", "mozilla", "firefox"),
		filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
	}
}

func windowsFirefoxRoots() []string {
	return []string{
		filepath.Join(os.Getenv("APPDATA"), "Mozilla", "Firefox"),
	}
}

func readFirefoxProfiles(root string) []firefoxProfile {
	iniPath := filepath.Join(root, "profiles.ini")
	if profiles, ok := readFirefoxProfilesINI(root, iniPath); ok && len(profiles) > 0 {
		return profiles
	}
	return fallbackFirefoxProfiles(root)
}

func readFirefoxProfilesINI(root, iniPath string) ([]firefoxProfile, bool) {
	file, err := os.Open(iniPath)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	var (
		section      string
		profiles     []firefoxProfile
		current      firefoxProfile
		defaultPaths []string
		hasData      bool
	)

	normalizeProfilePath := func(path string, relative bool) string {
		if relative {
			return filepath.Join(root, filepath.FromSlash(path))
		}
		return filepath.Clean(filepath.FromSlash(path))
	}

	normalizeInstallDefault := func(path string) string {
		path = strings.TrimSpace(path)
		if path == "" {
			return ""
		}
		if filepath.IsAbs(path) {
			return filepath.Clean(filepath.FromSlash(path))
		}
		return filepath.Join(root, filepath.FromSlash(path))
	}

	flush := func() {
		if !strings.HasPrefix(section, "Profile") || strings.TrimSpace(current.Path) == "" {
			current = firefoxProfile{}
			return
		}
		current.Path = normalizeProfilePath(current.Path, current.Relative)
		profiles = append(profiles, current)
		current = firefoxProfile{}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		hasData = true
		if strings.HasPrefix(section, "Install") {
			if key == "Default" {
				if path := normalizeInstallDefault(value); path != "" {
					defaultPaths = append(defaultPaths, path)
				}
			}
			continue
		}

		switch key {
		case "Name":
			current.Name = value
		case "Path":
			if value == "" {
				continue
			}
			current.Path = value
		case "Default":
			current.Default = value == "1"
		case "IsRelative":
			current.Relative = value == "1"
		}
	}
	flush()

	if len(defaultPaths) > 0 {
		defaultSet := make(map[string]struct{}, len(defaultPaths))
		for _, path := range defaultPaths {
			defaultSet[filepath.Clean(path)] = struct{}{}
		}
		for i := range profiles {
			if _, ok := defaultSet[filepath.Clean(profiles[i].Path)]; ok {
				profiles[i].Default = true
			}
		}
	}
	return profiles, hasData
}

func fallbackFirefoxProfiles(root string) []firefoxProfile {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	profiles := make([]firefoxProfile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if !pathExists(filepath.Join(dir, "cookies.sqlite")) {
			continue
		}
		profiles = append(profiles, firefoxProfile{Name: entry.Name(), Path: dir})
	}
	return profiles
}

type chromiumRoot struct {
	Browser string
	Root    string
}

func detectLinuxChromiumProfile(home string) (browserProfile, bool) {
	return detectChromiumProfile(linuxChromiumCandidates(home))
}

func detectWindowsChromiumProfile() (browserProfile, bool) {
	return detectChromiumProfile(windowsChromiumCandidates())
}

func detectChromiumProfile(candidates []chromiumRoot) (browserProfile, bool) {
	var (
		best     browserProfile
		bestTime time.Time
	)

	for _, candidate := range candidates {
		profile, modTime, ok := detectChromiumProfileForRoot(candidate)
		if !ok {
			continue
		}
		if best.Profile == "" || modTime.After(bestTime) {
			best = profile
			bestTime = modTime
		}
	}

	return best, best.Profile != ""
}

func detectChromiumProfileForRoot(candidate chromiumRoot) (browserProfile, time.Time, bool) {
	root := strings.TrimSpace(candidate.Root)
	if root == "" || !pathExists(root) {
		return browserProfile{}, time.Time{}, false
	}

	for _, name := range []string{"Default"} {
		dir := filepath.Join(root, name)
		if modTime, ok := chromiumCookieProfileTime(dir); ok {
			return browserProfile{Browser: candidate.Browser, Profile: dir}, modTime, true
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return browserProfile{}, time.Time{}, false
	}

	var (
		best     browserProfile
		bestTime time.Time
	)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "Profile ") && name != "Guest Profile" {
			continue
		}
		dir := filepath.Join(root, name)
		modTime, ok := chromiumCookieProfileTime(dir)
		if !ok {
			continue
		}
		if best.Profile == "" || modTime.After(bestTime) {
			best = browserProfile{Browser: candidate.Browser, Profile: dir}
			bestTime = modTime
		}
	}

	return best, bestTime, best.Profile != ""
}

func chromiumCookieProfileTime(dir string) (time.Time, bool) {
	for _, candidate := range []string{
		filepath.Join(dir, "Network", "Cookies"),
		filepath.Join(dir, "Cookies"),
	} {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return info.ModTime(), true
		}
	}
	return time.Time{}, false
}

func linuxChromiumCandidates(home string) []chromiumRoot {
	return []chromiumRoot{
		{Browser: "chrome", Root: filepath.Join(home, ".config", "google-chrome")},
		{Browser: "chromium", Root: filepath.Join(home, ".config", "chromium")},
		{Browser: "edge", Root: filepath.Join(home, ".config", "microsoft-edge")},
		{Browser: "brave", Root: filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser")},
		{Browser: "vivaldi", Root: filepath.Join(home, ".config", "vivaldi")},
		{Browser: "opera", Root: filepath.Join(home, ".config", "opera")},
		{Browser: "chrome", Root: filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome")},
		{Browser: "chromium", Root: filepath.Join(home, ".var", "app", "org.chromium.Chromium", "config", "chromium")},
		{Browser: "brave", Root: filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser")},
	}
}

func windowsChromiumCandidates() []chromiumRoot {
	local := os.Getenv("LOCALAPPDATA")
	appdata := os.Getenv("APPDATA")
	return []chromiumRoot{
		{Browser: "chrome", Root: filepath.Join(local, "Google", "Chrome", "User Data")},
		{Browser: "chromium", Root: filepath.Join(local, "Chromium", "User Data")},
		{Browser: "edge", Root: filepath.Join(local, "Microsoft", "Edge", "User Data")},
		{Browser: "brave", Root: filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data")},
		{Browser: "vivaldi", Root: filepath.Join(local, "Vivaldi", "User Data")},
		{Browser: "opera", Root: filepath.Join(appdata, "Opera Software")},
	}
}

func resolveRuntimeDeps() CheckDepsResult {
	deps := detectDeps(false)
	applyDetectedPaths(deps)
	return deps
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
	deps := resolveRuntimeDeps()
	if strings.TrimSpace(YtdlpResolved) == "" {
		return nil, fmt.Errorf("yt-dlp is required")
	}
	return commandOutput(ctx, timeout, YtdlpResolved, ytdlpCommandArgsFor(deps, args)...)
}

func startYTDLPMergedOutputCommand(
	ctx context.Context,
	timeout time.Duration,
	args ...string,
) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	deps := resolveRuntimeDeps()
	if strings.TrimSpace(YtdlpResolved) == "" {
		return nil, nil, nil, nil, fmt.Errorf("yt-dlp is required")
	}
	return startMergedOutputCommand(ctx, timeout, YtdlpResolved, ytdlpCommandArgsFor(deps, args)...)
}

func commandVersionLine(bin string, args ...string) string {
	if strings.TrimSpace(bin) == "" {
		return ""
	}
	for attempt := 0; attempt < versionProbeAttempts; attempt++ {
		out, _ := commandCombinedOutput(context.Background(), versionProbeTimeout, bin, args...)
		line := firstNonEmptyLine(string(out))
		if line != "" {
			return line
		}
		if attempt+1 < versionProbeAttempts {
			time.Sleep(versionProbeDelay)
		}
	}
	return ""
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

func InstallYtDlp(ch chan<- FileProgress) error {
	return InstallYtDlpFor(LoadLocale(), ch)
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
