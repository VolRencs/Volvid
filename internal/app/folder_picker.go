package app

import (
	"context"
	"errors"
)

var (
	ErrFolderPickerCancelled   = errors.New("folder selection canceled")
	errFolderPickerUnsupported = errors.New("folder picker is not supported on this platform")
)

func PickDownloadsDir(ctx context.Context, env *Env, current string, locale Locale) (string, error) {
	current = cleanAbsPath(current)
	if current == "" {
		current = cleanAbsPath(env.DownloadsDir())
	}
	if current == "" {
		current = systemDownloadsDir(env)
	}
	return pickDirectory(ctx, current, StringsFor(locale).PickDownloadsTitle)
}

func IsFolderPickerCancelled(err error) bool {
	return errors.Is(err, ErrFolderPickerCancelled)
}
