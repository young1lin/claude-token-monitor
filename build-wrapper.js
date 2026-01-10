const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Build for current platform only
const platform = process.platform;
const arch = process.arch === 'arm64' ? 'arm64' : 'amd64';
const ext = platform === 'win32' ? '.exe' : '';

console.log(`Building for current platform: ${platform}-${arch}`);

// Create bin directory
fs.mkdirSync('bin', { recursive: true });

// Build monitor
console.log('Building monitor...');
const monitorOutput = path.join('bin', `claude-token-monitor-${platform}-${arch}${ext}`);
try {
  execSync(
    `GOOS=${platform} GOARCH=${arch} go build -o ${monitorOutput} -ldflags="-s -w" ./cmd/monitor`,
    { stdio: 'inherit' }
  );

  // Copy to simple name
  const simpleName = platform === 'win32' ? 'claude-token-monitor.exe' : 'claude-token-monitor';
  fs.copyFileSync(monitorOutput, path.join('bin', simpleName));
  if (platform !== 'win32') {
    fs.chmodSync(path.join('bin', simpleName), '755');
  }
  console.log(`✅ Monitor: bin/${simpleName}`);
} catch (error) {
  console.error('Failed to build monitor');
}

// Build statusline
console.log('Building statusline...');
fs.mkdirSync('statusline-plugin/bin', { recursive: true });
const statuslineOutput = path.join('statusline-plugin', 'bin', `statusline-${platform}-${arch}${ext}`);
try {
  execSync(
    `GOOS=${platform} GOARCH=${arch} go build -o ${statuslineOutput} -ldflags="-s -w" ./cmd/statusline`,
    { stdio: 'inherit' }
  );
  console.log(`✅ Statusline: statusline-plugin/bin/statusline-${platform}-${arch}${ext}`);
} catch (error) {
  console.error('Failed to build statusline');
}

console.log('\n✅ Build complete!');
