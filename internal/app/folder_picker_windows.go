//go:build windows

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"
)

func pickDirectory(current, title string) (string, error) {
	script := strings.Join([]string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"[Console]::OutputEncoding = [System.Text.Encoding]::UTF8",
		"$dialog = New-Object System.Windows.Forms.FolderBrowserDialog",
		"$dialog.Description = " + quotePowerShellLiteral(title),
		"$dialog.ShowNewFolderButton = $true",
		"$dialog.SelectedPath = " + quotePowerShellLiteral(current),
		"if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {",
		"  [Console]::Write($dialog.SelectedPath)",
		"}",
	}, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), folderPickerTimeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-STA",
		"-EncodedCommand",
		encodePowerShell(script),
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if text := strings.TrimSpace(stderr.String()); text != "" {
			return "", fmt.Errorf("open folder picker: %s: %w", text, err)
		}
		return "", fmt.Errorf("open folder picker: %w", err)
	}

	path := cleanAbsPath(stdout.String())
	if path == "" {
		return "", ErrFolderPickerCancelled
	}
	return path, nil
}

func quotePowerShellLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		buf[i*2] = byte(value)
		buf[i*2+1] = byte(value >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
