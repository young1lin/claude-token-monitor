package config

import (
	"os"
	"testing"
)

// TestMain unsets CLAUDE_CONFIG_DIR for every test in this package so that
// developer machines exporting the env var don't have it leak into tests that
// assume the home-derived fallback path. Individual tests asserting
// env-honoring behavior call t.Setenv to set it for their own scope.
func TestMain(m *testing.M) {
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	os.Exit(m.Run())
}
