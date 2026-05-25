# Claude Token Monitor Setup

You are helping the user install or update the claude-token-monitor statusline plugin.

## Step 0: Version Check (Update Flow)

**Check if already installed:**

```bash
# Check if binary exists
ls ~/.claude/statusline* 2>/dev/null || echo "NOT_INSTALLED"

# Get local version (if installed)
~/.claude/statusline --version 2>/dev/null || echo "VERSION_UNKNOWN"
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
# Download
Invoke-WebRequest -Uri "https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_windows_amd64.zip" -OutFile "$env:TEMP\statusline.zip"
# Extract to ~/.claude/
Expand-Archive -Path "$env:TEMP\statusline.zip" -DestinationPath "$env:USERPROFILE\.claude\" -Force
# Cleanup
Remove-Item "$env:TEMP\statusline.zip"
# Verify
& "$env:USERPROFILE\.claude\statusline.exe" --version
```

### macOS/Linux

```bash
# Download URL
URL="https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_${OS}_${ARCH}.tar.gz"

# Download and extract to ~/.claude/
curl -L "$URL" | tar -xz -C "$HOME/.claude/"

# Make executable
chmod +x "$HOME/.claude/statusline"

# Verify
~/.claude/statusline --version
```

## Step 3: Configure settings.json

**IMPORTANT**: Use forward slashes `/` for paths (works on all platforms including Windows).

Read the existing `~/.claude/settings.json` and merge the statusLine configuration.

### Path Format (2026 Best Practice)

| Platform | Path Format |
|----------|-------------|
| Windows | `C:/Users/username/.claude/statusline.exe` |
| macOS | `/Users/username/.claude/statusline` |
| Linux | `/home/username/.claude/statusline` |

**Recommended**: Use `$HOME` expansion where possible:
- Unix: `$HOME/.claude/statusline`
- Windows: Use full path like `C:/Users/username/.claude/statusline.exe`

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

**Windows path example:**
```json
"command": "C:/Users/YourName/.claude/statusline.exe"
```

**macOS/Linux path example:**
```json
"command": "/Users/username/.claude/statusline"
// or
"command": "$HOME/.claude/statusline"
```

**Important**:
1. Merge with existing settings, don't overwrite!
2. Use forward slashes `/` (not `\\` or `\\\\`)
3. Avoid `%USERPROFILE%` - use actual path or `$HOME`

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
`~/.claude/statusline.yml` (global). **Do not commit the project-scoped file**
— `.claude/statusline.yml` is git-ignored by default to keep proxy
credentials per-machine.

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
# Verify binary works
~/.claude/statusline --version

# Verify settings.json
cat ~/.claude/settings.json
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
- macOS/Linux: Run `chmod +x ~/.claude/statusline`

### Statusline not updating
- Try sending a new message in Claude Code
- Check that Claude Code is reading the correct settings.json

### Version mismatch after update
- Restart Claude Code to reload the binary
- Verify the binary path in settings.json matches the installed location
