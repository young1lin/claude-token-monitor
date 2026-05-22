// Package claudedir resolves Claude Code's per-account config directory.
//
// In multi-account setups users set $CLAUDE_CONFIG_DIR (e.g.
// ~/.claude-account-ME) so that credentials, the usage cache, projects/, and
// settings.json all live in a dedicated tree rather than the historical
// ~/.claude. Every statusline data source that reaches into that tree must
// route through this package, otherwise it silently reports the *other*
// account's state.
package claudedir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// EnvVar is the standard Claude Code multi-account override.
const EnvVar = "CLAUDE_CONFIG_DIR"

// HomeProvider mirrors os.UserHomeDir's signature. Pass os.UserHomeDir for
// default behavior, a method like (*FileSystem).UserHomeDir when the caller
// has its own FS abstraction, or a closure in tests.
type HomeProvider func() (string, error)

// ErrNoConfigDir is returned when neither $CLAUDE_CONFIG_DIR nor a usable home
// directory is available. Callers should treat this as "no usable config dir"
// and skip their work — it is never a precondition violation.
var ErrNoConfigDir = errors.New("claudedir: $CLAUDE_CONFIG_DIR unset and no usable home directory")

// Resolve returns the active Claude Code config dir.
//
// Resolution order, matching Claude Code itself:
//  1. $CLAUDE_CONFIG_DIR (trimmed) when non-empty — multi-account setups
//  2. filepath.Join(home(), ".claude") — historical default
//
// home is invoked only when the env var is empty or whitespace-only. If home
// is nil or returns ("", err) and the env var is unset, ErrNoConfigDir is
// returned (wrapping the home error if any).
func Resolve(home HomeProvider) (string, error) {
	if v := strings.TrimSpace(os.Getenv(EnvVar)); v != "" {
		return v, nil
	}
	if home == nil {
		return "", ErrNoConfigDir
	}
	h, err := home()
	if err != nil {
		return "", err
	}
	if h == "" {
		return "", ErrNoConfigDir
	}
	return filepath.Join(h, ".claude"), nil
}
