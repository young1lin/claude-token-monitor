package content

import "os/exec"

// CommandRunner executes a command in a directory and returns its output.
// Tests can override defaultCommandRunner with a stub to avoid real process execution.
type CommandRunner interface {
	Run(dir string, name string, args ...string) ([]byte, error)
}

// RealCommandRunner executes real commands via os/exec.
type RealCommandRunner struct{}

// Run executes the given command in the specified directory.
func (r *RealCommandRunner) Run(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

// defaultCommandRunner is the runner used by all git functions.
// Tests can replace this with a StubCommandRunner to avoid real git execution.
var defaultCommandRunner CommandRunner = &RealCommandRunner{}
