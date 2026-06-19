# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.9] - 2026-06-19

### Added
- **🌳 tree icon when the cwd is a linked git worktree.** The git branch cell
  swaps `🌿` for `🌳` when the current directory is a linked worktree, detected
  via `git rev-parse --git-dir --git-common-dir` (unequal git-dir / common-dir
  means a linked worktree). The name after the icon is still the worktree's
  checked-out branch; the worktree directory name already shows in the `📁`
  cell, so nothing is duplicated. Detection runs inside the existing parallel
  git fetch under the 5s cache, so there is no extra subprocess cost. A new
  "Git Worktree Support" section in the README documents the
  `EnterWorktree` / `ExitWorktree` workflow (create / enter / exit).
- **cwd 是 linked git worktree 时显示 `🌳` 树图标。** 当前目录是 linked worktree 时，
  Git 分支格的 `🌿` 换成 `🌳`，通过 `git rev-parse --git-dir --git-common-dir` 检测
  （git-dir 与 common-dir 不等即为 linked worktree）。图标后显示的仍是该 worktree 检出的
  分支名；worktree 目录名已由 `📁` 格展示，不重复。检测并入已有的并行 git 采集 + 5s 缓存，
  无额外子进程开销。README 新增「Git Worktree 支持」一节，说明 `EnterWorktree` /
  `ExitWorktree` 的创建 / 进入 / 退出工作流。

## [0.2.8] - 2026-06-14

### Added
- **Idle/staleness marker on the time cell.** When the session has had no
  transcript activity for longer than a threshold, the time cell trails a
  `⏰ Xm` marker as a nudge to manually compact context. The threshold
  defaults to 300s (5 min); the marker is yellow past the threshold, red past
  3×. Configurable via the `STATUSLINE_IDLE_WARN_SECONDS` env var or the
  `format.idleWarnSeconds` YAML field; `0` disables it. Note: the statusline
  only refreshes on Claude Code triggers (new message / token change), so the
  marker shows when you return from being away and clears once a new
  transcript entry lands — it is a staleness hint, not a live watchdog.
- **时间格新增空闲/陈旧提示。** 会话超过阈值没有 transcript 活动时，时间格末尾显示
  `⏰ Xm`，提示手动压缩上下文。阈值默认 300s（5 分钟），超过变黄、超过 3× 变红；可通过
  环境变量 `STATUSLINE_IDLE_WARN_SECONDS` 或 YAML `format.idleWarnSeconds` 配置，设 `0`
  关闭。注意：状态栏只在 CC 触发时刷新（新消息 / token 变化），所以它在你离开回来时显示、
  一旦有新 transcript 条目就消失——是陈旧度提示，不是实时监控。
- **Quota reset countdown shows seconds under a minute.** The `↻` reset
  countdown previously collapsed any sub-minute reset to `<1m`; it now shows
  whole seconds, e.g. `↻ 56s`, in the final minute before a 5h/7d window
  flips. The formatter is shared, so both the Anthropic and GLM windows pick
  it up from one change.
- **额度重置倒计时最后一分钟显示秒数。** `↻` 重置倒计时此前把任何小于 1 分钟的重置
  统一显示为 `<1m`；现在显示整秒，如 `↻ 56s`，在 5h/7d 窗口翻转前更精确。共用同一
  格式化函数，Anthropic 与 GLM 一次改动同时生效。

### Internal
- **Collector layer cleanup (behavior unchanged).** The content-collector
  interface and `Manager` methods now take typed `*StatusLineInput` /
  `*TranscriptSummary` instead of `interface{}`, dropping the per-collector
  type-assertion boilerplate. The two duplicated `CommandRunner` definitions
  collapse into a new `internal/cmdrunner` package (old names retained via
  type aliases). Cell prefixes (`📁 `, `v`) moved from `main.go` into their
  collectors. Dead code removed: `getGitRemoteStatus`, and `Grid.ColWidths` /
  `calculateWidths` (the renderer recomputes column widths itself with an
  ANSI-aware width). Output is byte-for-byte unchanged.
- **采集层整洁化（行为不变）。** content collector 接口与 `Manager` 方法改为强类型
  `*StatusLineInput` / `*TranscriptSummary`，删掉每个 collector 的类型断言样板。两份重复的
  `CommandRunner` 合并到新包 `internal/cmdrunner`（旧名通过类型别名保留）。单元格前缀
  （`📁 `、`v`）从 `main.go` 移进各自 collector。删除死码：`getGitRemoteStatus`、
  `Grid.ColWidths` / `calculateWidths`（renderer 用 ANSI 感知宽度自己重算列宽）。输出逐字节
  不变。

### Docs
- **Performance benchmark added to README.** Documents the fire-and-forget
  startup-cost rationale and a measured Windows benchmark (Go 156ms vs Node
  176ms / Python 193ms / PowerShell ~2.1s / Bash ~13.3s), explaining why the
  compiled Go binary is the primary distribution while the script dirs stay
  as reference implementations.
- **README 新增性能基准。** 说明 fire-and-forget 模型下启动开销是关键指标，并给出
  Windows 实测基准（Go 156ms vs Node 176ms / Python 193ms / PowerShell ~2.1s / Bash
  ~13.3s），解释为何主力分发是 Go 编译的原生二进制、脚本目录仅作参考实现。

## [0.2.7] - 2026-06-05

### Fixed
- **Effort chip now shows every tier Claude Code reports (incl. `max` and
  `medium`).** The mode-flags collector previously rendered only
  `xhigh`/`high`/`low`: the top tier `effort.level = "max"` showed no chip at
  all, and the default `medium` was deliberately suppressed. Now `max` renders
  as a magenta top-tier chip (sharing the legacy `xhigh` colour), `medium`
  renders in cyan, and any unrecognised future tier surfaces its raw label.
  Only an absent/empty `effort.level` (older CC) stays hidden.
- **effort chip 现在显示 CC 上报的所有档位（含 `max` 与 `medium`）。** mode-flags
  此前只渲染 `xhigh`/`high`/`low`：最高档 `effort.level = "max"` 完全不显示，默认的
  `medium` 也被刻意隐藏。现在 `max` 渲染为紫色顶档 chip（与旧名 `xhigh` 同色），
  `medium` 渲染为青色，任何未知档位原样显示；仅当 `effort.level` 缺省/为空（旧版
  CC）时才隐藏。

## [0.2.6] - 2026-05-26

### Added
- **`mode-flags` indicator chip.** When Claude Code reports a non-default
  thinking mode, fast mode, or effort tier on stdin, the token cell now
  trails a small chip showing it: `[Opus 4.7 (1M context) [█░░] 58K/1M] 💭 xhigh`.
  The effort tier carries a colour (xhigh = magenta, high = yellow, low =
  green); thinking shows 💭 and fast mode ⚡. Medium effort is the implicit
  default and intentionally not rendered. Chip lives outside the brackets
  so the `[ model bar % ]` identifier stays a fixed shape.

### Changed
- **Anthropic quota uses CC 2.1.x stdin `rate_limits`, skipping the
  OAuth API.** When `rate_limits` is present in the host payload, the
  quota collector reads 5h/7d usage straight from stdin and avoids
  hitting `https://api.anthropic.com/api/oauth/usage` — saving a request
  per refresh and side-stepping the 429 backoff state machine entirely.
  Plan label (Max/Pro/Team) still comes from `.credentials.json`.
  Falls back to the OAuth API path on older CC builds that don't emit
  `rate_limits`. GLM path unaffected.
- **Claude Code version (`v2.1.150`) read from stdin instead of forking
  `claude --version`.** Eliminates a subprocess per refresh.

### Fixed
- N/A — this release is feature work, not bugfixes.

### Internal
- **Quota code split out of `time.go` (~1200 lines → ~75).** The historic
  `time.go` had grown to ~95% quota code (HTTP client, file cache, 429
  backoff state machine, Anthropic OAuth, GLM monitor). Split into one
  file per concern under `internal/statusline/content/`:
  - `time.go` — `CurrentTimeCollector` + timezone only
  - `provider.go` — provider detection (Anthropic / GLM / cache match)
  - `quota.go` — `QuotaCollector` + `UsageData` / MCP types + render
  - `quota_cache.go` — file-backed cache + 429 backoff state machine
  - `quota_http.go` — proxy + HTTP client + `parseRetryAfterHeader`
  - `quota_anthropic.go` — Anthropic OAuth fetcher + stdin fast path
  - `quota_glm.go` — GLM monitor fetcher + plan-window metadata

  Test files renamed to match (`time_cache_test.go` → `quota_cache_test.go`,
  `time_stdin_test.go` → `quota_stdin_test.go`, `glm_test.go` →
  `quota_glm_test.go`). No behaviour change; all existing tests pass
  against the new layout.
- **Unit-test speedups** to comply with `.claude/rules/unit-testing.md`
  (no `time.Sleep` in unit tests):
  - `parser`: `TestParseTranscriptCacheExpiration` previously slept 6s to
    let the 5s in-memory cache TTL expire. Introduced a `nowFn = time.Now`
    injection point in `transcript.go` and rewrote the test to advance a
    virtual clock. Package total **6.086s → 0.098s (~62×)**.
  - `content`: six tests dead-waited ~50ms each on the post-mark refresh
    coordination delay. `refreshCoordDelay` is now a `var` (was `const`)
    and `TestMain` zeroes it for the suite; the one test that exercises
    real coordination restores 50ms via `t.Cleanup`. Package total
    **0.689s → 0.357s**.

## [0.2.5] - 2026-05-26

### Changed
- **Context colour tiers expanded to 5 levels** (bright green → green → cyan → yellow → red). Red threshold moved from 60% to 75% to match AutoCompact at ~85%.
- **Extended-window (>200K) absolute-token colour tiers.** A 1M context window now warns at 180K/200K/250K absolute thresholds instead of showing green until 600K used.
- **MCP label replaced with 🧩 emoji** for horizontal space savings.
- **GLM window reset flicker fix.** 5h/7d segments now stay visible after a window resets (API briefly returns 0% with no reset time). Zero reset time renders as "↻ now".
- **Minimum fill block.** Any non-zero usage paints at least one filled block on the progress bar, preventing invisible colour on large windows.

## [0.2.4] - 2026-05-25

### Added
- **GLM/Z.ai Coding Plan quota support.** When `ANTHROPIC_BASE_URL` points to
  `api.z.ai`, `open.bigmodel.cn`, or `dev.bigmodel.cn`, the quota line now
  reads GLM's monitor quota endpoint and renders plan labels (`[Max]`,
  `[Pro]`, `[Lite]`), token windows, and MCP monthly call budgets.
- **Provider/account-isolated usage caches.** GLM cache files are keyed by
  provider plus a short fingerprint of `ANTHROPIC_AUTH_TOKEN`, preventing one
  local GLM account's quota from being shown after switching to another
  token. Anthropic keeps the legacy `.usage-cache.json` filename.

### Changed
- Default `cache.usageTTLSeconds` is now 90 seconds, reducing successful
  usage/quota polling to at most 40 requests per hour per provider/account
  while keeping quota display reasonably fresh. Users can still set 60 or
  120 seconds explicitly in YAML.
- Quota percentages and context percentages now use separate colour scales:
  quota gets more urgent as usage approaches the limit, while context gets
  more urgent as the AutoCompact threshold approaches.
- README now documents the fixed grid's glanceable-alignment goal and
  recommends Windows Terminal on Windows 11 for the most consistent column
  alignment.

### Fixed
- GLM/Z.ai quota polling now treats HTTP 429 as a rate-limit response and
  honors `Retry-After` before falling back to the existing 60 → 120 → 240 s
  exponential backoff.

## [0.2.3] - 2026-05-25

### Added
- **Multi-account support via `CLAUDE_CONFIG_DIR`.** The statusline now honors the
  `CLAUDE_CONFIG_DIR` environment variable to locate the correct `.claude/` data
  directory for users running multiple Claude Code accounts. Settings, memory
  files, and transcript paths all resolve through the configured directory.
  New package `internal/claudedir` centralizes the resolution logic.
- Tests for multi-account config resolution, memory scanning, and skills
  discovery across custom config directories.

### Changed
- Updated README (zh-CN & en-US) to document proxy/cache configuration, grid
  layout format, and debug mode usage.

## [0.2.2] - 2026-05-17

### Changed
- **Quota line switches to reset countdowns.** Inline reset times now render
  as a countdown to the next reset — the convention shared by every
  mainstream Claude/Codex statusline (ohugonnot, lee-fuhr, et al.). The
  cascade is `Xm` / `XhYm` / `XdYh` / `<1m` / `now`, and the `↻` glyph
  returns as the reset marker. The trailing `(UTC±N)` suffix is gone because
  the countdown is timezone-free. New format:
  `📊 86% 5h ↻ 4h32m · 8% 7d ↻ 1d22h`. Rationale: a countdown is directly
  actionable (no mental subtraction from the wall clock), denser than
  absolute time + TZ, and CI-friendly (no host-timezone dependency in
  tests).

## [0.2.1] - 2026-05-17

### Added
- **Configurable proxy for `api.anthropic.com` requests.** Default behavior is
  unchanged (direct connection); only when explicitly configured does the
  statusline route the OAuth-usage call through a proxy. Other HTTP traffic is
  never affected, and `HTTP_PROXY` / `HTTPS_PROXY` env vars are intentionally
  ignored to prevent leakage from unrelated tools.
  - YAML field `network.claudeAPIProxy` in `.claude/statusline.yml` (project)
    or `~/.claude/statusline.yml` (global).
  - Environment variable `STATUSLINE_CLAUDE_PROXY` for ad-hoc overrides.
  - CLI flag `--proxy=<url>` / `--proxy <url>` for one-off testing.
  - Resolution precedence: CLI flag > env > YAML > direct.
  - **HTTP, HTTPS and SOCKS5** schemes supported (`socks5h://` also accepted).
  - **Username/password authentication** read directly from the URL user-info
    (e.g. `http://alice:p%40ss@127.0.0.1:7890` or
    `socks5://bob:secret@127.0.0.1:1080`). Credentials must be URL-encoded.
- **Configurable usage-API cache TTL** via `cache.usageTTLSeconds`. Default
  60 seconds (≈ one HTTP request per minute). Non-positive values fall back
  to the 60s default so a misconfigured file can never accidentally hammer
  `api.anthropic.com` on every refresh.
- **`.yml` file extension support** alongside the existing `.yaml`.
  `.yml` is checked first; both are first-class at project and global scope.
- **Inline 7-day quota reset time** in the statusline. New format:
  `📊 22% 5h (↻ 05:20) · 2% 7d (↻ 03-24) (UTC+8)`. Each window carries its
  own reset time; timezone shown once at the end.
- **Interactive proxy setup** in the `/setup` slash command. Uses
  `AskUserQuestion` to collect: enable? → protocol (http/https/socks5) →
  host:port → auth? → username/password → writes
  `.claude/statusline.yml`. The default path is still "no proxy".
- `.gitignore` rule for `.claude/statusline.yml` / `.yaml` so project-scoped
  proxy credentials stay per-machine; `.claude/statusline.example.yaml`
  remains tracked as a template and now documents the proxy + cache fields.

### Fixed
- Subscription quota line silently disappearing when the cached 5h and 7d
  usage were both 0% — for example, immediately after a quota reset. The
  cached zero values were a legitimate "0% used" reading but
  `fallbackOrNil` mistreated them as "no data" and returned `nil`. The
  fallback now distinguishes the two cases using the `APIError` field on
  the cache record.
- Dead `cache.usageTTLSeconds` YAML field. The setting existed and had a
  default and a getter, but `time.go` was hard-coded to 60 seconds and
  never read it. It now actually drives `shouldRefreshResult`.

### Changed
- `DefaultConfig().Cache.UsageTTLSeconds` updated from 30 → 60 so the YAML
  default matches the previously effective (hard-coded) behavior.
- Quota line format unified — both 5h and 7d windows are always shown when
  the user has a subscription plan, regardless of utilization. Replaces the
  earlier branching format that omitted the cell entirely at 0%.

### Removed
- Dead helper `formatResetTime` and its test. Reset-time formatting is now
  inline per window (HH:MM for 5h, MM-DD for 7d).

### Dependencies
- Added `golang.org/x/net v0.54.0` (for `golang.org/x/net/proxy`, used by
  the SOCKS5 code path).
- Removed unused indirect `golang.org/x/sys` (via `go mod tidy`).

### Notes
- Failure cache (15 s) and 429 exponential backoff (60 → 120 → 240 s, capped
  at 5 min) remain non-configurable on purpose — they protect users from
  burning through rate limits when the upstream is unhealthy.

## [0.2.0] - 2026-03-26

### Changed
- Replaced the Skills cell (Row 1, Col 2) with a session total showing
  cumulative cost and token usage from Claude Code's stdin JSON. Added the
  `Cost` struct and `SessionID` to `StatusLineInput`. `SkillsCollector` is
  kept registered but no longer referenced by the grid layout.

### Tooling
- Bumped CI Go version to 1.25 and removed unused scripts.

### Documentation
- Added badges for Go Report Card, CI, platform, downloads, and Claude
  plugin; updated the Go version badge from 1.23+ to 1.25+; refreshed the
  statusline screenshot.
