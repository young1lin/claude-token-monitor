package parser

import "os/exec"

// CommandRunner executes a command in a directory and returns its output.
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

// defaultCommandRunner is the runner used by parser functions.
// Tests can replace this with a stub.
var defaultCommandRunner CommandRunner = &RealCommandRunner{}
