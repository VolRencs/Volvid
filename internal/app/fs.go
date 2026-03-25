package app

import (
	"io/fs"
	"path/filepath"
)

func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}
