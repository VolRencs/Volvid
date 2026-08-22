package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
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

	depsCacheMu         sync.Mutex
	depsCache           CheckDepsResult
	depsCacheReady      bool
	depsCacheFlight     *depsDetectCall
	depsCacheGeneration uint64

	runtimeDepsExpiry time.Time
	runtimeDepsValue  CheckDepsResult
	runtimeDepsFlight *depsDetectCall

	probeCacheMu sync.RWMutex
	probeCache   map[string]*MediaProbe
	probeFlight  map[string]*probeCall

	ffmpegEncodersMu    sync.Mutex
	ffmpegEncodersValue map[string]map[string]bool
}

func NewEnv() *Env {
	env := &Env{
		IsWindows:   runtime.GOOS == "windows",
		probeCache:  make(map[string]*MediaProbe),
		probeFlight: make(map[string]*probeCall),
	}

	exe := currentExecutablePath()
	env.initRuntimePaths(filepath.Dir(exe))
	env.initBinaryPaths()

	env.apiClient = NewHTTPClient(apiClientTimeout)
	env.dlClient = newDownloadHTTPClient()
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
