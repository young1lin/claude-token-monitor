# Unit Testing Guidelines

All unit tests MUST follow the **FIRST** principles. No exceptions.

## FIRST Principles

### Fast
- Tests MUST run in milliseconds, not seconds.
- NO external dependencies: no real filesystem, no network, no real git, no real HTTP calls.
- NO `time.Sleep` in unit tests. Use mock clocks or inject time providers.
- NO `exec.Command` calls to real binaries. Use dependency injection to mock command execution.

### Isolated
- Each test MUST run independently. No shared mutable state between tests.
- NO dependency on execution order. Tests MUST pass when run alone or in any combination.
- NO reliance on the host environment (OS, installed tools, environment variables, working directory).
- Use mocks, stubs, fakes, and dependency injection to isolate the unit under test.

### Repeatable
- Tests MUST produce the same result every time, on every machine, in every environment.
- NO flaky tests. If a test depends on timing, filesystem state, or external services, it is NOT a unit test.
- Use `t.TempDir()` for filesystem needs in integration tests, but prefer fakes/stubs for unit tests.

### Self-validating
- Each test MUST have a clear pass/fail outcome. No manual inspection required.
- Use `t.Errorf`, `t.Fatalf`, `require.NoError`, `assert.Equal`, etc. DO NOT use `t.Logf` as the primary assertion.
- Every test MUST follow the **AAA pattern** (Arrange, Act, Assert).

### Timely
- Write tests at the same time as production code (or immediately after).
- Do NOT leave untested code to "test later."
- When adding a new feature, the test coverage MUST NOT decrease.

## Dependency Injection for Testability

All code that interacts with external systems (filesystem, network, processes, environment variables) MUST use dependency injection:

```go
// BAD: Direct dependency, impossible to unit test without real git
func getGitBranch(cwd string) string {
    cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
    cmd.Dir = cwd
    output, err := cmd.Output()
    // ...
}

// GOOD: Inject a runner interface, mock it in tests
type CommandRunner interface {
    Run(dir string, args ...string) ([]byte, error)
}

func getGitBranch(cwd string, runner CommandRunner) string {
    output, err := runner.Run(cwd, "git", "symbolic-ref", "--short", "HEAD")
    // ...
}
```

## Test Doubles Selection

| Double | When to Use |
|--------|-------------|
| **Fake** | In-memory implementation (e.g., in-memory filesystem, fake clock) |
| **Mock** | Verify interactions (e.g., "was this function called with these args?") |
| **Stub** | Provide predefined responses (e.g., return fixed git output) |
| **Spy** | Record calls and return canned responses |

Prefer **Fakes** for complex dependencies and **Mocks** for verifying behavior.

## Go-Specific Rules

- Use `testing` package + `testify` (assert/require/mock) + `gomock` as needed.
- Table-driven tests for parameterized scenarios.
- `t.TempDir()` for filesystem operations (auto-cleaned).
- Export test helpers only if shared across packages; keep them `_test.go` otherwise.
- Use `go test ./... -count=1` to disable caching during CI.
