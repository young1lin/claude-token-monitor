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

**All platforms**: `$HOME/.claude/`

```
~/.claude/
├── projects/           # Session data for all projects
│   ├── C--Users-...-project1/
│   │   ├── session-id-1.jsonl
│   │   └── session-id-2.jsonl
│   └── C--Users-...-project2/
│       └── session-id-3.jsonl
├── settings.json       # Global settings
└── CLAUDE.md           # Global instructions (if exists)
```

It parses `type: "assistant"` messages to extract token usage data.
