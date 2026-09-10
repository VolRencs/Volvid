//go:build !linux && !windows

package adapters

import (
	"context"
	"fmt"
	"runtime"
)

func pickDirectory(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("folder picker is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
}
