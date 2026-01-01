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

function Get-DownloadUrl {
    param(
        [string]$Version,
        [string]$Binary,
        [string]$Arch
    )

    $fileName = "${Binary}-windows-${Arch}.exe"
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

    # Download pm.exe
    Write-Step "Downloading pm.exe..."
    $pmUrl = Get-DownloadUrl -Version $Version -Binary "pm" -Arch $Arch
    $pmPath = Join-Path $binDir "pm.exe"
    try {
        Invoke-WebRequest -Uri $pmUrl -OutFile $pmPath -UseBasicParsing
        Write-Success "Downloaded pm.exe"
    }
    catch {
        throw "Failed to download pm.exe from $pmUrl : $_"
    }

    # Download pommeld.exe
    Write-Step "Downloading pommeld.exe..."
    $daemonUrl = Get-DownloadUrl -Version $Version -Binary "pommeld" -Arch $Arch
    $daemonPath = Join-Path $binDir "pommeld.exe"
    try {
        Invoke-WebRequest -Uri $daemonUrl -OutFile $daemonPath -UseBasicParsing
        Write-Success "Downloaded pommeld.exe"
    }
    catch {
        throw "Failed to download pommeld.exe from $daemonUrl : $_"
    }

    return $binDir
}
#endregion

#region Language Configuration Installation
function Install-LanguageConfigs {
    param(
        [string]$InstallPath
    )

    $languagesDir = Join-Path $InstallPath "languages"
    $baseUrl = "https://raw.githubusercontent.com/dbinky/Pommel/main/languages"

    $languageFiles = @(
        "csharp.yaml",
        "dart.yaml",
        "elixir.yaml",
        "go.yaml",
        "java.yaml",
        "javascript.yaml",
        "kotlin.yaml",
        "php.yaml",
        "python.yaml",
        "rust.yaml",
        "solidity.yaml",
        "swift.yaml",
        "typescript.yaml"
    )

    Write-Step "Installing language configuration files..."

    # Create languages directory if needed
    if (-not (Test-Path $languagesDir)) {
        try {
            New-Item -ItemType Directory -Path $languagesDir -Force | Out-Null
            Write-Success "Created languages directory: $languagesDir"
        }
        catch {
            Write-Failure "Failed to create languages directory: $_"
            return
        }
    }

    $successCount = 0
    $failCount = 0

    foreach ($file in $languageFiles) {
        $url = "$baseUrl/$file"
        $destPath = Join-Path $languagesDir $file

        try {
            Invoke-WebRequest -Uri $url -OutFile $destPath -UseBasicParsing
            Write-Success "Downloaded $file"
            $successCount++
        }
        catch {
            Write-Warn "Failed to download $file : $_"
            $failCount++
        }
    }

    Write-Host ""
    if ($failCount -eq 0) {
        Write-Success "All $successCount language configs installed"
    }
    else {
        Write-Warn "Installed $successCount language configs, $failCount failed"
    }
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

        # Install language configuration files
        Install-LanguageConfigs -InstallPath $InstallDir

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
