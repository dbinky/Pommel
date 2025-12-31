#Requires -Version 5.1

<#
.SYNOPSIS
    Installs Pommel semantic code search tool on Windows.

.DESCRIPTION
    Downloads and installs pm.exe and pommeld.exe from GitHub releases,
    configures PATH, installs Ollama if needed, and pulls the embedding model.

.PARAMETER SkipOllama
    Skip Ollama installation.

.PARAMETER SkipModel
    Skip embedding model download.

.PARAMETER InstallDir
    Custom installation directory. Defaults to %LOCALAPPDATA%\Pommel.

.EXAMPLE
    irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -SkipOllama

.NOTES
    Requires: PowerShell 5.1+, Windows 10 or later
    Ollama installation requires winget (Windows Package Manager)
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

#region Output Functions
function Write-Step {
    param([string]$Message)
    Write-Host "=> $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Failure {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}
#endregion

#region Architecture Detection
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
#endregion

#region GitHub Release Functions
function Get-LatestRelease {
    <#
    .SYNOPSIS
        Gets the latest release version from GitHub.
    #>
    $apiUrl = "https://api.github.com/repos/$script:Repo/releases/latest"

    try {
        $release = Invoke-RestMethod -Uri $apiUrl -Headers @{
            "Accept" = "application/vnd.github.v3+json"
            "User-Agent" = "Pommel-Installer"
        }
        return $release.tag_name
    }
    catch {
        throw "Failed to get latest release: $_"
    }
}

function Get-ArchiveUrl {
    param(
        [string]$Version,
        [string]$Arch
    )

    $fileName = "pommel-${Version}-windows-${Arch}.zip"
    return "https://github.com/$script:Repo/releases/download/$Version/$fileName"
}
#endregion

#region Binary Installation
function Install-PommelBinaries {
    param(
        [string]$Version,
        [string]$Arch,
        [string]$InstallPath
    )

    $binDir = Join-Path $InstallPath "bin"

    # Create directory if needed
    if (-not (Test-Path $binDir)) {
        New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    }

    # Download archive
    Write-Step "Downloading Pommel archive..."
    $archiveUrl = Get-ArchiveUrl -Version $Version -Arch $Arch
    $tempZip = Join-Path $env:TEMP "pommel-$Version-windows-$Arch.zip"
    try {
        Invoke-WebRequest -Uri $archiveUrl -OutFile $tempZip -UseBasicParsing
        Write-Success "Downloaded archive"
    }
    catch {
        throw "Failed to download archive from $archiveUrl : $_"
    }

    # Extract binaries
    Write-Step "Extracting binaries..."
    try {
        $tempExtract = Join-Path $env:TEMP "pommel-extract-$([System.Guid]::NewGuid().ToString('N'))"
        Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force

        # Find and copy the binaries (they have platform suffix in the archive)
        $pmSource = Join-Path $tempExtract "pm-windows-$Arch.exe"
        $daemonSource = Join-Path $tempExtract "pommeld-windows-$Arch.exe"

        if (-not (Test-Path $pmSource)) {
            throw "pm-windows-$Arch.exe not found in archive"
        }
        if (-not (Test-Path $daemonSource)) {
            throw "pommeld-windows-$Arch.exe not found in archive"
        }

        # Copy to bin directory with simple names
        Copy-Item $pmSource (Join-Path $binDir "pm.exe") -Force
        Copy-Item $daemonSource (Join-Path $binDir "pommeld.exe") -Force

        Write-Success "Extracted pm.exe and pommeld.exe"

        # Cleanup temp files
        Remove-Item $tempZip -Force -ErrorAction SilentlyContinue
        Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
    }
    catch {
        # Cleanup on failure
        Remove-Item $tempZip -Force -ErrorAction SilentlyContinue
        throw "Failed to extract binaries: $_"
    }

    return $binDir
}
#endregion

#region PATH Configuration
function Add-ToPath {
    param(
        [string]$Directory
    )

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    # Check if already in PATH
    $pathEntries = $currentPath -split ";" | Where-Object { $_ -ne "" }
    if ($pathEntries -contains $Directory) {
        Write-Step "Already in PATH"
        return
    }

    Write-Step "Adding to PATH..."

    # Add to user PATH (doesn't require admin)
    $newPath = if ($currentPath) { "$currentPath;$Directory" } else { $Directory }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")

    # Also update current session
    $env:Path = "$env:Path;$Directory"

    Write-Success "Added to PATH (restart terminal for full effect)"
}
#endregion

#region Ollama Installation
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
        # Also check common install locations
        $ollamaPath = "$env:LOCALAPPDATA\Programs\Ollama\ollama.exe"
        if (Test-Path $ollamaPath) {
            return $true
        }
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
        Write-Warn "winget not available. Please install Ollama manually:"
        Write-Host "  https://ollama.ai/download/windows"
        return $false
    }

    Write-Step "Installing Ollama via winget..."

    try {
        $output = winget install --id Ollama.Ollama --silent --accept-package-agreements --accept-source-agreements 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Ollama installed"

            # Refresh PATH to find ollama
            $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
            $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
            $env:Path = "$machinePath;$userPath"

            return $true
        }
        else {
            Write-Warn "winget returned exit code $LASTEXITCODE"
            Write-Host "  Please install manually: https://ollama.ai/download/windows"
            return $false
        }
    }
    catch {
        Write-Warn "Failed to install Ollama: $_"
        Write-Host "  Please install manually: https://ollama.ai/download/windows"
        return $false
    }
}
#endregion

#region Model Installation
function Install-EmbeddingModel {
    Write-Step "Pulling embedding model (this may take a few minutes)..."

    try {
        # Find ollama executable
        $ollamaCmd = Get-Command ollama -ErrorAction SilentlyContinue
        if (-not $ollamaCmd) {
            $ollamaPath = "$env:LOCALAPPDATA\Programs\Ollama\ollama.exe"
            if (Test-Path $ollamaPath) {
                $ollamaCmd = $ollamaPath
            }
            else {
                Write-Warn "Cannot find ollama executable"
                Write-Host "  Run manually: ollama pull $script:OllamaModel"
                return $false
            }
        }

        # Check if Ollama is running
        $ollamaProcess = Get-Process -Name "ollama" -ErrorAction SilentlyContinue
        if (-not $ollamaProcess) {
            Write-Step "Starting Ollama..."
            if ($ollamaCmd -is [System.Management.Automation.CommandInfo]) {
                Start-Process $ollamaCmd.Source -ArgumentList "serve" -WindowStyle Hidden
            }
            else {
                Start-Process $ollamaCmd -ArgumentList "serve" -WindowStyle Hidden
            }
            Start-Sleep -Seconds 5
        }

        # Pull the model
        Write-Host ""
        if ($ollamaCmd -is [System.Management.Automation.CommandInfo]) {
            & $ollamaCmd.Source pull $script:OllamaModel
        }
        else {
            & $ollamaCmd pull $script:OllamaModel
        }

        if ($LASTEXITCODE -eq 0) {
            Write-Success "Embedding model installed"
            return $true
        }
        else {
            throw "ollama pull failed with exit code $LASTEXITCODE"
        }
    }
    catch {
        Write-Warn "Failed to pull model: $_"
        Write-Host "  Run manually: ollama pull $script:OllamaModel"
        return $false
    }
}
#endregion

#region Verification
function Test-Installation {
    param([string]$BinDir)

    Write-Step "Verifying installation..."

    $pmPath = Join-Path $BinDir "pm.exe"

    try {
        $version = & $pmPath version 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Pommel installed: $version"
            return $true
        }
        else {
            throw "pm version failed with exit code $LASTEXITCODE"
        }
    }
    catch {
        Write-Failure "Verification failed: $_"
        return $false
    }
}
#endregion

#region Main
function Main {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host "                    Pommel Installer for Windows                " -ForegroundColor Cyan
    Write-Host "              Semantic Code Search for AI Agents                " -ForegroundColor Cyan
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host ""

    try {
        # Detect architecture
        $arch = Get-Architecture
        Write-Step "Detected architecture: $arch"

        # Get latest version
        $version = Get-LatestRelease
        Write-Step "Latest version: $version"

        # Download binaries
        $binDir = Install-PommelBinaries -Version $version -Arch $arch -InstallPath $InstallDir

        # Add to PATH
        Add-ToPath -Directory $binDir

        # Install Ollama (unless skipped)
        $ollamaOk = $false
        if (-not $SkipOllama) {
            $ollamaOk = Install-Ollama

            # Pull model (unless skipped)
            if ($ollamaOk -and -not $SkipModel) {
                Install-EmbeddingModel | Out-Null
            }
        }
        else {
            Write-Step "Skipping Ollama installation (use -SkipOllama:$false to install)"
        }

        # Verify
        Write-Host ""
        if (Test-Installation -BinDir $binDir) {
            Write-Host ""
            Write-Host "================================================================" -ForegroundColor Green
            Write-Host "                   Installation Complete!                       " -ForegroundColor Green
            Write-Host "================================================================" -ForegroundColor Green
            Write-Host ""
            Write-Host "Next steps:" -ForegroundColor Cyan
            Write-Host "  1. Open a new terminal (to refresh PATH)"
            Write-Host "  2. Navigate to your project directory"
            Write-Host "  3. Run: pm init"
            Write-Host "  4. Start the daemon: pm start"
            Write-Host "  5. Search your code: pm search ""your query"""
            Write-Host ""
            Write-Host "For AI agents (Claude Code, etc.):"
            Write-Host "  pm search ""authentication middleware"" --json"
            Write-Host ""
        }
        else {
            Write-Host ""
            Write-Warn "Installation may be incomplete. Please check errors above."
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
#endregion
