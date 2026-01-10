#!/usr/bin/env node

console.log(`
╔════════════════════════════════════════════════════════════╗
║          Claude StatusLine Plugin Installed! 🎉            ║
╚════════════════════════════════════════════════════════════╝

📍 StatusLine: Token usage will appear in Claude Code status bar
   Shows: [Model] [Progress] Tokens/Total% | Git branch | Tools | Agents | TODO

🔄 To update: npm install @young1lin/claude-statusline@latest

❓ Usage:
   • The status line updates automatically
   • Shows real-time token usage, git info, tools, agents, TODOs
   • To disable: Remove "statusLine" from ~/.claude/settings.json

⚠️  Restart Claude Code to see changes!
`);
