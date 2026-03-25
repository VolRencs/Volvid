package app

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	Version = "6.1.3"

	ffmpegWinURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ytdlpBase    = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/VolRencs/YouTubeDownloader/releases/latest"

	slotResetDelay = 300 * time.Millisecond
)

var (
	IsWindows = runtime.GOOS == "windows"
	Arch      = runtime.GOARCH

	AppDir string

	DepsDir        string
	DlDir          string
	YtdlpBin       string
	FFmpegResolved string

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
	} else {
		YtdlpBin = filepath.Join(DepsDir, "yt-dlp")
	}
	apiClient = NewHTTPClient(8 * time.Second)
	dlClient = newDownloadHTTPClient()
}
