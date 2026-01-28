//go:build !windows

package main

// initConsole is a no-op on non-Windows platforms
func initConsole() {
	// ANSI escape sequences are natively supported on Unix systems
}
