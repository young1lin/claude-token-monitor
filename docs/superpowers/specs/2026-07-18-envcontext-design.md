# EnvContext 重构设计

**日期**:2026-07-18
**分支**:`refactor/envcontext`
**状态**:待 review

## 背景与动机

statusline 是 fire-and-forget 短命进程,每次刷新独立。当前进程内的"环境/配置信息"**分散获取、重复读取**:

- **OS 判断散在 3 处**:两个同名 `currentOS`(`cmd/statusline/main.go:29` 的 main 包、`internal/statusline/content/quota_cache.go:19` 的 content 包)+ `internal/statusline/content/git.go:400` 的 inline `runtime.GOOS`
- **终端检测**在 `init()` 读一次,但结果只落到两个全局(`runewidth.DefaultCondition.EastAsianWidth`、`layout.UseNarrowBlockWidth`),**没有可传递的结构**;任何 collector 想知道"是不是 Apple_Terminal"只能重新 `os.Getenv`
- **`time.Now()` 散落 11+ 处**(3 个独立 `nowFn`/`timeZoneFn` + 8 处直接调用),同次刷新的"当前时间"各取各的
- **`ANTHROPIC_BASE_URL`** 同次刷新被 `provider.go:detectProvider` 和 `quota_glm.go:glmBaseURL` 各读一次
- **`claudedir.Resolve`** 被 memory/skills/quota_anthropic/quota_cache 等 5+ 处各自调用

## 目标

引入一个 `content.Env`,进程启动时构建**一次**,作为唯一参数传给每个 collector。collector 不再自己 `os.Getenv` / 读全局 / 调 `time.Now`,统一从 `env` 取。消除重复、让环境信息可注入可测试、给未来新 collector 一个统一入口。

## 非目标(scope 边界)

- **不纳入 Claude Code stdin 的新字段**:`exceeds_200k_tokens`、`context_window.used_percentage`/`remaining_percentage`、`output_style.name`、`workspace.added_dirs`/`repo`、`prompt_id`、`session_name`。这些 statusline 当前未解析,本次不动,另开一次。
- **不改 composer 接口**:`Compose(map[ContentType]string)` 本就不碰 env/runtime,保持原样。
- **不动业务数据缓存层**:git/quota/memory/version 的进程内/跨进程 TTL 缓存保持现状。
- **parser 包的 `nowFn`**(阶段 2)默认保留,不强行改其公开接口。

## 架构

### Env 结构 — `internal/statusline/content/env.go`(新建)

放 content 包,因为它要装 `StatusLineInput`/`TranscriptSummary`(定义在 `folder.go`),放别处会循环依赖。layout 包不依赖 content,渲染时只需一个 `narrow bool`(阶段 4 处理)。

```go
package content

import "time"

// Env is the per-process context, built once at startup and passed to every
// collector. Bundles all "parse once" info: host OS, terminal capabilities,
// an authoritative "now", the Claude Code stdin payload + transcript summary,
// and resolved provider/config values.
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

// BuildEnv reads all env/config once and assembles the context.
func BuildEnv(input *StatusLineInput, summary *TranscriptSummary) *Env { ... }
```

### collector 接口 — `internal/statusline/content/types.go:51`

```go
type ContentCollector interface {
    Type() ContentType
    Collect(env *Env) (string, error)   // 原 Collect(input, summary)
    CacheTTL() time.Duration
    Timeout() time.Duration
    Optional() bool
}
```

collector 内部把 `input`/`summary` 改从 `env.Input`/`env.Summary` 取;OS/终端/时间/provider 从对应字段取。

### BuildEnv 职责

把现在散在 `main.go init()`、`provider.go`、`quota_glm.go`、各 `claudedir.Resolve` 的读取**各调一次**,组装 Env。终端/provider 检测逻辑从 main.go 迁入 content 包。

### 数据流

```
stdin → json.Unmarshal → input
     → parser.ParseTranscript → summary
     → BuildEnv(input, summary) → env
     → manager.Compose(env) → collector.Collect(env) → CellContent
     → layout.Grid → render.TableRenderer → stdout
```

## 终端信息来源(关键依据)

**实证(2026-07-18,`--debug` 捕获真实 stdin)**:Claude Code 的 stdin 顶层字段为 `context_window, cost, cwd, effort, exceeds_200k_tokens, fast_mode, model, output_style, prompt_id, session_id, session_name, thinking, transcript_path, version, workspace`,嵌套字段也无任何终端相关项。

**结论**:stdin **不提供终端信息**。`Env.Terminal` 必须由 `BuildEnv` 通过 `os.Getenv("TERM_PROGRAM")` / `("WT_SESSION")` / `("STATUSLINE_AMBIGUOUS_WIDE")` 检测(把 `main.go` 的 `detectWideCharTerminal`/`detectNarrowBlockTerminal` 逻辑迁入 content 包),**不能**从 `env.Input` 取。本设计据此固定 Terminal 来源。

## 分阶段实施

每阶段:**测试绿 + `go build` + 端到端 `bin/statusline` 验证 + 独立 commit**。

### 阶段 1 — 引入 Env + 改签名 + 收 OS/Terminal(基础,最大)

- 新建 `content/env.go`:`Env` 结构 + `BuildEnv` + 终端检测(从 main.go 迁入)。
- 改 `ContentCollector.Collect(env)`(`types.go:53`)+ 21 个 collector 实现签名 + 内部 `env.Input`/`env.Summary`。
- 改 `manager.go` 全部调用点传 `env`(`Compose`/`Get`/`GetAll`/`GetOptionalContent`/`collectWithTimeout`)。
- **迁移 OS**:删 `content/quota_cache.go:19` 与 `main.go:29` 的 `currentOS`;`git.go:400` inline `runtime.GOOS` 改 `env.OS.IsWindows`。
- `main.go`:`BuildEnv` 后仍设 `runewidth.DefaultCondition`(=`env.Terminal.AmbigWide`)+ `layout.UseNarrowBlockWidth`(=`env.Terminal.NarrowBlock`),行为零变化。
- 测试:collector 测试构造字面 `*Env`;`main_test.go` 的 `TestDetect*` 迁到 `content/env_test.go`。

### 阶段 2 — 统一 Now

- `env.Now` 在 `BuildEnv` 时 `time.Now()` 一次。
- 直接 `time.Now()` 调用(`git.go:108`、`memory.go:50`、`version.go:49`、`quota.go:199`、`quota_anthropic.go:154`、`quota_cache.go:273/355/407`、`manager.go:89`、`types.go:67`、`main.go:176`)改 `env.Now`。
- `time.go:15` `timeZoneFn`、`quota.go:15` `nowFn` 指向 `env.Now`;parser 的 `nowFn` 保留(非目标)。

### 阶段 3 — 收 provider / ClaudeDir

- `env.Provider` 在 `BuildEnv` 解析一次(`detectProvider` + `glmBaseURL` + `getGLMAuthToken` 合并)。
- `provider.go`、`quota_glm.go` 改读 `env.Provider`;**消除 `ANTHROPIC_BASE_URL` 二次读取**。
- `env.ClaudeDir = claudedir.Resolve("")` 一次;`memory.go`/`skills.go`/`quota_anthropic.go`/`quota_cache.go` 改读 `env.ClaudeDir`。

### 阶段 4 — layout 去 `UseNarrowBlockWidth` 全局

- `renderer.go`:`displayWidth` 不读全局,改成 `Renderer` 持有 `narrow bool`(`NewRenderer(grid, narrow)`)。
- `main.go` 从 `env.Terminal.NarrowBlock` 传入;删全局;`renderer_test.go`/`renderer_noalign_test.go` 改构造参数。

## 测试策略

- `BuildEnv` 用 `t.Setenv` 测终端/provider 检测分支(遵循 `.claude/rules/unit-testing.md` 的 FIRST)。
- collector 测试 stub 从 `(input, summary)` 升级为构造 `*Env`(机械批量,字面 `&Env{...}`)。
- 每阶段:`go test ./... -count=1 -race` + `go build` + 手跑 `bin/statusline` 核对 `\|` 对齐、内存、🗂️ 未回归。

## 风险与回滚

- **阶段 1 是大头**(21 collector + 测试改签名),机械但量大;完成后后续阶段轻量。中途要停,阶段 1 本身已交付价值(消除 OS 三处重复 + 统一终端 context)。
- parser 包(阶段 2)按非目标保留其 `nowFn`,不影响主线。
- 每阶段独立 commit,任何阶段出问题可单独 revert。

## 验证(端到端)

```bash
GOPROXY=https://goproxy.cn,direct go test ./... -count=1 -race
GOPROXY=https://goproxy.cn,direct go build -o bin/statusline ./cmd/statusline
TERM_PROGRAM=Apple_Terminal STATUSLINE_NO_COLOR=1 ./bin/statusline < /tmp/sl_input.json
```
