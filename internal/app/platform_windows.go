//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	windowsUpdateWaitTimeout   = 60
	windowsReplaceRetrySeconds = 20
)

func init() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
}

func applyUpdatePlatform(tmp, dest string) error {
	base := strings.TrimSuffix(dest, ".exe")
	_ = os.Remove(base + ".update.bat")

	script := base + ".update.ps1"
	content := windowsUpdateScript(tmp, dest, script)
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	if err := os.WriteFile(script, data, 0o644); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("создание update-скрипта: %w", err)
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", script,
	)
	cmd.SysProcAttr = detachedProcess()
	if err := cmd.Start(); err != nil {
		os.Remove(tmp)
		os.Remove(script)
		return fmt.Errorf("запуск update-скрипта: %w", err)
	}
	return nil
}

func windowsUpdateScript(tmp, dest, script string) string {
	return fmt.Sprintf(
		"$src = '%s'\r\n"+
			"$dst = '%s'\r\n"+
			"$self = '%s'\r\n"+
			"$pidToWait = %d\r\n"+
			"$waitSeconds = %d\r\n"+
			"$retrySeconds = %d\r\n"+
			"$updated = $false\r\n"+
			"try {\r\n"+
			"    Wait-Process -Id $pidToWait -Timeout $waitSeconds -ErrorAction Stop\r\n"+
			"} catch {\r\n"+
			"}\r\n"+
			"$deadline = (Get-Date).AddSeconds($retrySeconds)\r\n"+
			"while ((Get-Date) -lt $deadline) {\r\n"+
			"    try {\r\n"+
			"        [System.IO.File]::Copy($src, $dst, $true)\r\n"+
			"        Remove-Item -LiteralPath $src -Force -ErrorAction SilentlyContinue\r\n"+
			"        $updated = $true\r\n"+
			"        break\r\n"+
			"    } catch {\r\n"+
			"        Start-Sleep -Milliseconds 500\r\n"+
			"    }\r\n"+
			"}\r\n"+
			"if ($updated) {\r\n"+
			"    try {\r\n"+
			"        Start-Process -FilePath $dst | Out-Null\r\n"+
			"    } catch {\r\n"+
			"    }\r\n"+
			"} else {\r\n"+
			"    Remove-Item -LiteralPath $src -Force -ErrorAction SilentlyContinue\r\n"+
			"}\r\n"+
			"Start-Sleep -Milliseconds 200\r\n"+
			"Remove-Item -LiteralPath $self -Force -ErrorAction SilentlyContinue\r\n",
		psSingleQuoted(tmp),
		psSingleQuoted(dest),
		psSingleQuoted(script),
		os.Getpid(),
		windowsUpdateWaitTimeout,
		windowsReplaceRetrySeconds,
	)
}

func psSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
