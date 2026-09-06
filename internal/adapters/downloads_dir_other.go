//go:build !linux && !windows

package adapters

func systemDownloadsDirPlatform() string {
	return ""
}
