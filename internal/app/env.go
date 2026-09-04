package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type runtimePaths struct {
	AppDir    string
	ConfigDir string
	DataDir   string
	DepsDir   string
}

type managedBinaries struct {
	YtdlpBin   string
	FFmpegBin  string
	FFprobeBin string
	NodeBin    string
}

type httpClients struct {
	apiClient *http.Client
	dlClient  *http.Client
}

type depCaches struct {
	depsCache        *flightCache[struct{}, CheckDepsResult]
	runtimeDepsCache *flightCache[struct{}, CheckDepsResult]
	probeCache       *flightCache[string, *MediaProbe]

	firefoxUserAgentOnce  sync.Once
	firefoxUserAgentCache string

	ffmpegEncodersMu    sync.Mutex
	ffmpegEncodersValue map[string]map[string]bool
}

type Env struct {
	IsWindows bool

	runtimePaths
	managedBinaries
	httpClients
	depCaches

	dlDirMu      sync.RWMutex
	downloadsDir string
}

func NewEnv() *Env {
	env := &Env{
		IsWindows:        runtime.GOOS == "windows",
		depsCache:        newFlightCache[struct{}, CheckDepsResult](),
		runtimeDepsCache: newFlightCache[struct{}, CheckDepsResult](),
		probeCache:       newFlightCache[string, *MediaProbe](),
	}

	exe := currentExecutablePath()
	env.initRuntimePaths(filepath.Dir(exe))
	env.initBinaryPaths()

	env.apiClient = newTimeoutHTTPClient(apiClientTimeout)
	env.dlClient = newDownloadHTTPClient()
	if env.IsWindows {
		enableConsoleVirtualTerminal()
	}
	return env
}

func currentExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		if exe, err = filepath.Abs(os.Args[0]); err != nil {
			exe = os.Args[0]
		}
	}
	return exe
}

func (env *Env) initBinaryPaths() {
	suffix := ""
	if env.IsWindows {
		suffix = ".exe"
	}
	env.YtdlpBin = filepath.Join(env.DepsDir, "yt-dlp"+suffix)
	env.FFmpegBin = filepath.Join(env.DepsDir, "ffmpeg"+suffix)
	env.FFprobeBin = filepath.Join(env.DepsDir, "ffprobe"+suffix)
	env.NodeBin = filepath.Join(env.DepsDir, "node"+suffix)
}

func (env *Env) DownloadsDir() string {
	env.dlDirMu.RLock()
	defer env.dlDirMu.RUnlock()
	return env.downloadsDir
}

func (env *Env) setDownloadsDir(path string) {
	env.dlDirMu.Lock()
	env.downloadsDir = path
	env.dlDirMu.Unlock()
}

func (env *Env) invalidateFFmpegEncoders() {
	if env == nil {
		return
	}
	env.ffmpegEncodersMu.Lock()
	defer env.ffmpegEncodersMu.Unlock()
	clear(env.ffmpegEncodersValue)
}
