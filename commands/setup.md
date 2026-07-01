# Claude Token Monitor Setup

You are helping the user install or update the claude-token-monitor statusline plugin.

## Resolve the config directory (do this first, use it everywhere)

Claude Code's config directory is **not always `~/.claude`**. In multi-account
setups the user sets `$CLAUDE_CONFIG_DIR` (e.g. `~/.claude-account-ME`) and the
binary, `settings.json`, and `projects/` all live there instead. Installing into
`~/.claude/` while the user runs Claude Code with `$CLAUDE_CONFIG_DIR` set leaves
the *other* account's binary stale — the classic "I updated but nothing changed"
bug. So resolve the dir once and use it for **every** path below.

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

**IMPORTANT**: Use forward slashes `/` for paths (works on all platforms including Windows).

Read the existing **`$CC_DIR/settings.json`** (the resolved config dir — `~/.claude`
by default, or `$CLAUDE_CONFIG_DIR` when set) and merge the statusLine
configuration. Do **not** assume `~/.claude/settings.json`; if the user runs
Claude Code with `$CLAUDE_CONFIG_DIR` set, that is where Claude Code reads its
settings from.

### Path Format (2026 Best Practice)

The `command` must be the **absolute** path to the binary you just installed,
i.e. inside the resolved `$CC_DIR`. Claude Code does not expand `~` or env vars
in this field, so substitute the real resolved path.

| Platform | Default `$CC_DIR` | With `$CLAUDE_CONFIG_DIR=~/.claude-account-ME` |
|----------|-------------------|------------------------------------------------|
| Windows  | `C:/Users/username/.claude/statusline.exe` | `C:/Users/username/.claude-account-ME/statusline.exe` |
| macOS    | `/Users/username/.claude/statusline` | `/Users/username/.claude-account-ME/statusline` |
| Linux    | `/home/username/.claude/statusline` | `/home/username/.claude-account-ME/statusline` |

### Configuration Example

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:/Users/<username>/.claude/statusline.exe",
    "env": {
      "STATUSLINE_SINGLELINE": "1"
    }
  }
}
```

Replace `.claude` in the `command` with the actual last segment of `$CC_DIR`
(`.claude-account-ME` in a multi-account setup).

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

**Default behavior: NO proxy. Direct connection to `api.anthropic.com`, 60-second
cache TTL.** Skip this step if that's fine — nothing here is required.

When this step IS needed (corporate firewall, region restriction, custom
gateway), guide the user through configuration using `AskUserQuestion`. Do
**not** invent values — every parameter must come from the user.

### Step 4.1 — Ask whether to enable the proxy

Use `AskUserQuestion` with one question:

- Question: "Configure a proxy for Claude API usage requests? (Only affects
  api.anthropic.com — other tools are never proxied.)"
- header: "Proxy"
- options: `[ "No, direct connection (default)", "Yes, configure proxy" ]`

If the user picks "No" → leave the YAML proxy field empty and continue to
Step 4.6 (cache TTL).

### Step 4.2 — Pick the protocol

Only if Step 4.1 returned "Yes". Use `AskUserQuestion`:

- Question: "Which proxy protocol does the upstream support?"
- header: "Protocol"
- options:
  - `http` (most common — Clash, V2Ray default, mihomo)
  - `https` (proxy itself runs TLS — rare; pick this only if explicitly told)
  - `socks5` (SOCKS5 — also used by V2Ray, shadowsocks-rust)

Record the choice as `<proto>` (one of `http`, `https`, `socks5`).

### Step 4.3 — Pick host and port

Use `AskUserQuestion` with a few common defaults plus an "Other" path:

- Question: "Proxy host:port? Pick a common default or choose Other to type
  a custom address."
- header: "Address"
- options:
  - `127.0.0.1:7890` (Clash / mihomo default)
  - `127.0.0.1:1080` (SOCKS5 conventional port)
  - `127.0.0.1:8080` (generic HTTP proxy)

The user may select "Other" and type any `host:port`. Record as `<addr>`.

### Step 4.4 — Ask whether the proxy requires auth

Use `AskUserQuestion`:

- Question: "Does the proxy require a username and password?"
- header: "Auth"
- options: `[ "No (default)", "Yes" ]`

### Step 4.5 — Collect credentials (only when Step 4.4 is "Yes")

Use **two separate** `AskUserQuestion` calls so the answers are kept clean:

1. Username — provide a placeholder option (e.g. label "Type the username")
   and instruct the user to pick **Other** to enter their actual username.
2. Password — same shape, label "Type the password".

**You MUST URL-encode the credentials** before embedding them in the proxy
URL — `@`, `:`, `/`, `?`, `#`, space, etc. would otherwise corrupt parsing.
A safe Python one-liner the user can run if uncertain:

```bash
python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" 'my:pass@word'
```

### Step 4.6 — Compose the URL and write `.claude/statusline.yml`

Build the final URL:

| Auth | URL form |
|------|----------|
| No   | `<proto>://<addr>` |
| Yes  | `<proto>://<encoded-user>:<encoded-pass>@<addr>` |

Both `http` / `https` and `socks5` are supported by the statusline binary
(HTTP via `net/http`, SOCKS5 via `golang.org/x/net/proxy`). The user/password
pair is read directly from the URL's user-info field — no separate fields.

Then create/update `.claude/statusline.yml` (project-scoped) or
**`$CC_DIR/statusline.yml`** (global). The binary looks up the global file under
the resolved config dir (`claudedir.Resolve` → `$CLAUDE_CONFIG_DIR` or `~/.claude`),
so writing it to `~/.claude/` when `$CLAUDE_CONFIG_DIR` is set means it is never
read. **Do not commit the project-scoped file** — `.claude/statusline.yml` is
git-ignored by default to keep proxy credentials per-machine.

Final file (proxy + cache, both optional):

```yaml
network:
  # Leave empty / omit for direct connection. Examples:
  #   http://127.0.0.1:7890
  #   http://alice:p%40ss@127.0.0.1:7890   (URL-encoded credentials)
  #   socks5://bob:secret@127.0.0.1:1080
  claudeAPIProxy: "<final URL or empty>"

cache:
  # Seconds to cache a successful usage/quota response.
  # Default 90 (~40 req/hour). Non-positive falls back to 90.
  # 429 responses with Retry-After always use the server's retry window.
  usageTTLSeconds: 90
```

### Precedence (for advanced users)

The proxy URL is resolved at startup from these three sources, highest first:

1. `--proxy=<url>` CLI flag on the statusline command
2. `STATUSLINE_CLAUDE_PROXY` environment variable
3. `network.claudeAPIProxy` in YAML

`HTTP_PROXY` / `HTTPS_PROXY` environment variables are intentionally **not**
honored — they often leak from unrelated tools and must not silently route
Claude API traffic.

### Failure / rate-limit behavior (intentionally not configurable)

- Failed request → cached as failure for **15 s** before retry
- HTTP 429 → exponential backoff **60 → 120 → 240 s**, capped at **5 min**

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
