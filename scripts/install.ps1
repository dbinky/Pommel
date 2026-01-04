#Requires -Version 5.1

<#
.SYNOPSIS
    Installs Pommel semantic code search tool on Windows.

.DESCRIPTION
    Downloads and installs pm.exe and pommeld.exe from GitHub releases,
    configures the embedding provider, installs language configs,
    and optionally sets up Ollama with the embedding model.

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
    [switch]$SourceOnly,
    [string]$InstallDir = "$env:LOCALAPPDATA\Pommel"
)

# Source-only mode for testing
if ($SourceOnly) {
    return
}

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"  # Speed up downloads

# Configuration
$script:Repo = "dbinky/Pommel"
$script:OllamaModel = "unclemusclez/jina-embeddings-v2-base-code"

# Global state
$script:SelectedProvider = ""
$script:OllamaRemoteUrl = ""
$script:OpenAIApiKey = ""
$script:VoyageApiKey = ""
$script:IsUpgrade = $false
$script:CurrentVersion = ""

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

#region Version and Upgrade Detection
function Test-ExistingInstall {
    $script:IsUpgrade = $false
    $script:CurrentVersion = ""

    try {
        $pmCmd = Get-Command pm -ErrorAction Stop
        $versionOutput = & pm version 2>&1
        if ($versionOutput -match '(\d+\.\d+\.\d+)') {
            $script:IsUpgrade = $true
            $script:CurrentVersion = $Matches[1]
        }
    }
    catch {
        # Not installed
    }
}

function Test-ExistingProviderConfig {
    $configPath = Join-Path $env:APPDATA "pommel\config.yaml"
    if (Test-Path $configPath) {
        $content = Get-Content $configPath -Raw
        if ($content -match 'provider:') {
            return $true
        }
    }
    return $false
}
#endregion

#region Architecture Detection
function Get-Architecture {
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

#region Provider Selection
function Select-Provider {
    Write-Host ""
    Write-Host "[2/5] Configure embedding provider" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  How would you like to generate embeddings?"
    Write-Host ""
    Write-Host "  1) Local Ollama    - Free, runs on this machine (~300MB model)"
    Write-Host "  2) Remote Ollama   - Free, connect to Ollama on another machine"
    Write-Host "  3) OpenAI API      - Paid, no local setup required"
    Write-Host "  4) Voyage AI       - Paid, optimized for code search"
    Write-Host ""

    $choice = Read-Host "  Choice [1]"
    if ([string]::IsNullOrEmpty($choice)) { $choice = "1" }

    switch ($choice) {
        "1" { Setup-LocalOllama }
        "2" { Setup-RemoteOllama }
        "3" { Setup-OpenAI }
        "4" { Setup-Voyage }
        default {
            Write-Warn "Invalid choice. Please enter 1-4."
            Select-Provider
        }
    }
}

function Setup-LocalOllama {
    $script:SelectedProvider = "ollama"
    Write-Success "Selected: Local Ollama"
}

function Setup-RemoteOllama {
    $script:SelectedProvider = "ollama-remote"
    Write-Host ""
    $url = Read-Host "  Enter Ollama server URL (e.g., http://192.168.1.100:11434)"

    if ([string]::IsNullOrEmpty($url)) {
        Write-Warn "URL is required for remote Ollama"
        Setup-RemoteOllama
        return
    }

    $script:OllamaRemoteUrl = $url
    Write-Success "Selected: Remote Ollama at $url"
}

function Setup-OpenAI {
    $script:SelectedProvider = "openai"
    Write-Host ""
    $key = Read-Host "  Enter your OpenAI API key (leave blank to configure later)"

    if (-not [string]::IsNullOrEmpty($key)) {
        Write-Step "Validating API key..."
        if (Test-OpenAIKey $key) {
            $script:OpenAIApiKey = $key
            Write-Success "API key validated"
        }
        else {
            Write-Warn "Invalid API key. Run 'pm config provider' later to configure."
            $script:OpenAIApiKey = ""
        }
    }
    else {
        $script:OpenAIApiKey = ""
        Write-Step "Skipped. Run 'pm config provider' to add your API key later."
    }
}

function Setup-Voyage {
    $script:SelectedProvider = "voyage"
    Write-Host ""
    $key = Read-Host "  Enter your Voyage AI API key (leave blank to configure later)"

    if (-not [string]::IsNullOrEmpty($key)) {
        Write-Step "Validating API key..."
        if (Test-VoyageKey $key) {
            $script:VoyageApiKey = $key
            Write-Success "API key validated"
        }
        else {
            Write-Warn "Invalid API key. Run 'pm config provider' later to configure."
            $script:VoyageApiKey = ""
        }
    }
    else {
        $script:VoyageApiKey = ""
        Write-Step "Skipped. Run 'pm config provider' to add your API key later."
    }
}

function Test-OpenAIKey {
    param([string]$Key)

    try {
        $body = @{
            model = "text-embedding-3-small"
            input = "test"
        } | ConvertTo-Json

        $response = Invoke-RestMethod -Uri "https://api.openai.com/v1/embeddings" `
            -Method Post `
            -Headers @{ "Authorization" = "Bearer $Key"; "Content-Type" = "application/json" } `
            -Body $body `
            -ErrorAction Stop

        return $true
    }
    catch {
        return $false
    }
}

function Test-VoyageKey {
    param([string]$Key)

    try {
        $body = @{
            model = "voyage-code-3"
            input = @("test")
        } | ConvertTo-Json

        $response = Invoke-RestMethod -Uri "https://api.voyageai.com/v1/embeddings" `
            -Method Post `
            -Headers @{ "Authorization" = "Bearer $Key"; "Content-Type" = "application/json" } `
            -Body $body `
            -ErrorAction Stop

        return $true
    }
    catch {
        return $false
    }
}

function Write-GlobalConfig {
    $configDir = Join-Path $env:APPDATA "pommel"
    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }

    $configPath = Join-Path $configDir "config.yaml"

    $yaml = @"
# Pommel global configuration
# Generated by install script

embedding:
  provider: $($script:SelectedProvider)
"@

    switch ($script:SelectedProvider) {
        "ollama" {
            $yaml += @"

  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
"@
        }
        "ollama-remote" {
            $yaml += @"

  ollama:
    url: "$($script:OllamaRemoteUrl)"
    model: "unclemusclez/jina-embeddings-v2-base-code"
"@
        }
        "openai" {
            if (-not [string]::IsNullOrEmpty($script:OpenAIApiKey)) {
                $yaml += @"

  openai:
    api_key: "$($script:OpenAIApiKey)"
    model: "text-embedding-3-small"
"@
            }
            else {
                $yaml += @"

  openai:
    # api_key: "" # Set via OPENAI_API_KEY environment variable or run 'pm config provider'
    model: "text-embedding-3-small"
"@
            }
        }
        "voyage" {
            if (-not [string]::IsNullOrEmpty($script:VoyageApiKey)) {
                $yaml += @"

  voyage:
    api_key: "$($script:VoyageApiKey)"
    model: "voyage-code-3"
"@
            }
            else {
                $yaml += @"

  voyage:
    # api_key: "" # Set via VOYAGE_API_KEY environment variable or run 'pm config provider'
    model: "voyage-code-3"
"@
            }
        }
    }

    $yaml | Out-File -FilePath $configPath -Encoding utf8

    Write-Success "Configuration saved to $configPath"
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

#region Language Configuration Installation
function Get-LanguageFileList {
    $apiUrl = "https://api.github.com/repos/$script:Repo/contents/languages"

    try {
        $response = Invoke-RestMethod -Uri $apiUrl -Headers @{
            "Accept" = "application/vnd.github.v3+json"
            "User-Agent" = "Pommel-Installer"
        }

        # Filter for .yaml files only
        $yamlFiles = $response | Where-Object { $_.name -match '\.yaml$' } | ForEach-Object { $_.name }
        return $yamlFiles
    }
    catch {
        Write-Warn "Failed to discover language files from API: $_"
        return @()
    }
}

function Install-LanguageConfigs {
    param(
        [string]$InstallPath
    )

    $languagesDir = Join-Path $InstallPath "languages"
    $baseUrl = "https://raw.githubusercontent.com/$script:Repo/main/languages"

    Write-Step "Discovering language configuration files..."

    # Get list of language files dynamically from GitHub API
    $languageFiles = Get-LanguageFileList

    if ($languageFiles.Count -eq 0) {
        Write-Warn "No language configuration files found"
        return
    }

    Write-Step "Found $($languageFiles.Count) language configuration files"

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

        # Check if Ollama API is already accessible (avoids launching desktop app)
        $ollamaRunning = $false
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:11434/api/tags" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                $ollamaRunning = $true
            }
        }
        catch {
            # API not accessible, need to start Ollama
        }

        if (-not $ollamaRunning) {
            Write-Step "Starting Ollama service..."
            # Start ollama serve as a background job to avoid launching the desktop GUI
            $ollamaExe = if ($ollamaCmd -is [System.Management.Automation.CommandInfo]) { $ollamaCmd.Source } else { $ollamaCmd }
            Start-Process -FilePath $ollamaExe -ArgumentList "serve" -WindowStyle Hidden -RedirectStandardOutput "NUL" -RedirectStandardError "NUL"

            # Wait for API to become available
            $attempts = 0
            $maxAttempts = 10
            while ($attempts -lt $maxAttempts) {
                Start-Sleep -Seconds 1
                try {
                    $response = Invoke-WebRequest -Uri "http://localhost:11434/api/tags" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
                    if ($response.StatusCode -eq 200) {
                        break
                    }
                }
                catch {
                    $attempts++
                }
            }

            if ($attempts -eq $maxAttempts) {
                Write-Warn "Ollama may not have started correctly. If a desktop app appeared, you can minimize it."
            }
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
        # Check for existing install
        Test-ExistingInstall

        # Detect architecture
        $arch = Get-Architecture
        Write-Step "Detected architecture: $arch"

        # Get latest version
        $version = Get-LatestRelease

        if ($script:IsUpgrade) {
            Write-Step "Previous install detected (v$($script:CurrentVersion)) - upgrading to $version"
        }
        else {
            Write-Step "Installing Pommel $version"
        }

        Write-Host ""
        Write-Host "[1/5] Checking dependencies..." -ForegroundColor Cyan
        Write-Host ""

        # Provider selection (skip on upgrade if config exists)
        if ($script:IsUpgrade -and (Test-ExistingProviderConfig)) {
            Write-Step "Using existing provider configuration"
        }
        else {
            Select-Provider
            Write-GlobalConfig
        }

        # Download binaries
        $binDir = Install-PommelBinaries -Version $version -Arch $arch -InstallPath $InstallDir

        # Install language configuration files
        Install-LanguageConfigs -InstallPath $InstallDir

        # Add to PATH
        Add-ToPath -Directory $binDir

        # Install Ollama (only if local Ollama selected and not skipped)
        $ollamaOk = $false
        if ($script:SelectedProvider -eq "ollama" -and -not $SkipOllama) {
            $ollamaOk = Install-Ollama

            # Pull model (unless skipped)
            if ($ollamaOk -and -not $SkipModel) {
                Install-EmbeddingModel | Out-Null
            }
        }
        elseif ($script:SelectedProvider -eq "ollama" -and $SkipOllama) {
            Write-Step "Skipping Ollama installation (use -SkipOllama:`$false to install)"
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
            Write-Host "Change provider later:"
            Write-Host "  pm config provider"
            Write-Host ""
            Write-Host "Installed locations:" -ForegroundColor Cyan
            Write-Host "  Binaries:       $binDir"
            Write-Host "  Global config:  $env:APPDATA\pommel\config.yaml"
            Write-Host "  Languages:      $InstallDir\languages"
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
