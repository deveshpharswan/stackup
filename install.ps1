#Requires -Version 5.1
<#
.SYNOPSIS
    Install stackup CLI on Windows.
.DESCRIPTION
    Downloads the latest stackup release and adds it to PATH.
.PARAMETER InstallDir
    Directory to install stackup into. Defaults to $env:LOCALAPPDATA\Programs\stackup
#>
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\stackup"
)

$ErrorActionPreference = "Stop"
$Repo = "deveshpharswan/stackup"

function Get-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Install-Stackup {
    $arch = Get-Architecture
    Write-Host "Detecting platform: windows/$arch" -ForegroundColor Cyan

    # Get latest release
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $tag = $release.tag_name
    $version = $tag.TrimStart("v")
    Write-Host "Latest version: $version" -ForegroundColor Cyan

    # Download
    $filename = "stackup_${version}_windows_${arch}.zip"
    $url = "https://github.com/$Repo/releases/download/$tag/$filename"
    $tmpDir = Join-Path $env:TEMP "stackup-install"
    $zipPath = Join-Path $tmpDir $filename

    if (Test-Path $tmpDir) { Remove-Item $tmpDir -Recurse -Force }
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    Write-Host "Downloading $url..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

    # Extract
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

    # Install
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }
    Copy-Item (Join-Path $tmpDir "stackup.exe") -Destination (Join-Path $InstallDir "stackup.exe") -Force

    # Add to PATH if not already there
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to user PATH." -ForegroundColor Green
        Write-Host "Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
    }

    # Cleanup
    Remove-Item $tmpDir -Recurse -Force

    Write-Host ""
    Write-Host "Installed stackup $version to $InstallDir\stackup.exe" -ForegroundColor Green
    Write-Host "Run 'stackup version' to verify." -ForegroundColor Cyan
}

Install-Stackup
