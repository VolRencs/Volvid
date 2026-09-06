package adapters

import (
	"runtime"
	"strings"
	"volvid/internal/core"
)

func resolveRuntimeDeps(env *Env) core.CheckDepsResult {
	result, _ := env.runtimeDepsCache.LoadWithTTL(struct{}{}, runtimeDepsTTL, func() (core.CheckDepsResult, error) {
		return detectDeps(env, false), nil
	})
	return result
}
func ytdlpBaseArgs(env *Env, deps core.CheckDepsResult) []string {
	args := make([]string, 0, 7)
	args = append(args, "--ignore-config", "--no-warnings")
	args = append(args, core.FFmpegArgs(deps)...)
	if deps.Cookies.Status == core.StatusActive {
		cookieArg := strings.TrimSpace(deps.Cookies.Browser)
		if profile := strings.TrimSpace(deps.Cookies.YTDLPProfile); profile != "" {
			cookieArg += ":" + profile
		}
		if cookieArg != "" {
			args = append(args, "--cookies-from-browser", cookieArg)
		}
	}
	if deps.Runtime.Status == core.StatusActive && strings.TrimSpace(deps.Runtime.Path) != "" {
		args = append(args, "--js-runtimes", "node:"+deps.Runtime.Path)
	}
	if ua := runtimeUserAgent(env, deps); ua != "" {
		args = append(args, "--user-agent", ua)
	}
	return args
}
func ytdlpCommandArgsFor(env *Env, deps core.CheckDepsResult, base []string) []string {
	return append(ytdlpBaseArgs(env, deps), base...)
}
func runtimeUserAgent(env *Env, deps core.CheckDepsResult) string {
	if deps.Cookies.Status != core.StatusActive {
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
