package content

import "github.com/young1lin/claude-token-monitor/internal/cmdrunner"

// CommandRunner / RealCommandRunner alias the shared cmdrunner package so the
// content collectors keep their local names while the implementation lives in
// exactly one place (see internal/cmdrunner). Tests can still override
// defaultCommandRunner with a stub to avoid real process execution.
type CommandRunner = cmdrunner.Runner

// RealCommandRunner executes real commands via os/exec.
type RealCommandRunner = cmdrunner.Real

// defaultCommandRunner is the runner used by all git/version functions.
var defaultCommandRunner CommandRunner = &cmdrunner.Real{}
