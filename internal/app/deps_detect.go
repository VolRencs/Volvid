package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	StatusActive    = "active"
	StatusNoProfile = "browser found but no usable profile"
	StatusNotFound  = "not found"

	FamilyFirefox  = "firefox"
	FamilyChromium = "chromium"
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

func DetectDeps(env *Env) CheckDepsResult  { return loadDeps(env, false) }
func RefreshDeps(env *Env) CheckDepsResult { return loadDeps(env, true) }

func loadDeps(env *Env, force bool) CheckDepsResult {
	if !force {
		if v, ok := env.depsCache.Get(struct{}{}); ok {
			return v
		}
	}
	result, _ := env.depsCache.Load(struct{}{}, func() (CheckDepsResult, error) {
		return detectDeps(env, true), nil
	})
	return result
}

func invalidateDepsCache(env *Env) {
	env.depsCache.InvalidateAll()
	env.runtimeDepsCache.InvalidateAll()
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
	var ytdlp, ffmpeg, node DependencyInfo
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		ytdlp = detectExecutableDependency(context.Background(), depSpec{
			Key: "ytdlp", Name: "yt-dlp", Required: true, Downloadable: true,
			LookNames: []string{"yt-dlp"}, ManagedPath: env.YtdlpBin,
			VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine,
		}, withVersions)
	}()
	go func() {
		defer wg.Done()
		ffmpeg = detectExecutableDependency(context.Background(), depSpec{
			Key: "ffmpeg", Name: "ffmpeg", Required: true, Downloadable: true,
			LookNames: []string{"ffmpeg"}, ManagedPath: env.FFmpegBin,
			VersionArgs: []string{"-version"}, ParseVersion: ffmpegVersionFromLine,
		}, withVersions)
	}()
	go func() {
		defer wg.Done()
		node = detectExecutableDependency(context.Background(), depSpec{
			Key: "node", Name: "node", Required: false, Downloadable: true,
			LookNames: []string{"node"}, ManagedPath: env.NodeBin,
			VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine,
		}, withVersions)
	}()
	wg.Wait()

	deps := CheckDepsResult{YTDLP: ytdlp, FFmpeg: ffmpeg, Node: node}
	home, _ := os.UserHomeDir()
	deps.Cookies = detectBrowserCookies(strings.TrimSpace(home), runtime.GOOS)
	deps.Runtime = detectJSRuntime(node)
	return deps
}

func detectExecutableDependency(ctx context.Context, spec depSpec, withVersion bool) DependencyInfo {
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
		parse := spec.ParseVersion
		if parse == nil {
			parse = firstNonEmptyLine
		}
		line := commandVersionLine(ctx, dep.Path, spec.VersionArgs...)
		dep.Version = strings.TrimSpace(parse(line))
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

func detectBrowserCookies(home, goos string) BrowserCookiesInfo {
	candidates, foundRoots := collectCookieCandidates(cookieBrowserSpecs(home, goos))
	if len(candidates) > 0 {
		return browserCookiesInfoFromCandidate(newestCookieCandidate(candidates), goos)
	}
	if foundRoots {
		return BrowserCookiesInfo{Status: StatusNoProfile}
	}
	return BrowserCookiesInfo{Status: StatusNotFound}
}

func browserCookiesInfoFromCandidate(candidate cookieCandidate, goos string) BrowserCookiesInfo {
	return BrowserCookiesInfo{
		Status:       StatusActive,
		Browser:      strings.TrimSpace(candidate.Browser),
		ProfileName:  cookieProfileName(candidate.Profile),
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
		return cookieCandidates(spec.Browser, roots, profileScanner{
			dirs: firefoxProfileDirs,
			state: func(_ string, profileDir string) (string, time.Time) {
				return firefoxProfileState(profileDir)
			},
		})
	case FamilyChromium:
		return cookieCandidates(spec.Browser, roots, profileScanner{
			dirs: func(root string) []string {
				return chromiumProfileDirs(root, spec.SupportsProfiles)
			},
			state: chromiumProfileState,
		})
	default:
		return nil
	}
}

func newestCookieCandidate(candidates []cookieCandidate) cookieCandidate {
	return slices.MaxFunc(candidates, func(a, b cookieCandidate) int {
		return a.ModTime.Compare(b.ModTime)
	})
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

type profileScanner struct {
	dirs  func(root string) []string
	state func(root, profileDir string) (cookiePath string, modTime time.Time)
}

func cookieCandidates(browser string, roots []string, scan profileScanner) []cookieCandidate {
	out := make([]cookieCandidate, 0, len(roots))
	for _, root := range roots {
		for _, profileDir := range scan.dirs(root) {
			cookiePath, modTime := scan.state(root, profileDir)
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

func collectProfileDirs(root string, exists func(string) bool, globs []string, normalize func(string) string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	if !pathIsDir(root) {
		return nil
	}
	out := make([]string, 0, 4)
	if exists(root) {
		out = append(out, root)
	}
	for _, pattern := range globs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if normalize != nil {
				match = normalize(match)
			}
			if exists(match) {
				out = append(out, match)
			}
		}
	}
	return uniquePaths(out)
}

func firefoxProfileDirs(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return collectProfileDirs(root, firefoxProfileExists, []string{
		filepath.Join(root, "*.default*"),
		filepath.Join(root, "*", "cookies.sqlite"),
		filepath.Join(root, "Profiles", "*.default*"),
		filepath.Join(root, "Profiles", "*", "cookies.sqlite"),
	}, func(match string) string {
		if filepath.Base(match) == "cookies.sqlite" {
			return filepath.Dir(match)
		}
		return match
	})
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
	return profileStateWithFallback(cookiePath, cookieTime,
		filepath.Join(profileDir, "prefs.js"),
		profileDir,
	)
}

func chromiumProfileDirs(root string, supportsProfiles bool) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	if !supportsProfiles {
		return collectProfileDirs(root, chromiumProfileExists, nil, nil)
	}
	return collectProfileDirs(root, chromiumProfileExists, []string{
		filepath.Join(root, "Default"),
		filepath.Join(root, "Guest Profile"),
		filepath.Join(root, "System Profile"),
		filepath.Join(root, "Profile *"),
	}, nil)
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
	return profileStateWithFallback(cookiePath, cookieTime,
		filepath.Join(profileDir, "Preferences"),
		filepath.Join(root, "Local State"),
		profileDir,
	)
}

func profileStateWithFallback(cookiePath string, cookieTime time.Time, fallbacks ...string) (string, time.Time) {
	if !cookieTime.IsZero() {
		return cookiePath, cookieTime
	}
	for _, path := range fallbacks {
		if info, ok := fileInfo(path); ok {
			return "", info.ModTime()
		}
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

type cookieBrowserDef struct {
	Browser          string
	Family           string
	SupportsProfiles bool
}

var cookieBrowserDefs = []cookieBrowserDef{
	{Browser: "firefox", Family: FamilyFirefox, SupportsProfiles: true},
	{Browser: "chrome", Family: FamilyChromium, SupportsProfiles: true},
	{Browser: "chromium", Family: FamilyChromium, SupportsProfiles: true},
	{Browser: "brave", Family: FamilyChromium, SupportsProfiles: true},
	{Browser: "vivaldi", Family: FamilyChromium, SupportsProfiles: true},
	{Browser: "opera", Family: FamilyChromium, SupportsProfiles: false},
}

func buildCookieSpecs(roots func(browser string) []string) []cookieBrowserSpec {
	specs := make([]cookieBrowserSpec, 0, len(cookieBrowserDefs))
	for _, def := range cookieBrowserDefs {
		specs = append(specs, cookieBrowserSpec{
			Browser:          def.Browser,
			Family:           def.Family,
			Roots:            uniquePaths(roots(def.Browser)),
			SupportsProfiles: def.SupportsProfiles,
		})
	}
	return specs
}

func linuxCookieBrowserSpecs(home string) []cookieBrowserSpec {
	configHome := configHomeDir(home)
	chromeConfigHome := chromeConfigHomeDir(home)
	chromeDataDir := chromeUserDataDir(home)

	return buildCookieSpecs(func(browser string) []string {
		switch browser {
		case "firefox":
			return []string{
				filepath.Join(configHome, "mozilla", "firefox"),
				filepath.Join(home, ".mozilla", "firefox"),
				filepath.Join(home, ".librewolf"),
				filepath.Join(home, ".var", "app", "org.mozilla.firefox", "config", "mozilla", "firefox"),
				filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
				filepath.Join(home, ".var", "app", "io.gitlab.librewolf-community", ".librewolf"),
				filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
			}
		case "chrome":
			return []string{
				chromeDataDir,
				filepath.Join(chromeConfigHome, "google-chrome"),
				filepath.Join(configHome, "google-chrome"),
				filepath.Join(configHome, "google-chrome-beta"),
				filepath.Join(configHome, "google-chrome-unstable"),
				filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome"),
			}
		case "chromium":
			return []string{
				chromeDataDir,
				filepath.Join(chromeConfigHome, "chromium"),
				filepath.Join(configHome, "chromium"),
				filepath.Join(home, ".var", "app", "org.chromium.Chromium", "config", "chromium"),
				filepath.Join(home, "snap", "chromium", "common", "chromium"),
			}
		case "brave":
			return []string{
				filepath.Join(configHome, "BraveSoftware"),
				filepath.Join(configHome, "BraveSoftware", "Brave-Browser"),
				filepath.Join(configHome, "BraveSoftware", "Brave-Origin"),
				filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser"),
				filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Origin"),
				filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Browser"),
				filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Origin"),
			}
		case "vivaldi":
			return []string{filepath.Join(configHome, "vivaldi")}
		case "opera":
			return []string{
				filepath.Join(configHome, "opera"),
				filepath.Join(configHome, "opera-developer"),
				filepath.Join(configHome, "opera-beta"),
			}
		default:
			return nil
		}
	})
}

func windowsCookieBrowserSpecs() []cookieBrowserSpec {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))

	return buildCookieSpecs(func(browser string) []string {
		switch browser {
		case "firefox":
			return []string{
				pathUnder(appData, "Mozilla", "Firefox", "Profiles"),
				pathUnder(localAppData, "Packages", "Mozilla.Firefox_*", "LocalCache", "Roaming", "Mozilla", "Firefox", "Profiles"),
			}
		case "chrome":
			return []string{
				pathUnder(localAppData, "Google", "Chrome", "User Data"),
				pathUnder(localAppData, "Google", "Chrome Beta", "User Data"),
				pathUnder(localAppData, "Google", "Chrome SxS", "User Data"),
			}
		case "chromium":
			return []string{pathUnder(localAppData, "Chromium", "User Data")}
		case "brave":
			return []string{pathUnder(localAppData, "BraveSoftware", "Brave-Browser", "User Data")}
		case "vivaldi":
			return []string{pathUnder(localAppData, "Vivaldi", "User Data")}
		case "opera":
			return []string{
				pathUnder(appData, "Opera Software", "Opera Stable"),
				pathUnder(appData, "Opera Software", "Opera GX Stable"),
				pathUnder(appData, "Opera Software", "Opera Developer"),
			}
		default:
			return nil
		}
	})
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
	if rest, ok := strings.CutPrefix(raw, "~"+string(os.PathSeparator)); ok {
		return absoluteIfPossible(filepath.Join(home, rest))
	}
	if filepath.IsAbs(raw) {
		return absoluteIfPossible(raw)
	}
	if home == "" {
		return absoluteIfPossible(raw)
	}
	return absoluteIfPossible(filepath.Join(home, raw))
}

func dedupeStrings(items []string, key func(string) string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		k := key(item)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func uniquePaths(paths []string) []string {
	return dedupeStrings(paths, filepath.Clean)
}

func resolveRuntimeDeps(env *Env) CheckDepsResult {
	result, _ := env.runtimeDepsCache.LoadWithTTL(struct{}{}, runtimeDepsTTL, func() (CheckDepsResult, error) {
		return detectDeps(env, false), nil
	})
	return result
}

func ytdlpBaseArgs(env *Env, deps CheckDepsResult) []string {
	args := make([]string, 0, 7)
	args = append(args, "--ignore-config", "--no-warnings")
	args = append(args, ffmpegArgs(deps)...)
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
	return args
}

func ytdlpCommandArgsFor(env *Env, deps CheckDepsResult, base []string) []string {
	return append(ytdlpBaseArgs(env, deps), base...)
}

func runtimeUserAgent(env *Env, deps CheckDepsResult) string {
	if deps.Cookies.Status != StatusActive {
		return ""
	}
	if runtime.GOOS != "linux" || !strings.EqualFold(strings.TrimSpace(deps.Cookies.Browser), "firefox") {
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
	return firefoxVersionFromLine(commandVersionLine(context.Background(), bin, "--version"))
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
	return "X11; Linux x86_64"
}

func ytdlpOutput(env *Env, ctx context.Context, timeout time.Duration, deps CheckDepsResult, args ...string) ([]byte, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, fmt.Errorf("yt-dlp is required")
	}
	return commandOutput(ctx, timeout, bin, ytdlpCommandArgsFor(env, deps, args)...)
}

func startYTDLPMergedOutputCommand(env *Env, ctx context.Context, timeout time.Duration, deps CheckDepsResult, args ...string) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, nil, nil, nil, fmt.Errorf("yt-dlp is required")
	}
	return startMergedOutputCommand(ctx, timeout, bin, ytdlpCommandArgsFor(env, deps, args)...)
}

func commandVersionLine(ctx context.Context, bin string, args ...string) string {
	if strings.TrimSpace(bin) == "" {
		return ""
	}
	out, _ := commandCombinedOutput(ctx, versionProbeTimeout, bin, args...)
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
