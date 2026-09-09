package adapters

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"volvid/internal/core"
)

// resolveRuntimeDeps returns cached runtime deps (cookies/js-runtime/UA).
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

func buildDownloadCommandArgs(req core.DownloadRequest, deps core.CheckDepsResult, sourceURL, outputTemplate, format string, extra []string) ([]string, error) {
	args := make([]string, 0, 20+len(extra))
	args = append(args, core.FFmpegArgs(deps)...)

	modeArgs, err := downloadModeArgs(req.Profile, format)
	if err != nil {
		return nil, err
	}
	args = append(args, modeArgs...)
	args = append(args, downloadReliabilityArgs(req)...)
	args = append(args, "-o", outputTemplate, "--windows-filenames")
	args = appendFragmentDownloadArgs(args, req)
	args = append(args, extra...)
	args = append(args, sourceURL)
	return args, nil
}

func downloadReliabilityArgs(req core.DownloadRequest) []string {
	args := []string{
		"--continue",
		"--part",
		"--retries", strconv.Itoa(ytdlpDownloadRetries),
		"--fragment-retries", strconv.Itoa(ytdlpFragmentRetries),
		"--retry-sleep", "linear=1:5:2",
		"--abort-on-unavailable-fragments",
	}
	if req.Profile.Mode != core.ModeThumbnail {
		args = append(args, "--concurrent-fragments", strconv.Itoa(ytdlpConcurrentFragments))
	}
	return args
}

func downloadModeArgs(profile core.OutputProfile, format string) ([]string, error) {
	switch profile.Mode {
	case core.ModeThumbnail:
		return []string{"--skip-download", "--write-thumbnail"}, nil
	case core.ModeAudio:
		args := []string{"-f", "bestaudio/best", "--extract-audio"}
		if profile.AudioFormat != "" {
			args = append(args, "--audio-format", profile.AudioFormat)
		}
		if profile.AudioQuality != "" {
			args = append(args, "--audio-quality", profile.AudioQuality)
		}
		return args, nil
	case core.ModeVideo:
		return videoModeArgs(profile, format), nil
	default:
		return nil, fmt.Errorf("unsupported download mode %d", profile.Mode)
	}
}

func videoModeArgs(profile core.OutputProfile, format string) []string {
	container := strings.TrimSpace(profile.VideoContainer)
	if container == "" {
		container = "mp4"
	}

	if profile.RemuxOnly {
		return []string{"-f", format, "--remux-video", container}
	}

	return []string{"-f", format, "--merge-output-format", container}
}

func appendFragmentDownloadArgs(args []string, req core.DownloadRequest) []string {
	if req.Fragment == nil {
		return args
	}

	if section, ok := req.Fragment.SectionArg(); ok {
		args = append(args, "--download-sections", section)
		if req.Profile.Mode != core.ModeAudio {
			args = append(args, "--force-keyframes-at-cuts")
		}
	}
	return args
}
