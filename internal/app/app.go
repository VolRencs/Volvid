package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	Version = "6.1.8"

	ffmpegWinURL        = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ffmpegWinARM64URL   = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-winarm64-gpl.zip"
	ffmpegLinuxAMD64URL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	ffmpegLinuxARM64URL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz"
	nodeLatestV22URL    = "https://nodejs.org/download/release/latest-v22.x/"
	ytdlpBase           = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/VolRencs/YouTubeDownloader/releases/latest"

	slotResetDelay = 300 * time.Millisecond
)

var (
	IsWindows = runtime.GOOS == "windows"
	Arch      = runtime.GOARCH

	AppDir string

	DepsDir string
	DlDir   string

	YtdlpBin       string
	YtdlpResolved  string
	FFmpegBin      string
	FFprobeBin     string
	FFmpegResolved string
	NodeBin        string
	NodeResolved   string

	apiClient = &http.Client{Timeout: 8 * time.Second}
	dlClient  *http.Client
)

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
	AppDir = base
	DepsDir = filepath.Join(base, "_deps")
	DlDir = filepath.Join(base, "downloads")
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
