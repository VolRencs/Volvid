package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	StatusActive          = "active"
	StatusNoProfile       = "browser found but no usable profile"
	StatusBrowserNotFound = "browser not found"
	StatusNotFound        = "not found"

	FamilyFirefox  = "firefox"
	FamilyChromium = "chromium"

	ytdlpLineStart    = "VRDL_START"
	ytdlpLineProgress = "VRDL_PROGRESS"
	ytdlpLinePost     = "VRDL_POST"
	ytdlpLineMoved    = "VRDL_MOVED"
)

type cookieBrowserSpec struct {
	Browser          string
	Family           string
	Roots            []string
	SupportsProfiles bool
}

type cookieCandidate struct {
	Browser    string
	Profile    string
	CookiePath string
	ModTime    time.Time
}

type depsDetectCall struct {
	done       chan struct{}
	result     CheckDepsResult
	generation uint64
}

const runtimeDepsTTL = 15 * time.Second

func DetectDeps(env *Env) CheckDepsResult  { return loadDeps(env, false) }
func RefreshDeps(env *Env) CheckDepsResult { return loadDeps(env, true) }

func loadDeps(env *Env, force bool) CheckDepsResult {
	env.depsCacheMu.Lock()
	if env.depsCacheReady && !force {
		deps := env.depsCache
		env.depsCacheMu.Unlock()
		return deps
	}
	if call := env.depsCacheFlight; call != nil && call.generation == env.depsCacheGeneration {
		env.depsCacheMu.Unlock()
		<-call.done
		return call.result
	}
	call := &depsDetectCall{
		done:       make(chan struct{}),
		generation: env.depsCacheGeneration,
	}
	env.depsCacheFlight = call
	env.depsCacheMu.Unlock()

	deps := detectDeps(env, true)

	env.depsCacheMu.Lock()
	if env.depsCacheFlight == call {
		env.depsCacheFlight = nil
	}
	if env.depsCacheGeneration == call.generation {
		env.depsCache = deps
		env.depsCacheReady = true
	}
	call.result = deps
	close(call.done)
	env.depsCacheMu.Unlock()
	return deps
}

func InvalidateDepsCache(env *Env) {
	env.depsCacheMu.Lock()
	env.depsCache = CheckDepsResult{}
	env.depsCacheReady = false
	env.depsCacheGeneration++
	env.runtimeDepsExpiry = time.Time{}
	env.depsCacheMu.Unlock()
}

type depSpec struct {
	Key          string
	Name         string
	Required     bool
	Downloadable bool
	LookNames    []string
	ManagedPath  string
	VersionArgs  []string
	ParseVersion func(string) string
}

func detectDeps(env *Env, withVersions bool) CheckDepsResult {
	ytdlp := detectExecutableDependency(depSpec{
		Key: "ytdlp", Name: "yt-dlp", Required: true, Downloadable: true,
		LookNames: []string{"yt-dlp"}, ManagedPath: env.YtdlpBin,
		VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine,
	}, withVersions)
	ffmpeg := detectExecutableDependency(depSpec{
		Key: "ffmpeg", Name: "ffmpeg", Required: true, Downloadable: true,
		LookNames: []string{"ffmpeg"}, ManagedPath: env.FFmpegBin,
		VersionArgs: []string{"-version"}, ParseVersion: ffmpegVersionFromLine,
	}, withVersions)
	node := detectExecutableDependency(depSpec{
		Key: "node", Name: "node", Required: false, Downloadable: true,
		LookNames: []string{"node"}, ManagedPath: env.NodeBin,
		VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine,
	}, withVersions)

	deps := CheckDepsResult{YTDLP: ytdlp, FFmpeg: ffmpeg, Node: node}
	deps.Cookies = detectBrowserCookies(currentUserHome(), currentGOOS())
	deps.Runtime = detectJSRuntime(node)
	return deps
}

func detectExecutableDependency(spec depSpec, withVersion bool) DependencyInfo {
	dep := DependencyInfo{Key: spec.Key, Name: spec.Name, Required: spec.Required, Downloadable: spec.Downloadable, Source: DepMissing}

	if path, ok := firstLookPath(spec.LookNames...); ok {
		dep.Path = absoluteIfPossible(path)
		dep.Source = DepSystem
		dep.Available = true
	} else if pathExists(spec.ManagedPath) {
		dep.Path = spec.ManagedPath
		dep.Source = DepManaged
		dep.Available = true
	}

	if dep.Available && withVersion {
		line := commandVersionLine(dep.Path, spec.VersionArgs...)
		dep.Version = strings.TrimSpace(spec.ParseVersion(line))
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
	return runtime.GOOS
}

func detectBrowserCookies(home, goos string) BrowserCookiesInfo {
	candidates, foundRoots := collectCookieCandidates(cookieBrowserSpecs(home, goos))
	if len(candidates) > 0 {
		return browserCookiesInfoFromCandidate(newestCookieCandidate(candidates), goos)
	}
	if foundRoots {
		return BrowserCookiesInfo{Status: StatusNoProfile}
	}
	return BrowserCookiesInfo{Status: StatusBrowserNotFound}
}

func browserCookiesInfoFromCandidate(candidate cookieCandidate, goos string) BrowserCookiesInfo {
	return BrowserCookiesInfo{
		Status:       StatusActive,
		Browser:      strings.TrimSpace(candidate.Browser),
		ProfileName:  cookieProfileName(candidate.Profile),
		CookiePath:   strings.TrimSpace(candidate.CookiePath),
		YTDLPProfile: ytdlpCookieProfileArg(candidate, goos),
	}
}

func cookieProfileName(profile string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return ""
	}
	return filepath.Base(profile)
}

func ytdlpCookieProfileArg(candidate cookieCandidate, goos string) string {
	profile := strings.TrimSpace(candidate.Profile)
	if profile == "" {
		return ""
	}
	if goos == "windows" {
		return filepath.Base(profile)
	}
	return profile
}

func detectJSRuntime(node DependencyInfo) JSRuntimeInfo {
	if !node.Available {
		return JSRuntimeInfo{Status: StatusNotFound}
	}
	return JSRuntimeInfo{Status: StatusActive, Name: "node", Path: node.Path}
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
	seen := make(map[string]struct{}, len(specs)*4)
	foundRoots := false

	for _, spec := range specs {
		roots, ok := resolveCookieRoots(spec.Roots)
		if ok {
			foundRoots = true
		}
		for _, candidate := range browserCookieCandidates(spec, roots) {
			key := filepath.Clean(candidate.CookiePath)
			if key == "." || key == "" {
				key = filepath.Clean(candidate.Profile)
			}
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
	case FamilyFirefox:
		return firefoxCookieCandidates(spec.Browser, roots)
	case FamilyChromium:
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
	for _, root := range roots {
		for _, profileDir := range firefoxProfileDirs(root) {
			cookiePath, modTime := firefoxProfileState(profileDir)
			out = append(out, cookieCandidate{
				Browser:    browser,
				Profile:    absoluteIfPossible(profileDir),
				CookiePath: cookiePath,
				ModTime:    modTime,
			})
		}
	}
	return out
}

func chromiumCookieCandidates(browser string, roots []string, supportsProfiles bool) []cookieCandidate {
	out := make([]cookieCandidate, 0, len(roots))
	for _, root := range roots {
		for _, profileDir := range chromiumProfileDirs(root, supportsProfiles) {
			cookiePath, modTime := chromiumProfileState(root, profileDir)
			out = append(out, cookieCandidate{
				Browser:    browser,
				Profile:    absoluteIfPossible(profileDir),
				CookiePath: cookiePath,
				ModTime:    modTime,
			})
		}
	}
	return out
}

func firefoxProfileDirs(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	out := make([]string, 0, 4)
	if firefoxProfileExists(root) {
		out = append(out, root)
	}
	for _, pattern := range []string{
		filepath.Join(root, "*.default*"),
		filepath.Join(root, "*", "cookies.sqlite"),
		filepath.Join(root, "Profiles", "*.default*"),
		filepath.Join(root, "Profiles", "*", "cookies.sqlite"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if filepath.Base(match) == "cookies.sqlite" {
				match = filepath.Dir(match)
			}
			if firefoxProfileExists(match) {
				out = append(out, match)
			}
		}
	}
	return uniquePaths(out)
}

func firefoxProfileExists(profileDir string) bool {
	if !pathIsDir(profileDir) {
		return false
	}
	return pathExists(filepath.Join(profileDir, "cookies.sqlite")) ||
		pathExists(filepath.Join(profileDir, "prefs.js"))
}

func firefoxProfileState(profileDir string) (string, time.Time) {
	cookiePath, cookieTime := profileCookieState(profileDir, "cookies.sqlite")
	if !cookieTime.IsZero() {
		return cookiePath, cookieTime
	}
	if info, ok := fileInfo(filepath.Join(profileDir, "prefs.js")); ok {
		return "", info.ModTime()
	}
	if info, ok := fileInfo(profileDir); ok {
		return "", info.ModTime()
	}
	return "", time.Time{}
}

func chromiumProfileDirs(root string, supportsProfiles bool) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	out := make([]string, 0, 4)
	if chromiumProfileExists(root) {
		out = append(out, root)
	}
	if !supportsProfiles {
		return uniquePaths(out)
	}
	for _, fixed := range []string{"Default", "Guest Profile", "System Profile"} {
		profile := filepath.Join(root, fixed)
		if chromiumProfileExists(profile) {
			out = append(out, profile)
		}
	}
	if matches, err := filepath.Glob(filepath.Join(root, "Profile *")); err == nil {
		for _, match := range matches {
			if chromiumProfileExists(match) {
				out = append(out, match)
			}
		}
	}
	return uniquePaths(out)
}

func chromiumProfileExists(profileDir string) bool {
	if !pathIsDir(profileDir) {
		return false
	}
	for _, marker := range []string{"Preferences", filepath.Join("Network", "Cookies"), "Cookies"} {
		if pathExists(filepath.Join(profileDir, marker)) {
			return true
		}
	}
	return false
}

func chromiumProfileState(root, profileDir string) (string, time.Time) {
	cookiePath, cookieTime := profileCookieState(
		profileDir,
		filepath.Join(profileDir, "Network", "Cookies"),
		filepath.Join(profileDir, "Cookies"),
	)
	if !cookieTime.IsZero() {
		return cookiePath, cookieTime
	}
	if info, ok := fileInfo(filepath.Join(profileDir, "Preferences")); ok {
		return "", info.ModTime()
	}
	if info, ok := fileInfo(filepath.Join(root, "Local State")); ok {
		return "", info.ModTime()
	}
	if info, ok := fileInfo(profileDir); ok {
		return "", info.ModTime()
	}
	return "", time.Time{}
}

func profileCookieState(profileDir string, cookiePaths ...string) (string, time.Time) {
	for _, path := range cookiePaths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(profileDir, path)
		}
		if info, ok := fileInfo(path); ok && !info.IsDir() {
			return absoluteIfPossible(path), info.ModTime()
		}
	}
	return "", time.Time{}
}

func fileInfo(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	return info, true
}

func pathIsDir(path string) bool {
	info, ok := fileInfo(path)
	return ok && info.IsDir()
}

func linuxCookieBrowserSpecs(home string) []cookieBrowserSpec {
	configHome := configHomeDir(home)
	chromeConfigHome := chromeConfigHomeDir(home)
	chromeUserDataDir := chromeUserDataDir(home)

	return []cookieBrowserSpec{
		{Browser: "firefox", Family: FamilyFirefox, SupportsProfiles: true, Roots: uniquePaths([]string{
			filepath.Join(configHome, "mozilla", "firefox"),
			filepath.Join(home, ".mozilla", "firefox"),
			filepath.Join(home, ".librewolf"),
			filepath.Join(home, ".var", "app", "org.mozilla.firefox", "config", "mozilla", "firefox"),
			filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
			filepath.Join(home, ".var", "app", "io.gitlab.librewolf-community", ".librewolf"),
			filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
		})},
		{Browser: "chrome", Family: FamilyChromium, SupportsProfiles: true, Roots: uniquePaths([]string{
			chromeUserDataDir,
			filepath.Join(chromeConfigHome, "google-chrome"),
			filepath.Join(configHome, "google-chrome"),
			filepath.Join(configHome, "google-chrome-beta"),
			filepath.Join(configHome, "google-chrome-unstable"),
			filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome"),
		})},
		{Browser: "chromium", Family: FamilyChromium, SupportsProfiles: true, Roots: uniquePaths([]string{
			chromeUserDataDir,
			filepath.Join(chromeConfigHome, "chromium"),
			filepath.Join(configHome, "chromium"),
			filepath.Join(home, ".var", "app", "org.chromium.Chromium", "config", "chromium"),
			filepath.Join(home, "snap", "chromium", "common", "chromium"),
		})},
		{Browser: "brave", Family: FamilyChromium, SupportsProfiles: true, Roots: uniquePaths([]string{
			filepath.Join(configHome, "BraveSoftware"),
			filepath.Join(configHome, "BraveSoftware", "Brave-Browser"),
			filepath.Join(configHome, "BraveSoftware", "Brave-Origin"),
			filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser"),
			filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Origin"),
			filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Browser"),
			filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Origin"),
		})},
		{Browser: "vivaldi", Family: FamilyChromium, SupportsProfiles: true, Roots: []string{filepath.Join(configHome, "vivaldi")}},
		{Browser: "opera", Family: FamilyChromium, SupportsProfiles: false, Roots: uniquePaths([]string{
			filepath.Join(configHome, "opera"),
			filepath.Join(configHome, "opera-developer"),
			filepath.Join(configHome, "opera-beta"),
		})},
	}
}

func windowsCookieBrowserSpecs() []cookieBrowserSpec {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))

	return []cookieBrowserSpec{
		{Browser: "firefox", Family: FamilyFirefox, SupportsProfiles: true, Roots: uniquePaths([]string{
			pathUnder(appData, "Mozilla", "Firefox", "Profiles"),
			pathUnder(localAppData, "Packages", "Mozilla.Firefox_*", "LocalCache", "Roaming", "Mozilla", "Firefox", "Profiles"),
		})},
		{Browser: "chrome", Family: FamilyChromium, SupportsProfiles: true, Roots: uniquePaths([]string{
			pathUnder(localAppData, "Google", "Chrome", "User Data"),
			pathUnder(localAppData, "Google", "Chrome Beta", "User Data"),
			pathUnder(localAppData, "Google", "Chrome SxS", "User Data"),
		})},
		{Browser: "chromium", Family: FamilyChromium, SupportsProfiles: true, Roots: []string{pathUnder(localAppData, "Chromium", "User Data")}},
		{Browser: "brave", Family: FamilyChromium, SupportsProfiles: true, Roots: []string{pathUnder(localAppData, "BraveSoftware", "Brave-Browser", "User Data")}},
		{Browser: "vivaldi", Family: FamilyChromium, SupportsProfiles: true, Roots: []string{pathUnder(localAppData, "Vivaldi", "User Data")}},
		{Browser: "opera", Family: FamilyChromium, SupportsProfiles: false, Roots: uniquePaths([]string{
			pathUnder(appData, "Opera Software", "Opera Stable"),
			pathUnder(appData, "Opera Software", "Opera GX Stable"),
			pathUnder(appData, "Opera Software", "Opera Developer"),
		})},
	}
}

func pathUnder(base string, elem ...string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	parts := append([]string{base}, elem...)
	return filepath.Join(parts...)
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

func resolveRuntimeDeps(env *Env) CheckDepsResult {
	env.depsCacheMu.Lock()
	if time.Now().Before(env.runtimeDepsExpiry) {
		deps := env.runtimeDepsValue
		env.depsCacheMu.Unlock()
		return deps
	}
	if call := env.runtimeDepsFlight; call != nil {
		env.depsCacheMu.Unlock()
		<-call.done
		return call.result
	}
	call := &depsDetectCall{done: make(chan struct{})}
	env.runtimeDepsFlight = call
	env.depsCacheMu.Unlock()

	deps := detectDeps(env, false)

	env.depsCacheMu.Lock()
	env.runtimeDepsValue = deps
	env.runtimeDepsExpiry = time.Now().Add(runtimeDepsTTL)
	if env.runtimeDepsFlight == call {
		env.runtimeDepsFlight = nil
	}
	call.result = deps
	close(call.done)
	env.depsCacheMu.Unlock()
	return deps
}

func ytdlpCommandArgsFor(env *Env, deps CheckDepsResult, base []string) []string {
	args := make([]string, 0, len(base)+7)
	args = append(args, "--ignore-config")
	if deps.Cookies.Status == StatusActive {
		cookieArg := strings.TrimSpace(deps.Cookies.Browser)
		if profile := strings.TrimSpace(deps.Cookies.YTDLPProfile); profile != "" {
			cookieArg += ":" + profile
		}
		if cookieArg != "" {
			args = append(args, "--cookies-from-browser", cookieArg)
		}
	}
	if deps.Runtime.Status == StatusActive && strings.TrimSpace(deps.Runtime.Path) != "" {
		args = append(args, "--js-runtimes", "node:"+deps.Runtime.Path)
	}
	if ua := runtimeUserAgent(env, deps); ua != "" {
		args = append(args, "--user-agent", ua)
	}
	args = append(args, base...)
	return args
}

func runtimeUserAgent(env *Env, deps CheckDepsResult) string {
	if deps.Cookies.Status != StatusActive {
		return ""
	}
	if currentGOOS() != "linux" || !strings.EqualFold(strings.TrimSpace(deps.Cookies.Browser), "firefox") {
		return ""
	}
	return env.firefoxUserAgent()
}

func (env *Env) firefoxUserAgent() string {
	env.firefoxUserAgentOnce.Do(func() {
		env.firefoxUserAgentCache = buildFirefoxUserAgent()
	})
	return env.firefoxUserAgentCache
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
	platform, err := currentPlatform()
	if err != nil || platform.FirefoxUAPlatform == "" {
		return "X11; Linux x86_64"
	}
	return platform.FirefoxUAPlatform
}

func ytdlpOutput(env *Env, ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return ytdlpOutputFor(env, ctx, timeout, resolveRuntimeDeps(env), args...)
}

func startYTDLPMergedOutputCommand(env *Env, ctx context.Context, timeout time.Duration, args ...string) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	return startYTDLPMergedOutputCommandFor(env, ctx, timeout, resolveRuntimeDeps(env), args...)
}

func ytdlpOutputFor(env *Env, ctx context.Context, timeout time.Duration, deps CheckDepsResult, args ...string) ([]byte, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, fmt.Errorf("yt-dlp is required")
	}
	return commandOutput(ctx, timeout, bin, ytdlpCommandArgsFor(env, deps, args)...)
}

func startYTDLPMergedOutputCommandFor(env *Env, ctx context.Context, timeout time.Duration, deps CheckDepsResult, args ...string) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, nil, nil, nil, fmt.Errorf("yt-dlp is required")
	}
	return startMergedOutputCommand(ctx, timeout, bin, ytdlpCommandArgsFor(env, deps, args)...)
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
	_, ok := fileInfo(path)
	return ok
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
