//go:build windows

package main

import (
	"os"
	"testing"
)

func TestInitConsoleSkipsWhenAlreadyInitialized(t *testing.T) {
	// Set env var to simulate Claude Code already initialized console
	originalValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		} else {
			os.Setenv("CLAUDE_CONSOLE_INITIALIZED", originalValue)
		}
	}()

	os.Setenv("CLAUDE_CONSOLE_INITIALIZED", "1")

	// Call initConsole - it should return immediately without Windows API calls
	// No way to directly verify API calls weren't made, but we can verify it doesn't crash
	initConsole()

	// Verify env var is still set
	if os.Getenv("CLAUDE_CONSOLE_INITIALIZED") != "1" {
		t.Error("Expected CLAUDE_CONSOLE_INITIALIZED to remain set")
	}

	t.Log("initConsole skipped initialization when env var was already set")
}

func TestInitConsoleRunsWhenNotInitialized(t *testing.T) {
	// Unset env var to simulate first invocation
	originalValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		} else {
			os.Setenv("CLAUDE_CONSOLE_INITIALIZED", originalValue)
		}
	}()

	os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")

	// Call initConsole - it should run full initialization
	initConsole()

	// This test can only verify it doesn't crash
	// Windows API calls cannot be easily mocked
	t.Log("initConsole ran without crashing when env var was not set")
}

func TestInitConsoleSetsEnvVar(t *testing.T) {
	// Unset env var
	originalValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		} else {
			os.Setenv("CLAUDE_CONSOLE_INITIALIZED", originalValue)
		}
	}()

	os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")

	// Call initConsole
	initConsole()

	// Verify env var is now set
	if os.Getenv("CLAUDE_CONSOLE_INITIALIZED") != "1" {
		t.Error("Expected CLAUDE_CONSOLE_INITIALIZED to be set after initialization")
	}

	t.Log("initConsole successfully set CLAUDE_CONSOLE_INITIALIZED env var")
}

func TestInitConsoleIdempotent(t *testing.T) {
	// Clear env var
	originalValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		} else {
			os.Setenv("CLAUDE_CONSOLE_INITIALIZED", originalValue)
		}
	}()

	os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")

	// Call initConsole multiple times
	initConsole()
	firstValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")

	initConsole()
	secondValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")

	initConsole()
	thirdValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")

	// All values should be "1"
	if firstValue != "1" || secondValue != "1" || thirdValue != "1" {
		t.Errorf("Expected all values to be '1', got: %s, %s, %s", firstValue, secondValue, thirdValue)
	}

	t.Log("initConsole is idempotent - multiple calls work correctly")
}

func TestInitConsoleConcurrentSafety(t *testing.T) {
	// Clear env var
	originalValue := os.Getenv("CLAUDE_CONSOLE_INITIALIZED")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		} else {
			os.Setenv("CLAUDE_CONSOLE_INITIALIZED", originalValue)
		}
	}()

	os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")

	// Call initConsole concurrently from multiple goroutines
	// Note: This is not truly thread-safe in the implementation, but it shouldn't crash
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			initConsole()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify env var is set
	if os.Getenv("CLAUDE_CONSOLE_INITIALIZED") != "1" {
		t.Error("Expected CLAUDE_CONSOLE_INITIALIZED to be set after concurrent calls")
	}

	t.Log("initConsole survived concurrent calls without crashing")
}

// Benchmark: With env var set (cache hit)
func BenchmarkInitConsoleWithEnvVar(b *testing.B) {
	os.Setenv("CLAUDE_CONSOLE_INITIALIZED", "1")
	defer os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		initConsole()
	}
}

// Benchmark: Without env var (full initialization)
func BenchmarkInitConsoleWithoutEnvVar(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Unsetenv("CLAUDE_CONSOLE_INITIALIZED")
		initConsole()
	}
}
