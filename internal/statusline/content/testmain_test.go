package content

import (
	"os"
	"testing"
)

// TestMain unsets CLAUDE_CONFIG_DIR for every test in this package so that
// developer machines exporting the env var (multi-account setups, this repo's
// own contributors) don't have it leak into tests that assume the home-derived
// fallback path. Individual tests that need to assert env-honoring behavior
// (e.g. TestGetUserSkillsCount_HonorsClaudeConfigDir) call t.Setenv to restore
// or replace the value for the scope of that test only.
func TestMain(m *testing.M) {
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	os.Exit(m.Run())
}
