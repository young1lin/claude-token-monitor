# StatusLine Debug Mode

## Usage

Add `--debug` flag to enable debug logging in `.claude/settings.local.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:\\\\PythonProject\\\\minimal-mcp\\\\go\\\\claude-token-monitor\\\\statusline.exe --debug",
    "env": {
      "STATUSLINE_SINGLELINE": "1"
    }
  }
}
```

## Debug File Format

**Location**: `statusline.debug` (same directory as executable)

**Format**: One line timestamp + one line raw JSON

```
2026-02-15 01:55:06
{"session_id":"...","transcript_path":"~\\.claude\\...","cwd":"C:\\Project"}
2026-02-15 01:54:30
{"session_id":"...","transcript_path":"~\\.claude\\...","cwd":"C:\\Project"}
```

**Features**:
- Keeps last 20 entries (40 lines max)
- New entries prepended at top
- User home directory masked as `~` for privacy

## Privacy: User Home Directory Masking

Automatically replaces user home path with `~`:

| Platform | Original | Masked |
|----------|----------|--------|
| Windows | `C:\Users\Username` | `~` |
| macOS | `/Users/username` | `~` |
| Linux | `/home/username` | `~` |

### Implementation

The key is handling JSON backslash escaping:

- **Windows**: In JSON, `\` becomes `\\`, so `C:\Users\xxx` → `C:\\Users\\xxx`
- **macOS/Linux**: `/` doesn't need escaping

```go
// cmd/statusline/main.go
if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
    if runtime.GOOS == "windows" {
        // Escape backslashes for JSON matching
        escapedHomeDir := strings.ReplaceAll(homeDir, "\\", "\\\\")
        debugJSON = strings.ReplaceAll(debugJSON, escapedHomeDir, "~")
    } else {
        // Unix paths use forward slashes, no escaping needed
        debugJSON = strings.ReplaceAll(debugJSON, homeDir, "~")
    }
}
```

### Why This Works

1. `os.UserHomeDir()` returns native path format:
   - Windows: `C:\Users\Username`
   - macOS: `/Users/username`
   - Linux: `/home/username`

2. Raw JSON input contains escaped paths:
   - Windows: `"path":"C:\\Users\\Username\\..."`
   - Unix: `"path":"/Users/username/..."`

3. On Windows, we must match the escaped version `C:\\Users\\...` not the native `C:\Users\...`
