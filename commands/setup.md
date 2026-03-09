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

## Step 4: Verify Installation

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
