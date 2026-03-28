package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type jsonFileState struct {
	name     string
	path     string
	modTime  time.Time
	fileSize int64
}

func newJSONFileState(name, path string) jsonFileState {
	return jsonFileState{name: name, path: path}
}

func (s *jsonFileState) changedLocked() (bool, error) {
	stat, err := os.Stat(s.path)
	switch {
	case err == nil:
		return !stat.ModTime().Equal(s.modTime) || stat.Size() != s.fileSize, nil
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("stat %s: %w", s.path, err)
	}
}

func (s *jsonFileState) refreshFileStateLocked() error {
	stat, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", s.path, err)
	}
	s.modTime = stat.ModTime()
	s.fileSize = stat.Size()
	return nil
}

func logStoreReloadError(name string, err error) {
	if err != nil {
		log.Printf("%s store reload: %v", name, err)
	}
}

func loadJSONFile(path string, zeroValue any, dest any) error {
	data, err := readOrInitJSONFile(path, zeroValue)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	if err := ensureJSONParentDir(path); err != nil {
		return err
	}
	data, err := marshalJSONFileData(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func readOrInitJSONFile(path string, zeroValue any) ([]byte, error) {
	if err := ensureJSONParentDir(path); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(data) != 0 {
			return data, nil
		}
	default:
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
	}

	data, err = marshalJSONFileData(zeroValue)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", path, err)
	}
	if err := writeJSONFile(path, zeroValue); err != nil {
		return nil, err
	}
	return data, nil
}

func ensureJSONParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func marshalJSONFileData(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func reloadJSONStateLocked[T any](
	file *jsonFileState,
	force bool,
	load func(path string) (T, error),
	assign func(data T),
) (bool, error) {
	changed := force
	if !force {
		var err error
		changed, err = file.changedLocked()
		if err != nil {
			return false, err
		}
		if !changed {
			return false, nil
		}
	}

	data, err := load(file.path)
	if err != nil {
		_ = file.refreshFileStateLocked()
		return changed, err
	}
	assign(data)
	if err := file.refreshFileStateLocked(); err != nil {
		return true, err
	}
	return true, nil
}

func flushJSONStateLocked(file *jsonFileState, payload any) error {
	if err := writeJSONFile(file.path, payload); err != nil {
		return err
	}
	return file.refreshFileStateLocked()
}
