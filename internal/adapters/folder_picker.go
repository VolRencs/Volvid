package adapters

import (
	"context"
	"errors"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

var ErrFolderPickerCancelled = errors.New("folder selection canceled")

func PickDownloadsDir(ctx context.Context, env *Env, current string, locale core.Locale) (string, error) {
	current = cleanAbsPath(current)
	if current == "" {
		current = cleanAbsPath(env.DownloadsDir())
	}
	if current == "" {
		current = systemDownloadsDir(env)
	}
	return pickDirectory(ctx, current, i18n.StringsFor(locale).PickDownloadsTitle)
}

func IsFolderPickerCancelled(err error) bool {
	return errors.Is(err, ErrFolderPickerCancelled)
}
