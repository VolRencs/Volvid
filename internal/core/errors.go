package core

import "errors"

// ErrDownloadsDirLocked is returned when the download location is fixed
// via VOLVID_DOWNLOADS_DIR and the user tries to change it. Part of the
// UI contract: the TUI maps it to a localized message.
var ErrDownloadsDirLocked = errors.New("download location is fixed by VOLVID_DOWNLOADS_DIR")
