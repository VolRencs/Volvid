package app

import (
	"fmt"
	"runtime"
	"time"
)

const (
	Version = "7.1.0"

	ffmpegWinURL   = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ffmpegLinuxURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	nodeLatestURL  = "https://nodejs.org/download/release/latest/"
	ytdlpBase      = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/VolRencs/YouTubeDownloader/releases/latest"

	apiClientTimeout = 8 * time.Second
	slotResetDelay   = 300 * time.Millisecond
)

type runtimePlatform struct {
	UpdateAsset       string
	YTDLPAsset        string
	FFmpegURL         string
	NodeAssetSuffix   string
	FirefoxUAPlatform string
}

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
