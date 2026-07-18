# EnvContext Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a single `content.Env` built once at startup and passed to every collector via `Collect(env)`, eliminating scattered OS / terminal / `time.Now` / provider / `claudedir` reads.

**Architecture:** A new `content.Env` struct bundles all "parse-once" information (OS, terminal, now, stdin payload, transcript, provider, claude dir). `BuildEnv` reads env/config exactly once. The `ContentCollector.Collect` signature changes from `(input, summary)` to `(env)`, and collectors read `env.Input` / `env.Summary` / `env.OS` / etc. The `Composer` interface is untouched. Delivered in 4 stages, each independently shippable.

**Tech Stack:** Go 1.26, `github.com/mattn/go-runewidth`, `github.com/stretchr/testify`, `github.com/young1lin/claude-token-monitor/internal/{claudedir,cmdrunner,parser,statusline/layout,statusline/render}`.

## Global Constraints

- **`CGO_ENABLED=0`** — release builds are pure Go (`.goreleaser.yaml`); no cgo.
- **GOPROXY** — `proxy.golang.org` may time out on this machine; prefix build/test commands with `GOPROXY=https://goproxy.cn,direct`.
- **Tests follow `.claude/rules/unit-testing.md` FIRST** — no real env/process in unit tests; use `t.Setenv` and injected function vars.
- **Branch:** `refactor/envcontext` (already created). Spec: `docs/superpowers/specs/2026-07-18-envcontext-design.md`.
- Each task ends with `go test ./... -count=1` green + `go build` + a commit.

## File Structure

| File | Responsibility | Stage |
|---|---|---|
| `internal/statusline/content/env.go` (new) | `Env` struct + `BuildEnv` + `detectTerminal`/`detectProviderInfo` + injectable seams | 1, 3 |
| `internal/statusline/content/env_test.go` (new) | tests for `BuildEnv` detection branches (migrated from `main_test.go`) | 1 |
| `internal/statusline/content/types.go` | `ContentCollector.Collect(env *Env)` | 1 |
| `internal/statusline/content/manager.go` | `Compose/Get/GetAll/GetOptionalContent/collectWithTimeout` take `*Env` | 1 |
| 21 collector files (`folder.go`, `model.go`, `git.go`, `memory.go`, `skills.go`, `time.go`, `version.go`, `quota.go`, `mode_flags.go`, `session.go`, `process_memory.go`) | `Collect(env)` + read `env.Input`/`env.Summary` | 1 |
| `internal/statusline/content/quota_cache.go` | drop local `currentOS`; read `env.OS` | 1 |
| `internal/statusline/content/git.go` | `pathsEqual` uses `env.OS.IsWindows` instead of inline `runtime.GOOS` | 1 |
| `cmd/statusline/main.go` | call `BuildEnv`, pass `env` to manager, derive runewidth/layout globals from env | 1 |
| `cmd/statusline/main_test.go` | drop migrated `TestDetect*` | 1 |
| collector `*_test.go` | construct `*Env` instead of `(input, summary)` | 1 |
| `internal/statusline/content/process_memory_darwin.go` etc. | no signature change (platform helpers) | — |

---

## Stage 1 — Introduce Env + change signature + consolidate OS/Terminal

### Task 1: Create `content/env.go` with Env struct and BuildEnv

**Files:**
- Create: `internal/statusline/content/env.go`
- Create: `internal/statusline/content/env_test.go`

**Interfaces:**
- Produces: `type Env struct`, `type OSInfo`, `type TerminalInfo`, `type ProviderInfo`, `func BuildEnv(input *StatusLineInput, summary *TranscriptSummary) *Env`, injectable vars `detectTerminalFn`, `detectProviderFn`, `nowFn`, `goosFn`.

- [ ] **Step 1: Write the failing test**

`internal/statusline/content/env_test.go`:

```go
package content

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnv_PopulatesCoreFields(t *testing.T) {
	oldT, oldP, oldN, oldG := detectTerminalFn, detectProviderFn, nowFn, goosFn
	defer func() {
		detectTerminalFn, detectProviderFn, nowFn, goosFn = oldT, oldP, oldN, oldG
	}()
	fixedNow := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	nowFn = func() time.Time { return fixedNow }
	goosFn = func() string { return "darwin" }
	detectTerminalFn = func() TerminalInfo {
		return TerminalInfo{Program: "Apple_Terminal", NarrowBlock: true}
	}
	detectProviderFn = func() ProviderInfo { return ProviderInfo{Kind: "glm"} }

	in := &StatusLineInput{Cwd: "/x"}
	env := BuildEnv(in, &TranscriptSummary{})

	require.NotNil(t, env)
	assert.Equal(t, "darwin", env.OS.Name)
	assert.True(t, env.OS.IsDarwin)
	assert.False(t, env.OS.IsWindows)
	assert.Equal(t, "Apple_Terminal", env.Terminal.Program)
	assert.True(t, env.Terminal.NarrowBlock)
	assert.Equal(t, fixedNow, env.Now)
	assert.Equal(t, "glm", env.Provider.Kind)
	assert.Same(t, in, env.Input)
}

func TestDetectTerminal_AppleTerminalNarrowBlock(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "")
	goosFn = func() string { return "darwin" } // injected; restore via t? see note
	ti := detectTerminal()
	assert.True(t, ti.NarrowBlock)
	assert.False(t, ti.AmbigWide)
	assert.Equal(t, "Apple_Terminal", ti.Program)
}

func TestDetectTerminal_AmbigWideOptIn(t *testing.T) {
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "1")
	ti := detectTerminal()
	assert.True(t, ti.AmbigWide)
}

func TestDetectTerminal_WindowsCmdNarrow(t *testing.T) {
	old := goosFn
	defer func() { goosFn = old }()
	goosFn = func() string { return "windows" }
	t.Setenv("WT_SESSION", "")
	ti := detectTerminal()
	assert.True(t, ti.NarrowBlock)
}
```

Note: `goosFn` is a package var; tests that change it must restore (shown via `defer`). `t.Setenv` handles the env vars.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOPROXY=https://goproxy.cn,direct go test ./internal/statusline/content/ -run TestBuildEnv -count=1`
Expected: FAIL — `env.go` does not exist / `BuildEnv` undefined.

- [ ] **Step 3: Implement `env.go`**

`internal/statusline/content/env.go`:

```go
package content

import (
	"os"
	"runtime"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/claudedir"
)

// Env is the per-process context, built once at startup and passed to every
// collector. Bundles all "parse once" info so collectors never read env vars,
// globals, or time.Now directly.
type Env struct {
	Input     *StatusLineInput
	Summary   *TranscriptSummary
	OS        OSInfo
	Terminal  TerminalInfo
	Now       time.Time
	Provider  ProviderInfo
	ClaudeDir string
}

type OSInfo struct {
	Name      string // runtime.GOOS
	IsWindows bool
	IsDarwin  bool
}

type TerminalInfo struct {
	Program     string // raw TERM_PROGRAM
	IsWTSession bool   // WT_SESSION non-empty
	NarrowBlock bool   // Block Elements (█░) render at width 1
	AmbigWide   bool   // East Asian Ambiguous (·↻) render at width 2
}

type ProviderInfo struct {
	Kind      string // "anthropic" / "glm" / ...
	BaseURL   string // resolved ANTHROPIC_BASE_URL / ANTHROPIC_API_BASE_URL
	AuthToken string // ANTHROPIC_AUTH_TOKEN
}

// Injectable seams for unit tests (FIRST: no real env/process).
var (
	goosFn           = func() string { return runtime.GOOS }
	nowFn            = time.Now
	detectTerminalFn = detectTerminal
	detectProviderFn = detectProviderInfo
)

// BuildEnv reads all env/config once and assembles the context.
func BuildEnv(input *StatusLineInput, summary *TranscriptSummary) *Env {
	osName := goosFn()
	return &Env{
		Input:     input,
		Summary:   summary,
		OS:        OSInfo{Name: osName, IsWindows: osName == "windows", IsDarwin: osName == "darwin"},
		Terminal:  detectTerminalFn(),
		Now:       nowFn(),
		Provider:  detectProviderFn(),
		ClaudeDir: claudedir.Resolve(""),
	}
}

// detectTerminal reads terminal env once. Migrated from main.go's
// detectWideCharTerminal / detectNarrowBlockTerminal. Statusline stdin carries
// NO terminal fields (verified via --debug 2026-07-18), so this must come from
// the process environment.
func detectTerminal() TerminalInfo {
	program := os.Getenv("TERM_PROGRAM")
	ambigWide := os.Getenv("STATUSLINE_AMBIGUOUS_WIDE") == "1"
	narrow := false
	switch program {
	case "Apple_Terminal", "vscode", "WarpTerminal":
		narrow = true
	}
	if goosFn() == "windows" && os.Getenv("WT_SESSION") == "" {
		narrow = true
	}
	return TerminalInfo{
		Program:     program,
		IsWTSession: os.Getenv("WT_SESSION") != "",
		NarrowBlock: narrow,
		AmbigWide:   ambigWide,
	}
}

// detectProviderInfo resolves the API provider once. Stage 3 fills the real
// logic (migrated from provider.go / quota_glm.go); Stage 1 leaves it empty so
// existing provider detection keeps working unchanged.
func detectProviderInfo() ProviderInfo {
	return ProviderInfo{}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOPROXY=https://goproxy.cn,direct go test ./internal/statusline/content/ -run 'TestBuildEnv|TestDetectTerminal' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/statusline/content/env.go internal/statusline/content/env_test.go
git commit -m "refactor(content): add Env context with BuildEnv and terminal detection"
```

---

### Task 2: Change `Collect` signature to `Collect(env *Env)` (atomic migration)

This is an atomic signature change — Go interfaces require all implementors to change together, so the tree will not compile until every collector + manager + main is updated. Work top-to-bottom, then compile once at the end.

**Files:**
- Modify: `internal/statusline/content/types.go:53` (interface)
- Modify: `internal/statusline/content/manager.go` (5 functions)
- Modify: 11 collector files (21 `Collect` methods) — see list below
- Modify: `cmd/statusline/main.go` (build env, call `Compose(env)`)
- Modify: all collector `*_test.go` (construct `*Env`)

**Interfaces:**
- Consumes: `Env`, `BuildEnv` from Task 1.
- Produces: `ContentCollector.Collect(env *Env) (string, error)`; `Manager.Compose(env *Env) layout.CellContent`.

**The 21 collectors** (signature transform is uniform):

```
skills.go    SkillsCollector
version.go   ClaudeVersionCollector
memory.go    MemoryFilesCollector
folder.go    FolderCollector
mode_flags.go ModeFlagsCollector
model.go     ModelCollector, TokenBarCollector, TokenInfoCollector, SessionTotalCollector
git.go       GitBranchCollector, GitStatusCollector, GitRemoteCollector, GitWorktreeCollector
quota.go     QuotaCollector
time.go      CurrentTimeCollector
session.go   AgentCollector, TodoCollector, ToolsCollector, SessionDurationCollector, ToolStatusDetailCollector
process_memory.go ParentMemoryCollector
```

**Transform pattern:** For every `Collect`, change the signature and add ONE line at the top extracting the previously-named param, leaving the rest of the body byte-for-byte identical.

| Old first param name | Add at top of body |
|---|---|
| `statusInput *StatusLineInput` | `statusInput := env.Input` |
| `transcriptSummary *TranscriptSummary` | `transcriptSummary := env.Summary` |
| `summary *TranscriptSummary` | `summary := env.Summary` |
| `_ *StatusLineInput, _ *TranscriptSummary` (unused) | (nothing) |

- [ ] **Step 1: Change the interface**

`internal/statusline/content/types.go:51-57`:

```go
type ContentCollector interface {
	Type() ContentType
	Collect(env *Env) (string, error)
	CacheTTL() time.Duration
	Timeout() time.Duration
	Optional() bool
}
```

- [ ] **Step 2: Update `manager.go`**

Replace the `(input *StatusLineInput, summary *TranscriptSummary)` parameter pair with `(env *Env)` in these 5 functions, and pass `env` to `collector.Collect`:

- `Get(contentType ContentType, env *Env) (string, error)` — body: `collector.Collect(env)` (was `Collect(input, summary)`).
- `collectWithTimeout(ct ContentType, env *Env) (string, bool)` — passes `env` to `m.Get`.
- `GetAll(env *Env) map[ContentType]string`
- `GetOptionalContent(env *Env) map[ContentType]string`
- `Compose(env *Env) layout.CellContent` — body: `m.GetAll(env)` and `m.GetOptionalContent(env)`.

Inside `Get`, `cacheMu`/cache logic unchanged.

- [ ] **Step 3: Transform all 21 collectors**

For each collector file, change every `Collect(<old> *StatusLineInput, <old> *TranscriptSummary)` to `Collect(env *Env)` and insert the extraction line.

Representative examples:

`folder.go:125`:
```go
func (c *FolderCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	name := getProjectName(statusInput.Cwd)
	if name == "" {
		return "", nil
	}
	return "\U0001F5C2️ " + name, nil
}
```

`session.go:23` (AgentCollector):
```go
func (c *AgentCollector) Collect(env *Env) (string, error) {
	transcriptSummary := env.Summary
	// ... rest unchanged, uses transcriptSummary
```

`process_memory.go:112` (uses neither):
```go
func (c *ParentMemoryCollector) Collect(env *Env) (string, error) {
	mb, err := defaultProcessMemoryReader.ReadParentMemoryMB()
	// ... rest unchanged
```

Apply the same one-line transform to the remaining 18 methods listed above.

- [ ] **Step 4: Update `main.go` to build env and pass it**

In `run()` (`cmd/statusline/main.go`), after `summary` is assembled (around line 211) and before `contentMgr.Compose` (line 236):

```go
env := content.BuildEnv(&input, summary)
contentMap := contentMgr.Compose(env)
```

(Replace the existing `contentMgr.Compose(&input, summary)` line.)

Leave the existing `runewidth.DefaultCondition` / `layout.UseNarrowBlockWidth` global setup in `init()` untouched for now — Stage 4 removes it.

- [ ] **Step 5: Update collector tests to construct `*Env`**

In each collector `*_test.go`, replace direct `Collect(input, summary)` calls with `Collect(&Env{Input: input, Summary: summary})`. Representative (`folder_test.go`):

```go
result, err := collector.Collect(&Env{Input: tt.input, Summary: &TranscriptSummary{}})
```

For tests that only need summary (e.g. `session_test.go`), use `&Env{Summary: tt.summary}`. Apply across all collector test files.

- [ ] **Step 6: Build and test**

Run:
```bash
GOPROXY=https://goproxy.cn,direct go build ./...
GOPROXY=https://goproxy.cn,direct go test ./... -count=1
```
Expected: build OK, all tests PASS. If a collector test still calls the old signature, the compiler points at it — fix and re-run.

- [ ] **Step 7: End-to-end sanity**

```bash
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json
```
Expected: same 3-line output as before (folder, git, time/quota/memory) — no regression.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor(content): change Collect signature to Collect(env *Env)"
```

---

### Task 3: Consolidate OS reads onto `env.OS`

**Files:**
- Modify: `internal/statusline/content/quota_cache.go:19` (delete `currentOS`)
- Modify: `internal/statusline/content/memory.go:84` (use `env.OS.IsWindows`)
- Modify: `internal/statusline/content/git.go:400` (use `env.OS.IsWindows`)
- Modify: `cmd/statusline/main.go:29` (delete main-package `currentOS`; debug masking reads `env.OS.IsWindows`)

**Interfaces:**
- Consumes: `env.OS.IsWindows` (now available to collectors via `Collect(env)`).

- [ ] **Step 1: Delete `currentOS` in content package**

`internal/statusline/content/quota_cache.go:19` — remove `var currentOS = runtime.GOOS`. Find its readers:
- `memory.go:84` `if currentOS == "windows"` → this is inside `Collect`'s call chain? Check: `memory.go` `getEnterpriseClaudeMDPath` (line ~84) is called from `Collect`. It needs OS. Change it to accept `isWindows bool` or read from a passed `*Env`. Simplest: thread `env.OS.IsWindows` into the helper.

`memory.go` — change the helper signature to take `isWindows bool` and pass `env.OS.IsWindows` from `Collect`:
```go
func (c *MemoryFilesCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	// ... pass env.OS.IsWindows to the enterprise-path check
```
(If `getEnterpriseClaudeMDPath` is only called here, add an `isWindows bool` param.)

- [ ] **Step 2: `quota_cache.go` Windows rename guard**

`quota_cache.go:243` uses `currentOS == "windows"` before `os.Rename`. `quota_cache` is invoked from `QuotaCollector.Collect` → `getSubscriptionQuota`. Thread `env.OS.IsWindows` down, OR keep a package-level setter set from `BuildEnv`. Prefer threading: add `isWindows bool` param to the cache write function called from the quota path.

- [ ] **Step 3: `git.go:400` inline**

`git.go:400` `pathsEqual` uses `runtime.GOOS == "windows"`. `pathsEqual` is called from git collectors which now have `env`. Thread `env.OS.IsWindows` into `pathsEqual` (add param) or read from a collector-held value. Simplest: add `isWindows bool` param to `pathsEqual` and pass from callers.

- [ ] **Step 4: Delete `currentOS` in main package**

`cmd/statusline/main.go:29` — remove `var currentOS = runtime.GOOS`. Its readers:
- `main.go:151` debug JSON backslash masking — change to read `env.OS.IsWindows` (env is built before debug write? Check order: debug write is at line 137-192, env built later. **Reorder**: build `env` earlier, or keep a local `isWindows := runtime.GOOS == "windows"` inline at the debug site — acceptable since it's main-package glue, not a collector).

Use the inline local at the debug site:
```go
isWindows := runtime.GOOS == "windows"
if isWindows {
	escapedHomeDir := strings.ReplaceAll(homeDir, "\\", "\\\\")
	debugJSON = strings.ReplaceAll(debugJSON, escapedHomeDir, "~")
} else {
	debugJSON = strings.ReplaceAll(debugJSON, homeDir, "~")
}
```

- [ ] **Step 5: Build, test, verify, commit**

```bash
GOPROXY=https://goproxy.cn,direct go build ./... && \
GOPROXY=https://goproxy.cn,direct go test ./... -count=1 && \
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline && \
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json
```
Expected: build OK, tests PASS, output unchanged.

```bash
git add -A
git commit -m "refactor(content): consolidate OS reads onto env.OS, drop duplicate currentOS"
```

---

## Stage 2 — Single source of "now"

### Task 4: Replace scattered `time.Now()` with `env.Now`

**Files:**
- Modify: `internal/statusline/content/git.go:108`, `memory.go:50`, `version.go:49`, `quota.go:199`, `quota_anthropic.go:154`, `quota_cache.go:224/273/355/407`, `manager.go:89`, `types.go:67`
- Leave: `internal/parser/transcript.go:110` `nowFn` (out of scope — parser keeps its own)

**Interfaces:**
- Consumes: `env.Now`.

- [ ] **Step 1: Thread `env.Now` into collectors**

In each collector `Collect(env)`, the body can read `env.Now`. Replace direct `time.Now()` calls with `env.Now`:
- `git.go:108` (cache TTL check) — `gitCombinedCache` uses `time.Now()`. Thread `now time.Time` from `Collect`'s `env.Now` into `getGitDataParallel`.
- `memory.go:50`, `version.go:49` — replace `time.Now()` with `env.Now` (thread into helpers as needed).
- `quota.go:199` uses `nowFn()` — repoint: in `Collect`, set the package `nowFn = func() time.Time { return env.Now }`? No — cleaner: replace `nowFn()` reads with a passed `now`. Add `now time.Time` param to `getSubscriptionQuota`.
- `quota_anthropic.go:154`, `quota_cache.go:*` — thread `now` (from `env.Now`) into the quota/cache helpers.
- `manager.go:89` (`expiresAt: time.Now().Add(...)`) and `types.go:67` (`isExpired` uses `time.Now()`) — these are cache mechanics. `isExpired` needs "now"; thread `env.Now` via the cache check, or accept that cache expiry uses real `time.Now()` (it's infra, not display). **Decision:** leave `types.go:67 isExpired` and `manager.go:89` on real `time.Now()` — they're cache internals, not "display current time". Document this.

- [ ] **Step 2: Verify time-dependent cells unchanged**

The `🕐` time, quota countdown, and session duration cells must still show correct values.

```bash
GOPROXY=https://goproxy.cn,direct go test ./... -count=1
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json
```
Expected: tests PASS; time/quota/duration cells render normally.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor(content): use env.Now as single time source in collectors"
```

---

## Stage 3 — Consolidate provider + ClaudeDir

### Task 5: Fill `env.Provider` and read from it

**Files:**
- Modify: `internal/statusline/content/env.go` (`detectProviderInfo` — fill real logic)
- Modify: `internal/statusline/content/provider.go:45` (`detectProvider`)
- Modify: `internal/statusline/content/quota_glm.go:79` (`glmBaseURL`), `:112` (`getGLMAuthToken`)

**Interfaces:**
- Produces: `env.Provider.{Kind,BaseURL,AuthToken}` populated once in `BuildEnv`.

- [ ] **Step 1: Implement `detectProviderInfo`**

`env.go` — replace the stub:
```go
func detectProviderInfo() ProviderInfo {
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROPIC_API_BASE_URL")
	}
	kind := "anthropic"
	if isGLMBaseURL(baseURL) { // existing helper in quota_glm.go
		kind = "glm"
	}
	return ProviderInfo{
		Kind:      kind,
		BaseURL:   baseURL,
		AuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
	}
}
```
(Reuse `isGLMBaseURL` / the GLM-detection predicate already in `quota_glm.go` — do not duplicate.)

- [ ] **Step 2: Make quota path read `env.Provider`**

`provider.go:detectProvider` → return `env.Provider.Kind`. `quota_glm.go:glmBaseURL` → return `env.Provider.BaseURL`. `getGLMAuthToken` → return `env.Provider.AuthToken`. Thread `env` (or `env.Provider`) from `QuotaCollector.Collect` into `getSubscriptionQuota`.

This removes the second `ANTHROPIC_BASE_URL` read.

- [ ] **Step 3: Test, verify, commit**

```bash
GOPROXY=https://goproxy.cn,direct go test ./... -count=1
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json
git add -A
git commit -m "refactor(content): resolve provider once in env.Provider, drop duplicate env reads"
```

### Task 6: Resolve `ClaudeDir` once onto `env.ClaudeDir`

**Files:**
- Modify: `internal/statusline/content/memory.go`, `skills.go`, `quota_anthropic.go`, `quota_cache.go` — replace `claudedir.Resolve(...)` calls with `env.ClaudeDir`.

- [ ] **Step 1: Thread `env.ClaudeDir`**

`BuildEnv` already sets `ClaudeDir: claudedir.Resolve("")`. In each of the four files, the call site is reachable from a `Collect(env)` — pass `env.ClaudeDir` into the helper instead of calling `claudedir.Resolve` again.

- [ ] **Step 2: Test, verify, commit**

```bash
GOPROXY=https://goproxy.cn,direct go test ./... -count=1 && \
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline && \
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json && \
git add -A && git commit -m "refactor(content): resolve Claude config dir once onto env.ClaudeDir"
```

---

## Stage 4 — Remove `layout.UseNarrowBlockWidth` global

### Task 7: Renderer holds `narrow` instead of reading a global

**Files:**
- Modify: `internal/statusline/layout/renderer.go:17` (delete `UseNarrowBlockWidth`), `:33` (`displayWidth`), `:58` (`NewRenderer`)
- Modify: `internal/statusline/render/table.go` (`NewTableRenderer` threads narrow)
- Modify: `cmd/statusline/main.go` (pass `env.Terminal.NarrowBlock`)
- Modify: `internal/statusline/layout/renderer_test.go`, `renderer_noalign_test.go` (construct with narrow)

**Interfaces:**
- Produces: `layout.NewRenderer(grid *Grid, narrow bool) *Renderer`; `render.NewTableRenderer(grid *Grid, narrow bool)`.

- [ ] **Step 1: Write failing test**

`renderer_test.go` — update tests that set `UseNarrowBlockWidth = true/false` to instead construct `NewRenderer(grid, true)` / `NewRenderer(grid, false)`. E.g.:
```go
r := NewRenderer(grid, true) // narrow mode
got := displayWidthWith(r, "[██░░░░░░░░]")
```
(`displayWidth` becomes a method on `Renderer`, or takes `narrow`.)

- [ ] **Step 2: Refactor renderer**

`renderer.go`:
- Delete `var UseNarrowBlockWidth = false`.
- Add field `narrow bool` to `Renderer`; `NewRenderer(grid *Grid, narrow bool) *Renderer { return &Renderer{grid: grid, narrow: narrow} }`.
- Make `displayWidth` a method `(r *Renderer) displayWidth(s string) int` using `r.narrow`, OR pass `narrow` to `calculateColumnWidths`/`renderRowWithAlignment`. Prefer method form.

`render/table.go`: `NewTableRenderer(grid *Grid, narrow bool)` stores narrow, passes to `layout.NewRenderer` in `Render()`.

- [ ] **Step 3: Update `main.go`**

Replace `layout.UseNarrowBlockWidth = detectNarrowBlockTerminal()` with nothing (delete), and at render site:
```go
renderer := render.NewTableRenderer(grid, env.Terminal.NarrowBlock)
```
Also delete `detectNarrowBlockTerminal` from main.go (logic now lives in `detectTerminal` in env.go from Task 1).

- [ ] **Step 4: Test, verify, commit**

```bash
GOPROXY=https://goproxy.cn,direct go test ./... -count=1
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json | sed 's/ /./g'
```
Expected: tests PASS; `|` columns still align (first `|` @24, second @68).

```bash
git add -A
git commit -m "refactor(layout): renderer holds narrow flag, drop UseNarrowBlockWidth global"
```

---

## Self-Review

**1. Spec coverage:**
- Env struct (OS/Terminal/Now/Provider/ClaudeDir/Input/Summary) → Task 1. ✓
- `Collect(env)` signature → Task 2. ✓
- OS consolidation (3 sites) → Task 3. ✓
- `time.Now` consolidation → Task 4. ✓
- provider/baseURL/authToken → Task 5. ✓
- `claudedir.Resolve` → Task 6. ✓
- `layout.UseNarrowBlockWidth` removal → Task 7. ✓
- Terminal source = os.Getenv (evidence) → encoded in `detectTerminal` (Task 1). ✓
- Composer unchanged → noted in File Structure; no task touches it. ✓
- Out-of-scope: new stdin fields → not in any task. ✓

**2. Placeholder scan:** `detectProviderInfo` stub in Task 1 is intentional (filled in Task 5) and labeled as such — not a placeholder failure. No "TBD"/"add error handling"/"similar to". ✓

**3. Type consistency:** `Env`, `OSInfo`, `TerminalInfo`, `ProviderInfo` defined once (Task 1) and used consistently. `Collect(env *Env)` signature identical in interface (Task 2 Step 1) and all collectors (Task 2 Step 3). `NewRenderer(grid, narrow)` consistent across Task 7 steps. ✓

## Execution Handoff

(see following message)
