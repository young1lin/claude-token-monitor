//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Windows API functions for console control
var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutputCP       = modkernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP             = modkernel32.NewProc("SetConsoleCP")
	procGetConsoleMode           = modkernel32.NewProc("GetConsoleMode")
	procSetConsoleMode           = modkernel32.NewProc("SetConsoleMode")
	procGetStdHandle             = modkernel32.NewProc("GetStdHandle")
)

const (
	STD_OUTPUT_HANDLE                  = uintptr(-11 & 0xFFFFFFFF)
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	CP_UTF8                            = 65001
)

// initConsole initializes Windows console for UTF-8 and virtual terminal processing
func initConsole() {
	// Set console code page to UTF-8 (65001)
	procSetConsoleOutputCP.Call(CP_UTF8)
	procSetConsoleCP.Call(CP_UTF8)

	// Enable virtual terminal processing for ANSI escape sequences
	stdoutHandle, _, _ := procGetStdHandle.Call(STD_OUTPUT_HANDLE)
	if stdoutHandle != 0 {
		var mode uint32
		procGetConsoleMode.Call(stdoutHandle, uintptr(unsafe.Pointer(&mode)))
		procSetConsoleMode.Call(stdoutHandle, uintptr(mode|ENABLE_VIRTUAL_TERMINAL_PROCESSING))
	}
}
