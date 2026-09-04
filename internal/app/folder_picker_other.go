//go:build !linux && !windows

package app

import "context"

func pickDirectory(_ context.Context, current, title string) (string, error) {
	return "", errFolderPickerUnsupported
}
