package content

import (
	"os"
	"testing"
)

// TestMain does two things for every test in this package:
//
//  1. Unsets CLAUDE_CONFIG_DIR so developer machines exporting the env var
//     (multi-account setups, this repo's own contributors) don't leak it
//     into tests that assume the home-derived fallback path. Individual
//     tests that need to assert env-honoring behavior (e.g.
//     TestGetUserSkillsCount_HonorsClaudeConfigDir) call t.Setenv to restore
//     or replace the value for the scope of that test only.
//
//  2. Zeroes refreshCoordDelay. In production this is a 50ms settle window
//     to let a concurrent process win the refresh race; in a unit test
//     there's no second process, so the sleep is dead waiting. Six tests
//     hit this path and would otherwise add ~300ms of wall-clock time.
//     The single test that exercises the real coordination semantics
//     (TestShouldRefreshResult_RefreshMarkingWriteFail) restores 50ms via
//     t.Cleanup for its own duration.
func TestMain(m *testing.M) {
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	refreshCoordDelay = 0
	os.Exit(m.Run())
}
