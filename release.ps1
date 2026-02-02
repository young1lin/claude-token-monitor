#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Release script for claude-token-monitor on Windows PowerShell

.DESCRIPTION
    Creates a new release with version bump, git commit, tag, and push.
#>

$ErrorActionPreference = "Stop"

# Read current version
$VERSION_FILE = Join-Path $PSScriptRoot "VERSION"
if (Test-Path $VERSION_FILE) {
    $CURRENT_VERSION = Get-Content $VERSION_FILE -Raw
    $CURRENT_VERSION = $CURRENT_VERSION.Trim()
} else {
    $CURRENT_VERSION = "0.0.0"
}

Write-Host "Current version: $CURRENT_VERSION" -ForegroundColor Cyan

# Prompt for new version
$NEW_VERSION = Read-Host "Enter new version"

if ([string]::IsNullOrWhiteSpace($NEW_VERSION)) {
    Write-Host "Error: version cannot be empty" -ForegroundColor Red
    exit 1
}

# Validate version format (basic semver check)
if ($NEW_VERSION -notmatch '^\d+\.\d+\.\d+') {
    Write-Host "Error: version must be in format x.y.z (e.g., 1.2.3)" -ForegroundColor Red
    exit 1
}

# Update VERSION file
$NEW_VERSION | Out-File -FilePath $VERSION_FILE -Encoding utf8 -NoNewline

# Git operations
Write-Host "`nCreating release..." -ForegroundColor Yellow

git add VERSION
git commit -m "chore: release v$NEW_VERSION"
git tag "v$NEW_VERSION"
git push origin main
git push origin "v$NEW_VERSION"

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "Released v$NEW_VERSION successfully!" -ForegroundColor Green
Write-Host "GitHub Actions building:" -ForegroundColor Cyan
Write-Host "https://github.com/young1lin/claude-token-monitor/actions" -ForegroundColor Blue
