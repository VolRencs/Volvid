package adapters

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"volvid/internal/core"
)

func DetectDeps(env *Env) core.CheckDepsResult  { return loadDeps(env, false) }
func RefreshDeps(env *Env) core.CheckDepsResult { return loadDeps(env, true) }
func loadDeps(env *Env, force bool) core.CheckDepsResult {
	if !force {
		if v, ok := env.depsCache.Get(struct{}{}); ok {
			return v
		}
	}
	result, _ := env.depsCache.Load(struct{}{}, 0, nil, func() (core.CheckDepsResult, error) {
		return detectDeps(env, true), nil
	})
	return result
}
func invalidateDepsCache(env *Env) {
	env.depsCache.InvalidateAll()
	env.runtimeDepsCache.InvalidateAll()
	env.ffmpegEncoders.InvalidateAll()
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

var (
	ytdlpDepSpec   = depSpec{Key: "ytdlp", Name: "yt-dlp", Required: true, Downloadable: true, LookNames: []string{"yt-dlp"}, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}
	ffmpegDepSpec  = depSpec{Key: "ffmpeg", Name: "ffmpeg", Required: true, Downloadable: true, LookNames: []string{"ffmpeg"}, VersionArgs: []string{"-version"}, ParseVersion: ffmpegVersionFromLine}
	ffprobeDepSpec = depSpec{Key: "ffprobe", Name: "ffprobe", VersionArgs: []string{"-version"}, ParseVersion: firstNonEmptyLine}
	nodeDepSpec    = depSpec{Key: "node", Name: "node", Required: false, Downloadable: true, LookNames: []string{"node"}, VersionArgs: []string{"--version"}, ParseVersion: firstNonEmptyLine}
)

func detectDeps(env *Env, withVersions bool) core.CheckDepsResult {
	var ytdlp, ffmpeg, node core.DependencyInfo
	var wg sync.WaitGroup
	wg.Go(func() {
		spec := ytdlpDepSpec
		spec.ManagedPath = env.YtdlpBin
		ytdlp = detectExecutableDependency(context.Background(), spec, withVersions)
	})
	wg.Go(func() {
		spec := ffmpegDepSpec
		spec.ManagedPath = env.FFmpegBin
		ffmpeg = detectExecutableDependency(context.Background(), spec, withVersions)
	})
	wg.Go(func() {
		spec := nodeDepSpec
		spec.ManagedPath = env.NodeBin
		node = detectExecutableDependency(context.Background(), spec, withVersions)
	})
	wg.Wait()

	deps := core.CheckDepsResult{YTDLP: ytdlp, FFmpeg: ffmpeg, Node: node}
	home, _ := os.UserHomeDir()
	deps.Cookies = detectBrowserCookies(strings.TrimSpace(home), runtime.GOOS)
	deps.Runtime = detectJSRuntime(node)
	return deps
}
func detectExecutableDependency(ctx context.Context, spec depSpec, withVersion bool) core.DependencyInfo {
	dep := core.DependencyInfo{Key: spec.Key, Name: spec.Name, Required: spec.Required, Downloadable: spec.Downloadable, Source: core.DepMissing}

	if path, ok := firstLookPath(spec.LookNames...); ok {
		dep.Path = cleanAbsPath(path)
		dep.Source = core.DepSystem
		dep.Available = true
	} else if pathExists(spec.ManagedPath) {
		dep.Path = spec.ManagedPath
		dep.Source = core.DepManaged
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

var cookieBrowserDefs = []cookieBrowserDef{
	{Browser: "firefox", Family: core.FamilyFirefox, SupportsProfiles: true},
	{Browser: "chrome", Family: core.FamilyChromium, SupportsProfiles: true},
	{Browser: "chromium", Family: core.FamilyChromium, SupportsProfiles: true},
	{Browser: "brave", Family: core.FamilyChromium, SupportsProfiles: true},
	{Browser: "vivaldi", Family: core.FamilyChromium, SupportsProfiles: true},
	{Browser: "opera", Family: core.FamilyChromium, SupportsProfiles: false},
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
func ytdlpOutput(ctx context.Context, timeout time.Duration, deps core.CheckDepsResult, args ...string) ([]byte, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, fmt.Errorf("yt-dlp is required")
	}
	return commandOutput(ctx, timeout, bin, ytdlpCommandArgsFor(deps, args)...)
}
func startYTDLPMergedOutputCommand(ctx context.Context, timeout time.Duration, deps core.CheckDepsResult, args ...string) (*exec.Cmd, io.ReadCloser, context.Context, context.CancelFunc, error) {
	bin := strings.TrimSpace(deps.YTDLP.Path)
	if bin == "" {
		return nil, nil, nil, nil, fmt.Errorf("yt-dlp is required")
	}
	return startMergedOutputCommand(ctx, timeout, bin, ytdlpCommandArgsFor(deps, args)...)
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
	for line := range strings.SplitSeq(text, "\n") {
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
