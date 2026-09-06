//go:build !linux && !windows

package adapters

import "context"

func pickDirectory(_ context.Context, current, title string) (string, error) {
	return "", errFolderPickerUnsupported
}
