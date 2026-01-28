# Claude Token Monitor Setup

You are helping the user install the claude-token-monitor statusline plugin.

## Step 1: Detect Platform

Detect the user's platform:
- **Windows (win32)**: amd64
- **macOS (darwin)**: amd64 or arm64
- **linux**: amd64 or arm64

Check with:
- Windows: Check if `uname` exists, otherwise assume Windows
- macOS/Linux: `uname -s` for OS, `uname -m` for architecture

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
# Extract
Expand-Archive -Path "$env:TEMP\statusline.zip" -DestinationPath "$env:USERPROFILE\.claude\" -Force
# Cleanup
Remove-Item "$env:TEMP\statusline.zip"
```

### macOS/Linux
```bash
# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture
if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
fi

# Download URL
URL="https://github.com/young1lin/claude-token-monitor/releases/latest/download/statusline_${OS}_${ARCH}.tar.gz"

# Download and extract
curl -L "$URL" | tar -xz -C "$HOME/.claude/"

# Make executable
chmod +x "$HOME/.claude/statusline"
```

## Step 3: Configure settings.json

Read the existing `~/.claude/settings.json`:

```bash
cat ~/.claude/settings.json
```

Add/update the `statusLine` configuration:

```json
{
  "statusLine": {
    "type": "command",
    "command": "%USERPROFILE%\.claude\statusline.exe"  // Windows
    // or
    "command": "/home/user/.claude/statusline"  // macOS/Linux
  }
}
```

**Important**: Merge with existing settings, don't overwrite!

### Windows path
`%USERPROFILE%\.claude\statusline.exe`

### macOS/Linux path
`$HOME/.claude/statusline`

## Step 4: Verify Installation

Ask the user to check if the statusline appears in Claude Code.

If it doesn't appear, check:
1. Binary exists at the correct path
2. Binary has execute permission (macOS/Linux: `chmod +x`)
3. settings.json has correct statusLine configuration
4. Path uses double backslashes on Windows: `\\`

## Troubleshooting

### "Command not found"
- Check the binary path in settings.json
- Windows: Use `\\` for path separators (e.g., `C:\\Users\\....\\statusline.exe`)
- macOS/Linux: Use absolute path starting with `/`

### "Permission denied"
- macOS/Linux: Run `chmod +x ~/.claude/statusline`

### Statusline not updating
- Try sending a new message in Claude Code
- Check that Claude Code is reading the correct settings.json
