const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const platforms = [
  { goos: 'windows', goarch: 'amd64', ext: '.exe' },
  { goos: 'windows', goarch: 'arm64', ext: '.exe' },
  { goos: 'darwin', goarch: 'amd64', ext: '' },
  { goos: 'darwin', goarch: 'arm64', ext: '' },
  { goos: 'linux', goarch: 'amd64', ext: '' },
  { goos: 'linux', goarch: 'arm64', ext: '' }
];

console.log('Building claude-token-monitor for all platforms...');

// Create bin directories
fs.mkdirSync('bin', { recursive: true });
fs.mkdirSync('statusline-plugin/bin', { recursive: true });

// Build monitor for all platforms
console.log('\nBuilding monitor...');
platforms.forEach(platform => {
  console.log(`  Building for ${platform.goos} ${platform.goarch}...`);
  const output = path.join('bin', `claude-token-monitor-${platform.goos}-${platform.goarch}${platform.ext}`);
  try {
    execSync(
      `GOOS=${platform.goos} GOARCH=${platform.goarch} go build -o ${output} -ldflags="-s -w" ./cmd/monitor`,
      { stdio: 'inherit' }
    );
  } catch (error) {
    console.error(`Failed to build monitor for ${platform.goos}-${platform.goarch}`);
  }
});

// Build statusline for all platforms
console.log('\nBuilding statusline...');
platforms.forEach(platform => {
  console.log(`  Building for ${platform.goos} ${platform.goarch}...`);
  const output = path.join('statusline-plugin', 'bin', `statusline-${platform.goos}-${platform.goarch}${platform.ext}`);
  try {
    execSync(
      `GOOS=${platform.goos} GOARCH=${platform.goarch} go build -o ${output} -ldflags="-s -w" ./cmd/statusline`,
      { stdio: 'inherit' }
    );
  } catch (error) {
    console.error(`Failed to build statusline for ${platform.goos}-${platform.goarch}`);
  }
});

console.log('\n✅ Build complete!');
console.log('\nMonitor binaries:');
fs.readdirSync('bin')
  .filter(f => f.startsWith('claude-token-monitor-'))
  .forEach(f => console.log(`  bin/${f}`));

console.log('\nStatusline binaries:');
fs.readdirSync(path.join('statusline-plugin', 'bin'))
  .forEach(f => console.log(`  statusline-plugin/bin/${f}`));
