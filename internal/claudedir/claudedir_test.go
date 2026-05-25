package claudedir

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticHome returns a HomeProvider that always yields (path, nil).
func staticHome(path string) HomeProvider {
	return func() (string, error) { return path, nil }
}

// erroringHome returns a HomeProvider that always yields ("", err).
func erroringHome(err error) HomeProvider {
	return func() (string, error) { return "", err }
}

func TestResolve_EnvWinsOverHome(t *testing.T) {
	t.Setenv(EnvVar, filepath.FromSlash("/tmp/account-ME"))

	got, err := Resolve(staticHome(filepath.FromSlash("/tmp/should-not-be-used")))
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("/tmp/account-ME"), got,
		"CLAUDE_CONFIG_DIR must override the home-derived path")
}

func TestResolve_EnvTrimmed(t *testing.T) {
	t.Setenv(EnvVar, "   "+filepath.FromSlash("/tmp/account-ME")+"  ")

	got, err := Resolve(staticHome(filepath.FromSlash("/tmp/home")))
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("/tmp/account-ME"), got,
		"leading/trailing whitespace must be stripped from the env value")
}

func TestResolve_WhitespaceOnlyEnvIgnored(t *testing.T) {
	t.Setenv(EnvVar, "   \t\n  ")

	got, err := Resolve(staticHome(filepath.FromSlash("/tmp/home")))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.FromSlash("/tmp/home"), ".claude"), got,
		"whitespace-only env must be treated as unset")
}

func TestResolve_EmptyEnvFallsBackToHomeClaude(t *testing.T) {
	t.Setenv(EnvVar, "")

	got, err := Resolve(staticHome(filepath.FromSlash("/tmp/home")))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.FromSlash("/tmp/home"), ".claude"), got)
}

func TestResolve_HomeErrorPropagatesWhenEnvUnset(t *testing.T) {
	t.Setenv(EnvVar, "")
	sentinel := errors.New("home lookup failed")

	_, err := Resolve(erroringHome(sentinel))
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel,
		"home() errors must surface unmodified when env is unset")
}

func TestResolve_HomeErrorIgnoredWhenEnvSet(t *testing.T) {
	t.Setenv(EnvVar, filepath.FromSlash("/tmp/account-ME"))

	got, err := Resolve(erroringHome(errors.New("would-be-fatal")))
	require.NoError(t, err, "env should short-circuit before home() is called")
	assert.Equal(t, filepath.FromSlash("/tmp/account-ME"), got)
}

func TestResolve_NilProviderWithEnvSet(t *testing.T) {
	t.Setenv(EnvVar, filepath.FromSlash("/tmp/account-ME"))

	got, err := Resolve(nil)
	require.NoError(t, err, "nil provider is fine when env supplies the answer")
	assert.Equal(t, filepath.FromSlash("/tmp/account-ME"), got)
}

func TestResolve_NilProviderWithEnvUnset(t *testing.T) {
	t.Setenv(EnvVar, "")

	_, err := Resolve(nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoConfigDir)
}

func TestResolve_EmptyHomeWithEnvUnset(t *testing.T) {
	t.Setenv(EnvVar, "")

	_, err := Resolve(staticHome(""))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoConfigDir,
		"empty home is structurally the same as 'no home'")
}

func TestResolve_HomeProviderInvokedLazily(t *testing.T) {
	// When env is set, the home provider must not be called — saves a syscall
	// on every statusline refresh in multi-account setups.
	t.Setenv(EnvVar, filepath.FromSlash("/tmp/account-ME"))
	called := false
	home := func() (string, error) {
		called = true
		return "/never-reached", nil
	}

	_, err := Resolve(home)
	require.NoError(t, err)
	assert.False(t, called, "home provider must not be invoked when env supplies the answer")
}

func TestResolve_EnvValuePreservedVerbatim(t *testing.T) {
	// Resolve must not normalize, clean, or rewrite the env value beyond the
	// surrounding-whitespace trim. Users may point at UNC paths, relative
	// paths, or paths with trailing separators — we hand back what they set.
	tests := []struct {
		name string
		raw  string
	}{
		{"unix absolute", "/tmp/account-ME"},
		{"windows absolute with backslashes", `C:\Users\Administrator\.claude-account-ME`},
		{"windows absolute with forward slashes", "C:/Users/Administrator/.claude-account-ME"},
		{"relative", "./local-claude"},
		{"trailing separator preserved", "/tmp/account-ME/"},
		{"trailing backslash preserved", `C:\custom\`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvVar, tt.raw)
			got, err := Resolve(staticHome("/unused"))
			require.NoError(t, err)
			assert.Equal(t, tt.raw, got)
		})
	}
}
