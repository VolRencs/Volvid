//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func init() {
	// Включаем ANSI escape codes на Windows 10+
	k := syscall.NewLazyDLL("kernel32.dll")
	h, _, _ := k.NewProc("GetStdHandle").Call(uintptr(^uint32(10) + 1))
	var mode uint32
	k.NewProc("GetConsoleMode").Call(h, uintptr(unsafe.Pointer(&mode)))
	k.NewProc("SetConsoleMode").Call(h, uintptr(mode|0x0004))
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
}
