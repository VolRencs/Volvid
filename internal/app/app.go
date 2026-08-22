package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	Version = "7.0.0"

	ffmpegWinURL   = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ffmpegLinuxURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	nodeLatestURL  = "https://nodejs.org/download/release/latest/"
	ytdlpBase      = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/VolRencs/YouTubeDownloader/releases/latest"

	slotResetDelay = 300 * time.Millisecond
)

type runtimePlatform struct {
	UpdateAsset       string
	YTDLPAsset        string
	FFmpegURL         string
	NodeAssetSuffix   string
	FirefoxUAPlatform string
}

var (
	IsWindows = runtime.GOOS == "windows"

	AppDir    string
	ConfigDir string
	DataDir   string

	DepsDir string
	DlDir   string

	YtdlpBin   string
	FFmpegBin  string
	FFprobeBin string
	NodeBin    string

	apiClient *http.Client
	dlClient  *http.Client
)

func currentPlatform() (runtimePlatform, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return runtimePlatform{
			UpdateAsset:       "VolRenDownloader.exe",
			YTDLPAsset:        "yt-dlp.exe",
			FFmpegURL:         ffmpegWinURL,
			NodeAssetSuffix:   "-win-x64.zip",
			FirefoxUAPlatform: "",
		}, nil
	case "linux/amd64":
		return runtimePlatform{
			UpdateAsset:       "VolRenDownloader_linux_amd64",
			YTDLPAsset:        "yt-dlp_linux",
			FFmpegURL:         ffmpegLinuxURL,
			NodeAssetSuffix:   "-linux-x64.tar.gz",
			FirefoxUAPlatform: "X11; Linux x86_64",
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

func init() {
	exe, err := os.Executable()
	if err != nil {
		if exe, err = filepath.Abs(os.Args[0]); err != nil {
			exe = os.Args[0]
		}
	}
	base := filepath.Dir(exe)
	initRuntimePaths(base)
	if IsWindows {
		YtdlpBin = filepath.Join(DepsDir, "yt-dlp.exe")
		FFmpegBin = filepath.Join(DepsDir, "ffmpeg.exe")
		FFprobeBin = filepath.Join(DepsDir, "ffprobe.exe")
		NodeBin = filepath.Join(DepsDir, "node.exe")
	} else {
		YtdlpBin = filepath.Join(DepsDir, "yt-dlp")
		FFmpegBin = filepath.Join(DepsDir, "ffmpeg")
		FFprobeBin = filepath.Join(DepsDir, "ffprobe")
		NodeBin = filepath.Join(DepsDir, "node")
	}
	apiClient = NewHTTPClient(8 * time.Second)
	dlClient = newDownloadHTTPClient()
}
