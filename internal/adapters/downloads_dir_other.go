//go:build !linux && !windows

package adapters

// systemDownloadsDirPlatform has no native implementation on this platform;
// systemDownloadsDir falls back to ~/Downloads.
func systemDownloadsDirPlatform() string {
	return ""
}
