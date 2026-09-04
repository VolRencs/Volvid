package app

import (
	"fmt"
	"runtime"
)

var (
	Version = "7.2.3"

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
