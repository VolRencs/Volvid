//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	downloadsShell32                = syscall.NewLazyDLL("shell32.dll")
	downloadsOle32                  = syscall.NewLazyDLL("ole32.dll")
	shell32ProcSHGetKnownFolderPath = downloadsShell32.NewProc("SHGetKnownFolderPath")
	ole32ProcCoTaskMemFree          = downloadsOle32.NewProc("CoTaskMemFree")
	folderIDDownloads               = windows.GUID{
		Data1: 0x374DE290,
		Data2: 0x123F,
		Data3: 0x4565,
		Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B},
	}
)

func systemDownloadsDirPlatform() string {
	var rawPath *uint16
	r1, _, _ := shell32ProcSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(&folderIDDownloads)),
		0,
		0,
		uintptr(unsafe.Pointer(&rawPath)),
	)
	if r1 != 0 || rawPath == nil {
		return fallbackWindowsDownloadsDir()
	}
	defer ole32ProcCoTaskMemFree.Call(uintptr(unsafe.Pointer(rawPath)))

	path := strings.TrimSpace(windows.UTF16PtrToString(rawPath))
	if path == "" {
		return fallbackWindowsDownloadsDir()
	}
	return cleanAbsPath(path)
}

func fallbackWindowsDownloadsDir() string {
	profile := cleanAbsPath(os.Getenv("USERPROFILE"))
	if profile == "" {
		return ""
	}
	return filepath.Join(profile, "Downloads")
}
