# Phase 22: PowerShell Install Script

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** Phase 18 (CI/CD Setup - for release binaries)

## Objective

Create a PowerShell installation script that mirrors the functionality of `install.sh`, enabling one-line installation of Pommel on Windows.

## Background

### Current Unix Installation

`scripts/install.sh` provides:
- Architecture detection (amd64/arm64)
- Binary download from GitHub releases
- Installation to user-accessible location
- PATH configuration
- Ollama installation and model pulling
- Verification

### Target Installation Experience

```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

Or for users who prefer to review first:

```powershell
# Download and review
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1" -OutFile "install.ps1"
# Run after review
.\install.ps1
```

## Implementation Tasks

### Task 1: Review Existing install.sh

**File:** `scripts/install.sh`

**Document functionality:**
- [ ] Architecture detection logic
- [ ] Download URL construction
- [ ] Installation directory choice
- [ ] PATH modification approach
- [ ] Ollama installation steps
- [ ] Model pulling
- [ ] Error handling
- [ ] User feedback messages

### Task 2: Create PowerShell Script Structure

**File:** `scripts/install.ps1`

```powershell
#Requires -Version 5.1

<#
.SYNOPSIS
    Installs Pommel semantic code search tool on Windows.

.DESCRIPTION
    Downloads and installs pm.exe and pommeld.exe from GitHub releases,
    configures PATH, installs Ollama if needed, and pulls the embedding model.

.EXAMPLE
    irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex

.NOTES
    Requires: PowerShell 5.1+, Windows 10 1709+ (for winget)
#>

[CmdletBinding()]
param(
    [switch]$SkipOllama,
    [switch]$SkipModel,
    [string]$InstallDir = "$env:LOCALAPPDATA\Pommel"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"  # Speed up downloads

# Configuration
$script:Repo = "dbinky/Pommel"
$script:OllamaModel = "unclemusclez/jina-embeddings-v2-base-code"

# Colors for output
function Write-Step { param($Message) Write-Host "=> $Message" -ForegroundColor Cyan }
function Write-Success { param($Message) Write-Host "✓ $Message" -ForegroundColor Green }
function Write-Warning { param($Message) Write-Host "! $Message" -ForegroundColor Yellow }
function Write-Failure { param($Message) Write-Host "✗ $Message" -ForegroundColor Red }
```

### Task 3: Implement Architecture Detection

```powershell
function Get-Architecture {
    <#
    .SYNOPSIS
        Detects CPU architecture for binary download.
    #>

    $arch = $env:PROCESSOR_ARCHITECTURE

    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" { throw "32-bit Windows is not supported" }
        default { throw "Unknown architecture: $arch" }
    }
}
```

### Task 4: Implement GitHub Release Detection

```powershell
function Get-LatestRelease {
    <#
    .SYNOPSIS
        Gets the latest release version from GitHub.
    #>

    $apiUrl = "https://api.github.com/repos/$script:Repo/releases/latest"

    try {
        $release = Invoke-RestMethod -Uri $apiUrl -Headers @{
            "Accept" = "application/vnd.github.v3+json"
        }
        return $release.tag_name
    }
    catch {
        throw "Failed to get latest release: $_"
    }
}

function Get-DownloadUrl {
    param(
        [string]$Version,
        [string]$Binary,
        [string]$Arch
    )

    $fileName = "$Binary-windows-$Arch.exe"
    return "https://github.com/$script:Repo/releases/download/$Version/$fileName"
}
```

### Task 5: Implement Binary Download

```powershell
function Install-PommelBinaries {
    param(
        [string]$Version,
        [string]$Arch,
        [string]$InstallDir
    )

    $binDir = Join-Path $InstallDir "bin"

    # Create directory if needed
    if (-not (Test-Path $binDir)) {
        New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    }

    # Download pm.exe
    Write-Step "Downloading pm.exe..."
    $pmUrl = Get-DownloadUrl -Version $Version -Binary "pm" -Arch $Arch
    $pmPath = Join-Path $binDir "pm.exe"
    Invoke-WebRequest -Uri $pmUrl -OutFile $pmPath
    Write-Success "Downloaded pm.exe"

    # Download pommeld.exe
    Write-Step "Downloading pommeld.exe..."
    $daemonUrl = Get-DownloadUrl -Version $Version -Binary "pommeld" -Arch $Arch
    $daemonPath = Join-Path $binDir "pommeld.exe"
    Invoke-WebRequest -Uri $daemonUrl -OutFile $daemonPath
    Write-Success "Downloaded pommeld.exe"

    return $binDir
}
```

### Task 6: Implement PATH Configuration

```powershell
function Add-ToPath {
    param(
        [string]$Directory
    )

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    # Check if already in PATH
    if ($currentPath -split ";" | Where-Object { $_ -eq $Directory }) {
        Write-Step "Already in PATH"
        return
    }

    Write-Step "Adding to PATH..."

    # Add to user PATH (doesn't require admin)
    $newPath = "$currentPath;$Directory"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

    # Also update current session
    $env:Path = "$env:Path;$Directory"

    Write-Success "Added to PATH (restart terminal for full effect)"
}
```

### Task 7: Implement Ollama Installation

```powershell
function Test-WingetAvailable {
    try {
        $null = Get-Command winget -ErrorAction Stop
        return $true
    }
    catch {
        return $false
    }
}

function Test-OllamaInstalled {
    try {
        $null = Get-Command ollama -ErrorAction Stop
        return $true
    }
    catch {
        return $false
    }
}

function Install-Ollama {
    Write-Step "Checking for Ollama..."

    if (Test-OllamaInstalled) {
        Write-Success "Ollama already installed"
        return $true
    }

    if (-not (Test-WingetAvailable)) {
        Write-Warning "winget not available. Please install Ollama manually:"
        Write-Host "  https://ollama.ai/download/windows"
        return $false
    }

    Write-Step "Installing Ollama via winget..."

    try {
        winget install --id Ollama.Ollama --silent --accept-package-agreements --accept-source-agreements
        Write-Success "Ollama installed"

        # Refresh PATH to find ollama
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        return $true
    }
    catch {
        Write-Warning "Failed to install Ollama: $_"
        Write-Host "  Please install manually: https://ollama.ai/download/windows"
        return $false
    }
}
```

### Task 8: Implement Model Pulling

```powershell
function Install-EmbeddingModel {
    Write-Step "Pulling embedding model (this may take a few minutes)..."

    try {
        # Ensure Ollama is running
        $ollamaProcess = Get-Process -Name "ollama" -ErrorAction SilentlyContinue
        if (-not $ollamaProcess) {
            Write-Step "Starting Ollama..."
            Start-Process "ollama" -ArgumentList "serve" -WindowStyle Hidden
            Start-Sleep -Seconds 3
        }

        # Pull the model
        & ollama pull $script:OllamaModel

        if ($LASTEXITCODE -eq 0) {
            Write-Success "Embedding model installed"
            return $true
        }
        else {
            throw "ollama pull failed with exit code $LASTEXITCODE"
        }
    }
    catch {
        Write-Warning "Failed to pull model: $_"
        Write-Host "  Run manually: ollama pull $script:OllamaModel"
        return $false
    }
}
```

### Task 9: Implement Verification

```powershell
function Test-Installation {
    Write-Step "Verifying installation..."

    try {
        $version = & pm version 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Pommel installed: $version"
            return $true
        }
        else {
            throw "pm version failed"
        }
    }
    catch {
        Write-Failure "Verification failed: $_"
        return $false
    }
}
```

### Task 10: Main Installation Flow

```powershell
function Main {
    Write-Host ""
    Write-Host "Pommel Installer for Windows" -ForegroundColor Cyan
    Write-Host "=============================" -ForegroundColor Cyan
    Write-Host ""

    try {
        # Detect architecture
        $arch = Get-Architecture
        Write-Step "Detected architecture: $arch"

        # Get latest version
        $version = Get-LatestRelease
        Write-Step "Latest version: $version"

        # Download binaries
        $binDir = Install-PommelBinaries -Version $version -Arch $arch -InstallDir $InstallDir

        # Add to PATH
        Add-ToPath -Directory $binDir

        # Install Ollama (unless skipped)
        if (-not $SkipOllama) {
            $ollamaOk = Install-Ollama

            # Pull model (unless skipped)
            if ($ollamaOk -and -not $SkipModel) {
                Install-EmbeddingModel | Out-Null
            }
        }

        # Verify
        Write-Host ""
        if (Test-Installation) {
            Write-Host ""
            Write-Host "Installation complete!" -ForegroundColor Green
            Write-Host ""
            Write-Host "Next steps:" -ForegroundColor Cyan
            Write-Host "  1. Open a new terminal (to refresh PATH)"
            Write-Host "  2. Navigate to your project directory"
            Write-Host "  3. Run: pm init --auto --claude --start"
            Write-Host ""
        }
        else {
            Write-Host ""
            Write-Warning "Installation may be incomplete. Please check errors above."
            exit 1
        }
    }
    catch {
        Write-Host ""
        Write-Failure "Installation failed: $_"
        exit 1
    }
}

# Run main
Main
```

### Task 11: Test Script Manually

**Manual testing checklist (on Windows machine):**

- [ ] Fresh install (no previous Pommel)
- [ ] Upgrade existing install
- [ ] Architecture detection (x64)
- [ ] Architecture detection (ARM64 if available)
- [ ] winget available - Ollama installs
- [ ] winget not available - graceful fallback
- [ ] Ollama already installed - skips
- [ ] Model already pulled - quick completion
- [ ] PATH addition works
- [ ] Verification passes
- [ ] `pm version` works after install
- [ ] `pm init` works in a project

### Task 12: Create Windows Testing Context Document

**File:** `docs/plans/windows_testing.md`

Create context document for Windows Claude instance to run manual tests.

## Test Cases

### Automated Tests (Limited)

PowerShell scripts are harder to unit test. Focus on:

| Test | Method |
|------|--------|
| Script syntax valid | `powershell -NoExecute -File install.ps1` |
| Functions defined | Dot-source and check |

### Manual Tests (Primary)

| Test | Expected Result |
|------|-----------------|
| Fresh install | Binaries downloaded, PATH set, Ollama installed, model pulled |
| Upgrade install | Binaries replaced, PATH unchanged |
| Skip Ollama flag | Ollama not installed, continues |
| Skip model flag | Model not pulled, continues |
| No winget | Warning message, continues without Ollama |
| x64 architecture | Downloads amd64 binaries |
| ARM64 architecture | Downloads arm64 binaries |
| Verification | `pm version` succeeds |

## Acceptance Criteria

- [ ] Script runs via `irm | iex`
- [ ] Downloads correct architecture binaries
- [ ] Installs to `%LOCALAPPDATA%\Pommel\bin`
- [ ] Adds to user PATH (no admin needed)
- [ ] Installs Ollama via winget (if available)
- [ ] Pulls embedding model
- [ ] Provides clear error messages
- [ ] `pm version` works after install
- [ ] Works on Windows 10 and Windows 11

## Files Created

| File | Purpose |
|------|---------|
| `scripts/install.ps1` | Windows installation script |
| `docs/plans/windows_testing.md` | Testing context for Windows |

## Notes

- Script requires PowerShell 5.1+ (included in Windows 10+)
- Uses user-level PATH to avoid admin requirements
- winget required for automatic Ollama install (Windows 10 1709+)
- Ollama must be running for model pull to work
- Consider adding checksum verification for security
- May want to add `-Force` flag to overwrite existing install
