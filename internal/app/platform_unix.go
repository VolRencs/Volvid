//go:build !windows

package app

import (
	"fmt"
	"os"
)

func applyUpdatePlatform(tmp, dest string) error {
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("chmod new binary: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func enableConsoleVirtualTerminal() {}
