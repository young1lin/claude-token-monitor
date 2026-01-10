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

// Binary name mapping for monitor
const binaryMap = {
  'win32': {
    'amd64': 'claude-token-monitor-windows-amd64.exe',
    'arm64': 'claude-token-monitor-windows-arm64.exe',
    '386': 'claude-token-monitor-windows-386.exe'
  },
  'darwin': {
    'amd64': 'claude-token-monitor-darwin-amd64',
    'arm64': 'claude-token-monitor-darwin-arm64'
  },
  'linux': {
    'amd64': 'claude-token-monitor-linux-amd64',
    'arm64': 'claude-token-monitor-linux-arm64',
    '386': 'claude-token-monitor-linux-386'
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

function install() {
  console.log(`Installing claude-token-monitor...`);
  console.log(`Platform: ${platform}, Architecture: ${arch}`);

  const binaryName = getBinaryName();
  const sourcePath = path.join(__dirname, 'bin', binaryName);
  const destDir = path.join(__dirname, 'bin');
  const destPath = path.join(destDir, platform === 'win32' ? 'claude-token-monitor.exe' : 'claude-token-monitor');

  // Create bin directory
  fs.mkdirSync(destDir, { recursive: true });

  // Check if binary exists
  if (!fs.existsSync(sourcePath)) {
    console.error(`Binary not found: ${sourcePath}`);
    console.error(`Please run: npm run build:all`);
    process.exit(1);
  }

  // Copy binary to simpler name
  fs.copyFileSync(sourcePath, destPath);

  // Make executable on Unix
  if (platform !== 'win32') {
    fs.chmodSync(destPath, '755');
  }

  console.log(`Monitor binary installed to: ${destPath}`);
  console.log(`Installation complete!`);
  console.log(`Run: claude-token-monitor`);
}

install();
