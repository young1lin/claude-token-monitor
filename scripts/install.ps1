#
# Claude Token Monitor - Auto Install Script (Windows)
# This script is triggered by Claude Code plugin post-install hook
#

$ErrorActionPreference = "Stop"

# Configuration
$REPO = "young1lin/claude-token-monitor"
$INSTALL_DIR = Join-Path $env:USERPROFILE ".claude"
$BINARY_NAME = "statusline.exe"

Write-Host "[claude-token-monitor] Starting auto-update..." -ForegroundColor Green

# Detect architecture
$ARCH = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Host "[claude-token-monitor] Platform: windows/${ARCH}" -ForegroundColor Green

# Get latest version from GitHub API
Write-Host "[claude-token-monitor] Fetching latest version..." -ForegroundColor Green

try {
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/${REPO}/releases/latest" -UseBasicParsing
    $LATEST_VERSION = $response.tag_name -replace '^v', ''
} catch {
    Write-Host "Error: Failed to get latest version" -ForegroundColor Red
    exit 1
}

Write-Host "[claude-token-monitor] Latest version: v${LATEST_VERSION}" -ForegroundColor Green

# Check current version
$CURRENT_VERSION = ""
$binaryPath = Join-Path $INSTALL_DIR $BINARY_NAME

if (Test-Path $binaryPath) {
    try {
        $versionOutput = & $binaryPath --version 2>$null
        if ($versionOutput -match '(\d+\.\d+\.\d+)') {
            $CURRENT_VERSION = $Matches[1]
        }
    } catch {
        # Ignore errors
    }
}

if ($CURRENT_VERSION -eq $LATEST_VERSION) {
    Write-Host "[claude-token-monitor] Already up to date (v${CURRENT_VERSION})" -ForegroundColor Green
    exit 0
}

if ($CURRENT_VERSION) {
    Write-Host "[claude-token-monitor] Updating: v${CURRENT_VERSION} -> v${LATEST_VERSION}" -ForegroundColor Yellow
} else {
    Write-Host "[claude-token-monitor] Installing: v${LATEST_VERSION}" -ForegroundColor Green
}

# Build download URL
$ARCHIVE_NAME = "statusline_windows_${ARCH}.zip"
$DOWNLOAD_URL = "https://github.com/${REPO}/releases/download/v${LATEST_VERSION}/${ARCHIVE_NAME}"

Write-Host "[claude-token-monitor] Downloading: ${DOWNLOAD_URL}" -ForegroundColor Green

# Create temp directory
$TMP_DIR = Join-Path $env:TEMP "claude-token-monitor-$(Get-Random)"
New-Item -ItemType Directory -Path $TMP_DIR -Force | Out-Null

try {
    # Download
    $archivePath = Join-Path $TMP_DIR $ARCHIVE_NAME
    Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $archivePath -UseBasicParsing

    if (-not (Test-Path $archivePath)) {
        Write-Host "Error: Download failed" -ForegroundColor Red
        exit 1
    }

    # Extract
    Write-Host "[claude-token-monitor] Extracting..." -ForegroundColor Green
    $extractPath = Join-Path $TMP_DIR "extracted"
    Expand-Archive -Path $archivePath -DestinationPath $extractPath -Force

    # Find binary
    $extractedBinary = Get-ChildItem -Path $extractPath -Recurse -Filter "statusline.exe" | Select-Object -First 1

    if (-not $extractedBinary) {
        Write-Host "Error: Binary not found in archive" -ForegroundColor Red
        exit 1
    }

    # Create install directory if not exists
    if (-not (Test-Path $INSTALL_DIR)) {
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
    }

    # Stop any running instance (best effort)
    Get-Process -Name "statusline" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500

    # Remove old binary if exists
    if (Test-Path $binaryPath) {
        # Try to remove, if fails rename with .old suffix
        try {
            Remove-Item $binaryPath -Force
        } catch {
            $oldPath = "${binaryPath}.old"
            if (Test-Path $oldPath) {
                Remove-Item $oldPath -Force -ErrorAction SilentlyContinue
            }
            Rename-Item $binaryPath $oldPath -Force
        }
    }

    # Install
    Copy-Item $extractedBinary.FullName -Destination $binaryPath -Force

    Write-Host "[claude-token-monitor] Installed to: ${binaryPath}" -ForegroundColor Green

    # Verify
    try {
        $installedVersion = & $binaryPath --version 2>$null
        if ($installedVersion -match '(\d+\.\d+\.\d+)') {
            Write-Host "[claude-token-monitor] Verified: v$($Matches[1])" -ForegroundColor Green
        }
    } catch {
        Write-Host "[claude-token-monitor] Installed (version check skipped)" -ForegroundColor Green
    }

    Write-Host "[claude-token-monitor] Update complete!" -ForegroundColor Green

} finally {
    # Cleanup
    if (Test-Path $TMP_DIR) {
        Remove-Item $TMP_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
}
