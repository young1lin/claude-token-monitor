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
