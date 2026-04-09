//go:build !linux && !windows

package app

func systemDownloadsDirPlatform() string {
	return ""
}
