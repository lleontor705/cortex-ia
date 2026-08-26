# ─────────────────────────────────────────────────────────────────────────────
# install.ps1 — Cortex-IA Windows Installer
#
# Usage:
#   irm https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.ps1 | iex
# ─────────────────────────────────────────────────────────────────────────────

$ErrorActionPreference = "Stop"

$Repo = "lleontor705/cortex-ia"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\cortex-ia\bin"
$ExePath = Join-Path $InstallDir "cortex-ia.exe"

function Write-Cyan($msg)  { Write-Host "[INFO]  $msg" -ForegroundColor Cyan }
function Write-Green($msg) { Write-Host "[OK]    $msg" -ForegroundColor Green }
function Write-Red($msg)   { Write-Host "[ERROR] $msg" -ForegroundColor Red }

Write-Host @"

  ╔═══════════════════════════════════════════════════════════╗
  ║              cortex-ia Windows Installer                  ║
  ║  AI Agent Ecosystem & Multiplexer Configurator            ║
  ╚═══════════════════════════════════════════════════════════╝

"@ -ForegroundColor Cyan

# 1. Detect Architecture
$Arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Red "32-bit Windows is not supported."
    exit 1
}

Write-Cyan "Detected Windows ($Arch)"

# 2. Create Target Directory
if (!(Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

# 3. Locate or Download Binary
$LocalBin = Join-Path $PSScriptRoot "..\bin\cortex-ia.exe"
if (Test-Path -Path $LocalBin) {
    Write-Cyan "Installing from local build: $LocalBin"
    Copy-Item -Path $LocalBin -Destination $ExePath -Force
} else {
    Write-Cyan "Fetching latest release version from GitHub..."
    try {
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $Release.tag_name
        $ArchiveName = "cortex-ia_${($Version -replace '^v','')}_windows_${Arch}.zip"
        $DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$ArchiveName"

        Write-Cyan "Downloading $ArchiveName..."
        $TempZip = Join-Path ([System.IO.Path]::GetTempPath()) $ArchiveName
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing

        Write-Cyan "Extracting..."
        Expand-Archive -Path $TempZip -DestinationPath $InstallDir -Force
        Remove-Item -Path $TempZip -Force
    } catch {
        Write-Cyan "Release archive not found, attempting go install..."
        if (Get-Command go -ErrorAction SilentlyContinue) {
            go install "github.com/$Repo/cmd/cortex-ia@latest"
            $GopathBin = Join-Path (go env GOPATH) "bin\cortex-ia.exe"
            if (Test-Path -Path $GopathBin) {
                Copy-Item -Path $GopathBin -Destination $ExePath -Force
            }
        } else {
            Write-Red "Failed to download binary: $_"
            exit 1
        }
    }
}

if (!(Test-Path -Path $ExePath)) {
    Write-Red "cortex-ia.exe was not found in $InstallDir"
    exit 1
}

# 4. Add to User PATH
$UserPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Cyan "Adding $InstallDir to User PATH..."
    [System.Environment]::SetEnvironmentVariable("PATH", "$InstallDir;$UserPath", "User")
    $env:PATH = "$InstallDir;$env:PATH"
}
Write-Green "PATH configured."

# 5. Set Environment Variable OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true
Write-Cyan "Configuring OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true..."
[System.Environment]::SetEnvironmentVariable("OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS", "true", "User")
$env:OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS = "true"
Write-Green "Environment variable OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true set."

# 6. Run cortex-ia sync to deploy MCPs, plugins, and agents
Write-Cyan "Deploying Cortex-IA ecosystem, plugins, and agents..."
& $ExePath sync

Write-Host @"

  Installation complete!
  Quick Start:
    cortex-ia          # Launch Interactive TUI & Management Console
    cortex-ia web      # Launch Web Console (http://127.0.0.1:7331)
    cortex-ia herdr    # Diagnose Herdr and AGY multiplexing

"@ -ForegroundColor Green
