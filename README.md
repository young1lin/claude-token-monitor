# Claude Token Monitor

A TUI tool for real-time Claude Code token usage monitoring.

## Features

- Real-time token usage display
- Context window percentage visualization
- Cost estimation
- Session history with persistence
- Cross-platform (Windows, macOS, Linux)

## Installation

```bash
go install github.com/young1lin/claude-token-monitor/cmd/monitor@latest
```

## Usage

```bash
claude-token-monitor
```

## Requirements

- Go 1.23+
- Claude Code installed

## How it works

The tool monitors Claude Code's JSONL session files located at:
- Windows: `%APPDATA%\Claude\`
- macOS: `~/Library/Application Support/Claude/`
- Linux: `~/.config/Claude/`

It parses `type: "assistant"` messages to extract token usage data.
