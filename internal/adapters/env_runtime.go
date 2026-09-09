package adapters

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"volvid/internal/core"
)

var (
	Version = "7.4.2"

	ffmpegWinURL   = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ffmpegLinuxURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	nodeLatestURL  = "https://nodejs.org/download/release/latest/"
	ytdlpBase      = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/VolRencs/Volvid/releases/latest"
)

type runtimePlatform struct {
	UpdateAsset     string
	YTDLPAsset      string
	FFmpegURL       string
	NodeAssetSuffix string
}

func currentPlatform() (runtimePlatform, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return runtimePlatform{
			UpdateAsset:     "Volvid.exe",
			YTDLPAsset:      "yt-dlp.exe",
			FFmpegURL:       ffmpegWinURL,
			NodeAssetSuffix: "-win-x64.zip",
		}, nil
	case "linux/amd64":
		return runtimePlatform{
			UpdateAsset:     "Volvid",
			YTDLPAsset:      "yt-dlp_linux",
			FFmpegURL:       ffmpegLinuxURL,
			NodeAssetSuffix: "-linux-x64.tar.gz",
		}, nil
	default:
		return runtimePlatform{}, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func optimalParallelism(items, hardLimit int) int {
	if items <= 1 {
		return 1
	}
	limit := max(2, runtime.GOMAXPROCS(0))
	if hardLimit > 0 {
		limit = min(limit, hardLimit)
	}
	return min(items, limit)
}

const (
	defaultDialTimeout           = 30 * time.Second
	defaultKeepAlive             = 30 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = time.Second
	defaultResponseHeaderTimeout = 60 * time.Second
	defaultFileDownloadTimeout   = 2 * time.Hour
	defaultSafeRetryAttempts     = 3
	defaultSafeRetryBackoff      = 250 * time.Millisecond
	maxHTTPRedirects             = 10
	downloadCopyBufferSize       = 1 << 20
	retryBodyDrainLimit          = 1 << 20
	apiClientTimeout             = 8 * time.Second
	manifestFetchTimeout         = 30 * time.Second
	manifestMaxBytes             = 1 << 20
)

const (
	qualityScanTimeout      = 90 * time.Second
	searchTimeout           = 90 * time.Second
	playlistFetchTimeout    = 15 * time.Minute
	maxDetailedQualityURLs  = 5
	maxParallelQualityScans = 6
	// Runtime deps (cookies/js-runtime) change rarely mid-session and every
	// refresh stats browser profiles; installs invalidate the cache
	// explicitly, so a long TTL is safe.
	runtimeDepsTTL = 5 * time.Minute
	probeCacheTTL  = 10 * time.Minute
)

const (
	versionProbeTimeout  = 1500 * time.Millisecond
	tarCommandTimeout    = 2 * time.Minute
	maxExtractedFileSize = 512 << 20
	folderPickerTimeout  = 2 * time.Minute
)

const (
	processTerminateGrace    = 2 * time.Second
	commandStderrCaptureSize = 8 << 10
	commandStdoutMaxBytes    = 64 << 20
	commandLineBufferSize    = 128 << 10
	commandLineMaxBytes      = 16 << 20
	maxYtdlpErrorLine        = 512
	ffmpegEncodersTimeout    = 3 * time.Second
)

const (
	ytdlpDownloadRetries     = 10
	ytdlpFragmentRetries     = 10
	ytdlpConcurrentFragments = 12
)

const (
	progressEmitInterval = 100 * time.Millisecond
	slotResetDelay       = 300 * time.Millisecond
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
	depsCache        *flightCache[struct{}, core.CheckDepsResult]
	runtimeDepsCache *flightCache[struct{}, core.CheckDepsResult]
	probeCache       *flightCache[string, *core.MediaProbe]

	firefoxUserAgentOnce  sync.Once
	firefoxUserAgentCache string

	ffmpegEncodersMu     sync.Mutex
	ffmpegEncodersValue  map[string]map[string]bool
	ffmpegEncodersFlight map[string]*encoderFlight
}

// encoderFlight dedupes concurrent `ffmpeg -encoders` probes for one binary.
type encoderFlight struct {
	done     chan struct{}
	encoders map[string]bool
}

type Env struct {
	IsWindows bool

	runtimePaths
	managedBinaries
	httpClients
	depCaches

	dirs dirStore
}

// dirStore owns the mutable downloads-folder override behind a lock,
// so path state is not scattered across Env methods.
type dirStore struct {
	mu  sync.RWMutex
	dir string
}

func (s *dirStore) get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dir
}

func (s *dirStore) set(path string) {
	s.mu.Lock()
	s.dir = path
	s.mu.Unlock()
}

func NewEnv() *Env {
	env := &Env{
		IsWindows:        runtime.GOOS == "windows",
		depsCache:        newFlightCache[struct{}, core.CheckDepsResult](),
		runtimeDepsCache: newFlightCache[struct{}, core.CheckDepsResult](),
		probeCache:       newFlightCache[string, *core.MediaProbe](),
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
	if env == nil {
		return ""
	}
	return env.dirs.get()
}

func (env *Env) setDownloadsDir(path string) {
	if env == nil {
		return
	}
	env.dirs.set(path)
}

func (env *Env) invalidateFFmpegEncoders() {
	if env == nil {
		return
	}
	env.ffmpegEncodersMu.Lock()
	defer env.ffmpegEncodersMu.Unlock()
	clear(env.ffmpegEncodersValue)
	clear(env.ffmpegEncodersFlight)
}

const (
	appDirName      = "Volvid"
	envConfigDir    = "VOLVID_CONFIG_DIR"
	envDataDir      = "VOLVID_DATA_DIR"
	envDownloadsDir = "VOLVID_DOWNLOADS_DIR"
	envDepsDir      = "VOLVID_DEPS_DIR"
)

func (env *Env) initRuntimePaths(exeDir string) {
	env.AppDir = cleanAbsPath(exeDir)
	env.ConfigDir = resolveConfigDir(env)
	env.DataDir = resolveDataDir(env)
	env.DepsDir = resolveArtifactDir(envDepsDir, filepath.Join(env.DataDir, "deps"))
	env.dirs.set(resolveDownloadsDir(env))
}

func resolveConfigDir(env *Env) string {
	if path := envPath(envConfigDir); path != "" {
		return path
	}
	if root, ok := userConfigRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(env.AppDir, ".volvid", "config")
}

func resolveDataDir(env *Env) string {
	if path := envPath(envDataDir); path != "" {
		return path
	}
	if root, ok := userDataRoot(); ok {
		return filepath.Join(root, appDirName)
	}
	return filepath.Join(env.AppDir, ".volvid", "data")
}

func resolveArtifactDir(envKey, defaultPath string) string {
	if path := envPath(envKey); path != "" {
		return path
	}
	return cleanAbsPath(defaultPath)
}

func userConfigRoot() (string, bool) {
	root, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(root) == "" {
		return "", false
	}
	return root, true
}

func userDataRoot() (string, bool) {
	switch runtime.GOOS {
	case "windows":
		if root := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); root != "" {
			return cleanAbsPath(root), true
		}
	default:
		if root := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); root != "" {
			return cleanAbsPath(root), true
		}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, ".local", "share"), true
		}
	}
	if root, ok := userConfigRoot(); ok {
		return root, true
	}
	return "", false
}

func envPath(key string) string {
	return cleanAbsPath(os.Getenv(key))
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return filepath.Clean(abs)
}
