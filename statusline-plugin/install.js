#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');

// Platform detection
const platform = os.platform();
const arch = os.arch();

// Map node arch to go arch
const archMap = {
  'x64': 'amd64',
  'arm64': 'arm64',
  'ia32': '386'
};

const goArch = archMap[arch] || arch;

// Binary name mapping
const binaryMap = {
  'win32': {
    'amd64': 'statusline-windows-amd64.exe',
    'arm64': 'statusline-windows-arm64.exe',
    '386': 'statusline-windows-386.exe'
  },
  'darwin': {
    'amd64': 'statusline-darwin-amd64',
    'arm64': 'statusline-darwin-arm64'
  },
  'linux': {
    'amd64': 'statusline-linux-amd64',
    'arm64': 'statusline-linux-arm64',
    '386': 'statusline-linux-386'
  }
};

function getBinaryName() {
  if (!binaryMap[platform]) {
    console.error(`Unsupported platform: ${platform}`);
    process.exit(1);
  }
  if (!binaryMap[platform][goArch]) {
    console.error(`Unsupported architecture: ${goArch} on ${platform}`);
    process.exit(1);
  }
  return binaryMap[platform][goArch];
}

function getClaudeConfigPath() {
  const claudeConfigDir = process.env.CLAUDE_CONFIG_DIR;
  if (claudeConfigDir) {
    return claudeConfigDir;
  }
  return path.join(os.homedir(), '.claude');
}

function install() {
  console.log(`Installing claude-statusline...`);
  console.log(`Platform: ${platform}, Architecture: ${arch}`);

  const binaryName = getBinaryName();
  const sourcePath = path.join(__dirname, 'bin', binaryName);
  const claudeConfigPath = getClaudeConfigPath();
  const destDir = path.join(claudeConfigPath, 'claude-statusline');
  const destPath = path.join(destDir, platform === 'win32' ? 'statusline.exe' : 'statusline');

  // Check if binary exists
  if (!fs.existsSync(sourcePath)) {
    console.error(`Binary not found: ${sourcePath}`);
    console.error(`Please run: npm run build:all (in parent directory)`);
    process.exit(1);
  }

  // Create destination directory
  fs.mkdirSync(destDir, { recursive: true });

  // Copy binary
  fs.copyFileSync(sourcePath, destPath);

  // Make executable on Unix
  if (platform !== 'win32') {
    fs.chmodSync(destPath, '755');
  }

  console.log(`Binary installed to: ${destPath}`);

  // Update Claude Code settings
  const settingsPath = path.join(claudeConfigPath, 'settings.json');

  let settings = {};
  if (fs.existsSync(settingsPath)) {
    try {
      const content = fs.readFileSync(settingsPath, 'utf8');
      settings = JSON.parse(content);
    } catch (e) {
      console.warn(`Warning: Could not parse settings.json, creating new`);
    }
  }

  // Add statusLine configuration
  settings.statusLine = {
    type: 'command',
    command: destPath
  };

  // Write settings
  fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2));

  console.log(`Claude Code settings updated`);
  console.log(`Installation complete!`);
  console.log(`Restart Claude Code to see the status line.`);
}

install();
