package app

import (
	"errors"
	"time"
)

const folderPickerTimeout = 2 * time.Minute

var (
	ErrFolderPickerCancelled   = errors.New("folder selection canceled")
	ErrFolderPickerUnsupported = errors.New("folder picker is not supported on this platform")
)

func PickDownloadsDir(env *Env, current string, locale Locale) (string, error) {
	current = cleanAbsPath(current)
	if current == "" {
		current = cleanAbsPath(env.DownloadsDir())
	}
	if current == "" {
		current = systemDownloadsDir(env)
	}
	return pickDirectory(current, StringsFor(locale).PickDownloadsTitle)
}

func IsFolderPickerCancelled(err error) bool {
	return errors.Is(err, ErrFolderPickerCancelled)
}
