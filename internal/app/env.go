package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Env struct {
	IsWindows bool

	AppDir    string
	ConfigDir string
	DataDir   string
	DepsDir   string

	YtdlpBin   string
	FFmpegBin  string
	FFprobeBin string
	NodeBin    string

	apiClient *http.Client
	dlClient  *http.Client

	dlDirMu      sync.RWMutex
	downloadsDir string

	firefoxUserAgentOnce  sync.Once
	firefoxUserAgentCache string

	depsCache        *FlightCache[struct{}, CheckDepsResult]
	runtimeDepsCache *FlightCache[struct{}, CheckDepsResult]

	probeCache *FlightCache[string, *MediaProbe]

	ffmpegEncodersMu    sync.Mutex
	ffmpegEncodersValue map[string]map[string]bool
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

	env.apiClient = NewHTTPClient(apiClientTimeout)
	env.dlClient = newDownloadHTTPClient()
	return env
}

func (env *Env) ensureCaches() {
	if env.depsCache == nil {
		env.depsCache = newFlightCache[struct{}, CheckDepsResult]()
	}
	if env.runtimeDepsCache == nil {
		env.runtimeDepsCache = newFlightCache[struct{}, CheckDepsResult]()
	}
	if env.probeCache == nil {
		env.probeCache = newFlightCache[string, *MediaProbe]()
	}
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
