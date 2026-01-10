# Cross-platform build script for claude-token-monitor (Windows)

Write-Host "Building claude-token-monitor for all platforms..." -ForegroundColor Green

# Create bin directories
New-Item -ItemType Directory -Force -Path bin | Out-Null
New-Item -ItemType Directory -Force -Path statusline-plugin\bin | Out-Null

# Platforms to build
$platforms = @(
    @{ OS="windows"; ARCH="amd64"; EXE=".exe" },
    @{ OS="windows"; ARCH="arm64"; EXE=".exe" },
    @{ OS="darwin"; ARCH="amd64"; EXE="" },
    @{ OS="darwin"; ARCH="arm64"; EXE="" },
    @{ OS="linux"; ARCH="amd64"; EXE="" },
    @{ OS="linux"; ARCH="arm64"; EXE="" }
)

# Build monitor
Write-Host "Building monitor..." -ForegroundColor Cyan
foreach ($p in $platforms) {
    Write-Host "  Building for $($p.OS)-$($p.ARCH)..."
    $env:GOOS = $p.OS
    $env:GOARCH = $p.ARCH
    go build -o "bin\claude-token-monitor-$($p.OS)-$($p.ARCH)$($p.EXE)" -ldflags="-s -w" ./cmd/monitor
}

# Build statusline
Write-Host "Building statusline..." -ForegroundColor Cyan
foreach ($p in $platforms) {
    Write-Host "  Building for $($p.OS)-$($p.ARCH)..."
    $env:GOOS = $p.OS
    $env:GOARCH = $p.ARCH
    go build -o "statusline-plugin\bin\statusline-$($p.OS)-$($p.ARCH)$($p.EXE)" -ldflags="-s -w" ./cmd/statusline
}

Write-Host "Build complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Monitor binaries:"
Get-ChildItem bin\ | Where-Object { $_.Name -like "claude-token-monitor-*" } | Format-Table Name, Length

Write-Host ""
Write-Host "Statusline binaries:"
Get-ChildItem statusline-plugin\bin\ | Format-Table Name, Length

Write-Host ""
Write-Host "To create npm packages: npm run pack:monitor && npm run pack:statusline"
