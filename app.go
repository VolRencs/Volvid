package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	Version    = "5.0.0"
	GithubRepo = "VolRencs/YouTubeDownloader"

	ffmpegWinURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	ytdlpBase    = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

	githubAPIURL = "https://api.github.com/repos/" + GithubRepo + "/releases/latest"

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
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 60 * time.Second
	t.DialContext = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	dlClient = &http.Client{Transport: t}
}
