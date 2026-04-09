//go:build !linux && !windows

package app

func pickDirectory(current, title string) (string, error) {
	return "", ErrFolderPickerUnsupported
}
