# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
