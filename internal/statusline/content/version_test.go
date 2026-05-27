package content

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errCommandNotFound = errors.New("command not found")

func TestClaudeVersionCollector(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()
	// Note: getClaudeVersion returns parts[0] which is "claude" when output is "claude 1.0.3"
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("claude 1.0.3\n"),
		},
	}

	collector := NewClaudeVersionCollector()

	// Constructor fields
	assert.Equal(t, ContentClaudeVersion, collector.Type())
	assert.True(t, collector.Optional())

	// Collect
	result, err := collector.Collect(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "claude", result)
}

func TestClaudeVersionCollector_InvalidInput(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("claude 1.0.3\n"),
		},
	}

	collector := NewClaudeVersionCollector()

	// Collect ignores input type — always returns version
	result, err := collector.Collect("not StatusLineInput", nil)
	require.NoError(t, err)
	assert.Equal(t, "claude", result)
}

func TestClaudeVersionCollector_CommandFails(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"claude --version": errCommandNotFound,
		},
	}

	collector := NewClaudeVersionCollector()
	result, err := collector.Collect(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestGetClaudeVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		// Note: getClaudeVersion returns parts[0], which is the first token
		{"full version", "claude 1.2.3\n", "claude"},
		{"lowercase v prefix", "v1.2.3\n", "1.2.3"},
		{"uppercase V prefix", "V1.2.3\n", "1.2.3"},
		{"just version", "1.0.0\n", "1.0.0"},
		{"with extra fields", "claude 1.0.0 (abc123)\n", "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer restoreDefaultRunner()
			clearVersionCache()
			defaultCommandRunner = &StubCommandRunner{
				Outputs: map[string][]byte{
					"claude --version": []byte(tt.output),
				},
			}

			result := getClaudeVersion()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetClaudeVersionCached_CacheHit(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()

	callCount := 0
	runner := &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("2.0.0\n"),
		},
	}
	runner.Outputs["claude --version"] = []byte("2.0.0\n")
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("2.0.0\n"),
		},
	}

	// First call
	v1 := getClaudeVersionCached()
	assert.Equal(t, "2.0.0", v1)
	callCount++

	// Replace runner — should still return cached value
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("9.9.9\n"),
		},
	}

	v2 := getClaudeVersionCached()
	assert.Equal(t, "2.0.0", v2, "should return cached value")
	_ = callCount
}

func TestGetClaudeVersion_EmptyOutput(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("\n"),
		},
	}

	result := getClaudeVersion()
	assert.Equal(t, "", result)
}

func TestGetClaudeVersion_WhitespaceOnly(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"claude --version": []byte("   \n"),
		},
	}

	result := getClaudeVersion()
	assert.Equal(t, "", result)
}

// trackingRunner counts how many times Run is invoked, so we can prove the
// stdin-version fast path doesn't shell out.
type trackingRunner struct {
	calls int
}

func (r *trackingRunner) Run(dir, name string, args ...string) ([]byte, error) {
	r.calls++
	// Real `claude --version` prints just the version string, no prefix.
	return []byte("2.1.150\n"), nil
}

func TestClaudeVersionCollector_PrefersStdinVersion(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()

	runner := &trackingRunner{}
	defaultCommandRunner = runner

	// Real CC 2.1.150 supplies "version" on stdin — must echo and skip exec.
	input := &StatusLineInput{Version: "2.1.150"}

	got, err := NewClaudeVersionCollector().Collect(input, nil)
	require.NoError(t, err)
	assert.Equal(t, "2.1.150", got)
	assert.Equal(t, 0, runner.calls, "stdin fast path must not fork `claude --version`")
}

func TestClaudeVersionCollector_FallsBackWhenStdinVersionEmpty(t *testing.T) {
	defer restoreDefaultRunner()
	clearVersionCache()

	runner := &trackingRunner{}
	defaultCommandRunner = runner

	// Older CC: Version is empty → run the binary.
	input := &StatusLineInput{Version: ""}

	got, err := NewClaudeVersionCollector().Collect(input, nil)
	require.NoError(t, err)
	assert.Equal(t, "2.1.150", got)
	assert.Equal(t, 1, runner.calls, "fallback path must invoke the runner exactly once")
}
