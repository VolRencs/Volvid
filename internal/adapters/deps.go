package adapters

import (
	"context"
	"errors"
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
		return detectDeps(env), nil
	})
	return result
}

func EnrichDeps(env *Env, ctx context.Context, deps core.CheckDepsResult) core.CheckDepsResult {
	entries := []struct {
		dep  *core.DependencyInfo
		spec depSpec
	}{
		{&deps.YTDLP, ytdlpDepSpec},
		{&deps.FFmpeg, ffmpegDepSpec},
		{&deps.Node, nodeDepSpec},
	}
	var wg sync.WaitGroup
	for _, entry := range entries {
		if !entry.dep.Available || entry.dep.Version != "" || strings.TrimSpace(entry.dep.Path) == "" {
			continue
		}
		wg.Go(func() {
			entry.dep.Version = parsedVersion(entry.spec, probeVersion(ctx, entry.dep.Path, entry.spec.VersionArgs...))
		})
	}
	wg.Wait()
	return deps
}

func parsedVersion(spec depSpec, raw string) string {
	if parse := spec.ParseVersion; parse != nil {
		return strings.TrimSpace(parse(raw))
	}
	return strings.TrimSpace(raw)
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

func detectDeps(env *Env) core.CheckDepsResult {
	var ytdlp, ffmpeg, node core.DependencyInfo
	var wg sync.WaitGroup
	wg.Go(func() {
		spec := ytdlpDepSpec
		spec.ManagedPath = env.YtdlpBin
		ytdlp = detectDependency(spec)
	})
	wg.Go(func() {
		spec := ffmpegDepSpec
		spec.ManagedPath = env.FFmpegBin
		ffmpeg = detectDependency(spec)
	})
	wg.Go(func() {
		spec := nodeDepSpec
		spec.ManagedPath = env.NodeBin
		node = detectDependency(spec)
	})
	wg.Wait()

	deps := core.CheckDepsResult{YTDLP: ytdlp, FFmpeg: ffmpeg, Node: node}
	home, _ := os.UserHomeDir()
	deps.Cookies = detectBrowserCookies(strings.TrimSpace(home), runtime.GOOS)
	deps.Runtime = detectJSRuntime(node)
	return deps
}

func detectDependency(spec depSpec) core.DependencyInfo {
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
	line, _ := commandVersionLine(context.Background(), bin, "--version")
	return firefoxVersionFromLine(line)
}
func firefoxVersionFromLine(line string) string {
	for field := range strings.FieldsSeq(strings.TrimSpace(line)) {
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
func commandVersionLine(ctx context.Context, bin string, args ...string) (string, error) {
	if strings.TrimSpace(bin) == "" {
		return "", errors.New("version probe path is empty")
	}
	out, err := commandCombinedOutput(ctx, versionProbeTimeout, bin, args...)
	return firstNonEmptyLine(string(out)), err
}

func probeVersion(ctx context.Context, bin string, args ...string) string {
	for range versionProbeAttempts {
		line, err := commandVersionLine(ctx, bin, args...)
		if line != "" {
			return line
		}
		if !errors.Is(err, context.DeadlineExceeded) || (ctx != nil && ctx.Err() != nil) {
			return ""
		}
	}
	return ""
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
