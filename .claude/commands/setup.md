# Claude Token Monitor Setup

You are helping the user install or update the claude-token-monitor statusline plugin.

## Resolve the config directory (do this first, use it everywhere)

Claude Code's config directory is **not always `~/.claude`**. In multi-account
setups the user sets `$CLAUDE_CONFIG_DIR` (e.g. `~/.claude-account-ME`) and the
binary, `settings.json`, and `projects/` all live there instead. Installing into
`~/.claude/` while the user runs Claude Code with `$CLAUDE_CONFIG_DIR` set leaves
the *other* account's binary stale — the classic "I updated but nothing changed"
bug. Resolve the dir once and use it for **every** path below.

Resolution (matches the binary's own `claudedir.Resolve` — env var wins, then
`~/.claude`):

- If `$CLAUDE_CONFIG_DIR` is set and non-empty → use it.
- Otherwise → `~/.claude` (`$USERPROFILE\.claude` on Windows).

```bash
# macOS / Linux
CC_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
```

```powershell
# Windows (PowerShell)
$CC_DIR = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $env:USERPROFILE '.claude' }
```

Every `~/.claude/...` in the steps below means `$CC_DIR/...`. Substitute the
resolved absolute path when you write `settings.json` (the `command` field must
be absolute — Claude Code does not expand `~` or env vars there).

## Step 0: Version Check (Update Flow)

**Check if already installed:**

```bash
# Check if binary exists (use the resolved $CC_DIR, not ~/.claude)
ls "$CC_DIR"/statusline* 2>/dev/null || echo "NOT_INSTALLED"

# Get local version (if installed)
"$CC_DIR"/statusline --version 2>/dev/null || echo "VERSION_UNKNOWN"
# Output: "statusline version 0.1.12 (commit: abc1234)"
```

**Get latest version from server:**

```bash
# Option 1: GitHub API (recommended)
curl -s https://api.github.com/repos/young1lin/claude-token-monitor/releases/latest | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'

# Option 2: Direct VERSION file
curl -s https://raw.githubusercontent.com/young1lin/claude-token-monitor/main/VERSION
```

**Compare versions:**

```
Local: 0.1.10
Server: 0.1.12
→ Server > Local → Proceed with update

Local: 0.1.12
Server: 0.1.12
→ Already up to date, skip download
```

**Version comparison logic (semver):**
- Parse major.minor.patch
- Compare major first, then minor, then patch
- If server > local → download and update

## Step 1: Detect Platform

Detect the user's platform:
- **Windows (win32)**: amd64
- **macOS (darwin)**: amd64 or arm64
- **linux**: amd64 or arm64

Check with:
- Windows: Check if `uname` exists, otherwise assume Windows
- macOS/Linux: `uname -s` for OS, `uname -m` for architecture

```bash
# macOS/Linux
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture
if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
fi
```

## Step 2: Download Binary

Base URL: `https://github.com/young1lin/claude-token-monitor/releases/latest/download/`

File mappings:
| Platform | Arch | File |
|----------|------|------|
| Windows | amd64 | `statusline_windows_amd64.zip` |
| macOS | amd64 | `statusline_darwin_amd64.tar.gz` |
| macOS | arm64 | `statusline_darwin_arm64.tar.gz` |
| Linux | amd64 | `statusline_linux_amd64.tar.gz` |
| Linux | arm64 | `statusline_linux_arm64.tar.gz` |

### Windows (PowerShell)

```powershell
# $CC_DIR resolved earlier (honors $CLAUDE_CONFIG_DIR, else $USERPROFILE\.claude)
# Download
Invoke-WebRequest -Uri "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_windows_amd64.zip" -OutFile "$env:TEMP\statusline.zip"
# Extract into the resolved config dir
Expand-Archive -Path "$env:TEMP\statusline.zip" -DestinationPath "$CC_DIR\" -Force
# Cleanup
Remove-Item "$env:TEMP\statusline.zip"
# Verify
& "$CC_DIR\statusline.exe" --version
```

### macOS/Linux

```bash
# $CC_DIR resolved earlier (honors $CLAUDE_CONFIG_DIR, else $HOME/.claude)
# Download URL
URL="https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_${OS}_${ARCH}.tar.gz"

# Download and extract into the resolved config dir
curl -L "$URL" | tar -xz -C "$CC_DIR/"

# Make executable
chmod +x "$CC_DIR/statusline"

# Verify
"$CC_DIR"/statusline --version
```

## Step 3: Configure settings.json

**IMPORTANT**: Skip this step if updating — only configure on first install.

Use forward slashes `/` for paths (works on all platforms including Windows).

Read the existing **`$CC_DIR/settings.json`** (the resolved config dir — `~/.claude`
by default, or `$CLAUDE_CONFIG_DIR` when set) and merge the statusLine
configuration. Do **not** assume `~/.claude/settings.json`; if the user runs
Claude Code with `$CLAUDE_CONFIG_DIR` set, that is where Claude Code reads from.

### Path Format (2026 Best Practice)

The `command` must be the **absolute** path to the binary you just installed,
i.e. inside the resolved `$CC_DIR`. Claude Code does not expand `~` or env vars
in this field, so substitute the real resolved path.

| Platform | Default `$CC_DIR` | With `$CLAUDE_CONFIG_DIR=~/.claude-account-ME` |
|----------|-------------------|------------------------------------------------|
| Windows  | `C:/Users/username/.claude/statusline.exe` | `C:/Users/username/.claude-account-ME/statusline.exe` |
| macOS    | `/Users/username/.claude/statusline` | `/Users/username/.claude-account-ME/statusline` |
| Linux    | `/home/username/.claude/statusline` | `/home/username/.claude-account-ME/statusline` |

**Recommended**: Use `$HOME` expansion only when `$CLAUDE_CONFIG_DIR` is unset:
- Unix (default): `$HOME/.claude/statusline`
- Windows (default): full path like `C:/Users/username/.claude/statusline.exe`

### Configuration Example

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:/Users/<username>/.claude/statusline.exe"
  }
}
```

Replace `.claude` in the `command` with the actual last segment of `$CC_DIR`
(`.claude-account-ME` in a multi-account setup).

> **Note**: `STATUSLINE_SINGLELINE=1` is the default behavior, no env override needed.

**Windows path example:**
```json
"command": "C:/Users/YourName/.claude/statusline.exe"
```

**macOS/Linux path example:**
```json
"command": "/Users/username/.claude/statusline"
// or, only when $CLAUDE_CONFIG_DIR is unset:
"command": "$HOME/.claude/statusline"
```

**Important**:
1. Merge with existing settings, don't overwrite!
2. Use forward slashes `/` (not `\\` or `\\\\`)
3. Avoid `%USERPROFILE%` - use actual path or `$HOME`
4. The `command` path and the file's own location must share the same `$CC_DIR`
   — writing `settings.json` to `~/.claude/` while pointing `command` at (or
   installing the binary under) a different dir is exactly the mismatch that
   breaks updates.

## Step 4: Optional — Configure Proxy & Cache (interactive)

**Default behavior: NO proxy. Direct connection to `api.anthropic.com`,
60-second cache TTL.** Skip this step entirely if that's fine.

When this step IS needed, use `AskUserQuestion` to collect every parameter
from the user — do not invent values. Both HTTP/HTTPS and SOCKS5 are
supported, with optional username/password.

### 4.1 — Enable proxy? (`AskUserQuestion`)

- Question: "Configure a proxy for Claude API usage requests? (Only
  api.anthropic.com — other tools are never proxied.)"
- header: "Proxy"
- options:
  - "No, direct connection (default)"
  - "Yes, configure proxy"

If "No" → leave proxy empty, jump to 4.6 (cache TTL only).

### 4.2 — Protocol (`AskUserQuestion`)

- Question: "Which proxy protocol does the upstream support?"
- header: "Protocol"
- options:
  - `http` (Clash/mihomo/V2Ray default)
  - `https` (TLS-wrapped proxy — rare)
  - `socks5` (SOCKS5)

Record as `<proto>`.

### 4.3 — Address (`AskUserQuestion`)

- Question: "Proxy host:port? Pick a common default or Other to type a
  custom address."
- header: "Address"
- options:
  - `127.0.0.1:7890` (Clash / mihomo default)
  - `127.0.0.1:1080` (SOCKS5 conventional)
  - `127.0.0.1:8080` (generic HTTP proxy)
  - (user can pick Other to type any host:port)

Record as `<addr>`.

### 4.4 — Auth required? (`AskUserQuestion`)

- Question: "Does the proxy require a username and password?"
- header: "Auth"
- options: `[ "No (default)", "Yes" ]`

### 4.5 — Credentials (only if 4.4 = "Yes")

Two separate `AskUserQuestion` calls — instruct the user to pick **Other**
each time to type the actual value:

1. Username — header "Username"
2. Password — header "Password"

**URL-encode** before embedding (special characters like `@`, `:`, `/`, `#`,
space must be percent-encoded). Quick helper:

```bash
python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" 'p@ss:word'
```

### 4.6 — Assemble URL + write `.claude/statusline.yml`

| Auth | URL |
|------|-----|
| No   | `<proto>://<addr>` |
| Yes  | `<proto>://<enc-user>:<enc-pass>@<addr>` |

Then write the file: project-scoped `.claude/statusline.yml` (git-ignored,
credentials stay local) or global **`$CC_DIR/statusline.yml`** — the binary
loads the global file from the resolved config dir (`claudedir.Resolve`), so
writing it to `~/.claude/` when `$CLAUDE_CONFIG_DIR` is set means it is never
read.

```yaml
network:
  # Examples:
  #   http://127.0.0.1:7890
  #   http://alice:p%40ss@127.0.0.1:7890       (URL-encoded credentials)
  #   socks5://bob:secret@127.0.0.1:1080
  claudeAPIProxy: "<final URL or empty>"

cache:
  usageTTLSeconds: 60   # default 60s; non-positive → fallback to 60s
```

### Precedence and non-configurable items

- Precedence: `--proxy=<url>` CLI flag > `STATUSLINE_CLAUDE_PROXY` env > YAML
- `HTTP_PROXY` / `HTTPS_PROXY` are intentionally ignored (no leakage)
- Failure cache (15 s) and 429 backoff (60 → 120 → 240, cap 5 min) are fixed

## Step 5: Verify Installation

Ask the user to check if the statusline appears in Claude Code.

```bash
# Verify binary works (in the resolved $CC_DIR, not ~/.claude)
"$CC_DIR"/statusline --version

# Verify settings.json lives in the SAME resolved config dir
cat "$CC_DIR/settings.json"
```

If it doesn't appear, check:
1. Binary exists at the correct path
2. Binary has execute permission (macOS/Linux: `chmod +x`)
3. settings.json has correct statusLine configuration
4. Path uses forward slashes `/`

## Troubleshooting

### "Command not found"
- Check the binary path in settings.json
- Use forward slashes `/` for path separators (works on Windows too)
- Ensure the path is absolute (starts with `C:/` on Windows or `/` on Unix)

### "Permission denied"
- macOS/Linux: Run `chmod +x "$CC_DIR"/statusline`

### Statusline not updating
- Try sending a new message in Claude Code
- Check that Claude Code is reading the correct settings.json

### Version mismatch after update / "I updated but nothing changed"
This is almost always a `$CLAUDE_CONFIG_DIR` mismatch: the binary was installed
to `~/.claude/` while Claude Code runs with `$CLAUDE_CONFIG_DIR` set (or vice
versa), so the *other* config dir still holds the old binary.
- Re-run setup with `$CC_DIR` resolved from `$CLAUDE_CONFIG_DIR` (see top of this
  file) and install into that exact dir.
- Confirm `command` in `settings.json`, the `settings.json` file itself, and the
  binary on disk all live under the same `$CC_DIR`.
- Restart Claude Code to reload the binary.
