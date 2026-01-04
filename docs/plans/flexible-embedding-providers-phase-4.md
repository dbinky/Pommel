# Phase 4: Install Scripts

**Parent Design:** [2025-01-04-flexible-embedding-providers-design.md](./2025-01-04-flexible-embedding-providers-design.md)

## Overview

This phase updates both installation scripts (bash and PowerShell) to support interactive provider selection, upgrade detection, and version display.

## Deliverables

1. `scripts/install.sh` - Updated bash installer
2. `scripts/install.ps1` - Updated PowerShell installer
3. `scripts/test_install.sh` - Bash installer tests
4. `scripts/test_install.ps1` - PowerShell installer tests

## Implementation Order (TDD)

### Note on Testing Shell Scripts

Shell scripts require different testing approaches than Go code:

1. **Function-level unit tests** - Extract functions and test in isolation
2. **Integration tests** - Run full script in controlled environment
3. **Mock external commands** - Stub out `curl`, `ollama`, API calls

---

### Step 1: Bash - Upgrade Detection and Version Display

**File:** `scripts/install.sh`

**Test file:** `scripts/test_install.sh`

**Tests to write first:**

```bash
#!/usr/bin/env bash
# test_install.sh - Unit tests for install.sh functions

set -e

# Source the functions (we'll refactor install.sh to allow sourcing)
source "$(dirname "$0")/install.sh" --source-only

# === Happy Path Tests ===

test_detect_existing_install_found() {
    echo "Test: detect_existing_install when pm exists"

    # Mock pm command
    pm() { echo "pm version 0.5.2"; }
    export -f pm

    detect_existing_install

    assert_equals "true" "$IS_UPGRADE"
    assert_equals "0.5.2" "$CURRENT_VERSION"
}

test_detect_existing_install_not_found() {
    echo "Test: detect_existing_install when pm not installed"

    # Ensure pm doesn't exist in PATH
    unset -f pm 2>/dev/null || true

    detect_existing_install

    assert_equals "false" "$IS_UPGRADE"
}

test_get_latest_version() {
    echo "Test: get_latest_version from GitHub"

    # Mock curl response
    curl() {
        echo '{"tag_name": "v0.6.0"}'
    }
    export -f curl

    VERSION=$(get_latest_version)

    assert_equals "v0.6.0" "$VERSION"
}

# === Failure Scenario Tests ===

test_detect_existing_install_pm_fails() {
    echo "Test: detect_existing_install when pm command fails"

    pm() { return 1; }
    export -f pm

    detect_existing_install

    assert_equals "false" "$IS_UPGRADE"
}

test_get_latest_version_network_error() {
    echo "Test: get_latest_version with network error"

    curl() { return 1; }
    export -f curl

    # Should exit with error or return empty
    if VERSION=$(get_latest_version 2>/dev/null); then
        assert_equals "" "$VERSION"
    fi
}

# === Edge Case Tests ===

test_detect_existing_install_unusual_version() {
    echo "Test: detect_existing_install with dev version"

    pm() { echo "pm version 0.6.0-dev+abc123"; }
    export -f pm

    detect_existing_install

    assert_equals "true" "$IS_UPGRADE"
    # Should extract base version
    assert_contains "$CURRENT_VERSION" "0.6.0"
}

# Helper functions
assert_equals() {
    if [[ "$1" != "$2" ]]; then
        echo "FAIL: Expected '$1' but got '$2'"
        exit 1
    fi
    echo "  PASS"
}

assert_contains() {
    if [[ "$1" != *"$2"* ]]; then
        echo "FAIL: Expected '$1' to contain '$2'"
        exit 1
    fi
    echo "  PASS"
}

# Run tests
run_tests() {
    test_detect_existing_install_found
    test_detect_existing_install_not_found
    test_get_latest_version
    test_detect_existing_install_pm_fails
    test_get_latest_version_network_error
    test_detect_existing_install_unusual_version

    echo ""
    echo "All tests passed!"
}

run_tests
```

**Implementation updates to `install.sh`:**

```bash
# Add near top of script
VERSION=""  # Will be set by get_latest_version

get_latest_version() {
    local api_url="https://api.github.com/repos/$REPO/releases/latest"
    local response

    response=$(curl -s "$api_url" 2>/dev/null) || {
        error "Failed to fetch latest version from GitHub"
    }

    VERSION=$(echo "$response" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)

    if [[ -z "$VERSION" ]]; then
        error "Could not determine latest version"
    fi

    echo "$VERSION"
}

detect_existing_install() {
    IS_UPGRADE=false
    CURRENT_VERSION=""

    if command -v pm &> /dev/null; then
        CURRENT_VERSION=$(pm version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1) || true
        if [[ -n "$CURRENT_VERSION" ]]; then
            IS_UPGRADE=true
            info "Previous install detected (v${CURRENT_VERSION}) - upgrading to ${VERSION}"
        fi
    fi
}
```

---

### Step 2: Bash - Provider Selection Prompt

**Tests to write first:**

```bash
# test_install.sh (continued)

# === Provider Selection Tests ===

test_select_provider_local_ollama() {
    echo "Test: select_provider with choice 1 (Local Ollama)"

    # Simulate user input
    echo "1" | select_provider

    assert_equals "ollama" "$SELECTED_PROVIDER"
}

test_select_provider_remote_ollama() {
    echo "Test: select_provider with choice 2 (Remote Ollama)"

    echo -e "2\nhttp://192.168.1.100:11434" | select_provider

    assert_equals "ollama-remote" "$SELECTED_PROVIDER"
    assert_equals "http://192.168.1.100:11434" "$OLLAMA_REMOTE_URL"
}

test_select_provider_openai() {
    echo "Test: select_provider with choice 3 (OpenAI)"

    # Mock API validation
    validate_openai_key() { return 0; }
    export -f validate_openai_key

    echo -e "3\nsk-test-key" | select_provider

    assert_equals "openai" "$SELECTED_PROVIDER"
    assert_equals "sk-test-key" "$OPENAI_API_KEY"
}

test_select_provider_openai_skip_key() {
    echo "Test: select_provider OpenAI with blank key"

    echo -e "3\n" | select_provider  # Empty key

    assert_equals "openai" "$SELECTED_PROVIDER"
    assert_equals "" "$OPENAI_API_KEY"
}

test_select_provider_voyage() {
    echo "Test: select_provider with choice 4 (Voyage)"

    validate_voyage_key() { return 0; }
    export -f validate_voyage_key

    echo -e "4\npa-test-key" | select_provider

    assert_equals "voyage" "$SELECTED_PROVIDER"
    assert_equals "pa-test-key" "$VOYAGE_API_KEY"
}

test_select_provider_default() {
    echo "Test: select_provider with empty input defaults to 1"

    echo "" | select_provider

    assert_equals "ollama" "$SELECTED_PROVIDER"
}

test_select_provider_invalid_then_valid() {
    echo "Test: select_provider with invalid then valid choice"

    echo -e "99\n1" | select_provider

    assert_equals "ollama" "$SELECTED_PROVIDER"
}

# === API Validation Tests ===

test_validate_openai_key_success() {
    echo "Test: validate_openai_key with valid key"

    # Mock successful API response
    curl() {
        echo '{"data": [{"embedding": []}]}'
    }
    export -f curl

    validate_openai_key "sk-valid-key"
    assert_equals 0 $?
}

test_validate_openai_key_invalid() {
    echo "Test: validate_openai_key with invalid key"

    curl() {
        echo '{"error": {"message": "Invalid API key"}}'
        return 0
    }
    export -f curl

    if validate_openai_key "sk-invalid"; then
        echo "FAIL: Should have returned error"
        exit 1
    fi
    echo "  PASS"
}

test_validate_voyage_key_success() {
    echo "Test: validate_voyage_key with valid key"

    curl() {
        echo '{"data": [{"embedding": []}]}'
    }
    export -f curl

    validate_voyage_key "pa-valid-key"
    assert_equals 0 $?
}
```

**Implementation:**

```bash
# Provider selection functions

select_provider() {
    echo ""
    echo "[2/4] How would you like to generate embeddings?"
    echo ""
    echo "  1) Local Ollama    - Free, runs on this machine (~300MB model)"
    echo "  2) Remote Ollama   - Free, connect to Ollama on another machine"
    echo "  3) OpenAI API      - Paid, no local setup required"
    echo "  4) Voyage AI       - Paid, optimized for code search"
    echo ""
    read -p "  Choice [1]: " choice
    choice=${choice:-1}

    case $choice in
        1) setup_local_ollama ;;
        2) setup_remote_ollama ;;
        3) setup_openai ;;
        4) setup_voyage ;;
        *)
            warn "Invalid choice. Please enter 1-4."
            select_provider  # Retry
            ;;
    esac
}

setup_local_ollama() {
    SELECTED_PROVIDER="ollama"
    info "Selected: Local Ollama"
}

setup_remote_ollama() {
    SELECTED_PROVIDER="ollama-remote"
    echo ""
    read -p "  Enter Ollama server URL (e.g., http://192.168.1.100:11434): " url

    if [[ -z "$url" ]]; then
        warn "URL is required for remote Ollama"
        setup_remote_ollama
        return
    fi

    OLLAMA_REMOTE_URL="$url"
    info "Selected: Remote Ollama at $url"
}

setup_openai() {
    SELECTED_PROVIDER="openai"
    echo ""
    read -p "  Enter your OpenAI API key (leave blank to configure later): " key

    if [[ -n "$key" ]]; then
        info "Validating API key..."
        if validate_openai_key "$key"; then
            OPENAI_API_KEY="$key"
            success "API key validated"
        else
            warn "Invalid API key. Run 'pm config provider' later to configure."
            OPENAI_API_KEY=""
        fi
    else
        OPENAI_API_KEY=""
        info "Skipped. Run 'pm config provider' to add your API key later."
    fi
}

setup_voyage() {
    SELECTED_PROVIDER="voyage"
    echo ""
    read -p "  Enter your Voyage AI API key (leave blank to configure later): " key

    if [[ -n "$key" ]]; then
        info "Validating API key..."
        if validate_voyage_key "$key"; then
            VOYAGE_API_KEY="$key"
            success "API key validated"
        else
            warn "Invalid API key. Run 'pm config provider' later to configure."
            VOYAGE_API_KEY=""
        fi
    else
        VOYAGE_API_KEY=""
        info "Skipped. Run 'pm config provider' to add your API key later."
    fi
}

validate_openai_key() {
    local key="$1"
    local response

    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $key" \
        -H "Content-Type: application/json" \
        -d '{"model": "text-embedding-3-small", "input": "test"}' \
        "https://api.openai.com/v1/embeddings" 2>/dev/null)

    local http_code=$(echo "$response" | tail -1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        return 0
    else
        return 1
    fi
}

validate_voyage_key() {
    local key="$1"
    local response

    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $key" \
        -H "Content-Type: application/json" \
        -d '{"model": "voyage-code-3", "input": ["test"]}' \
        "https://api.voyageai.com/v1/embeddings" 2>/dev/null)

    local http_code=$(echo "$response" | tail -1)

    if [[ "$http_code" == "200" ]]; then
        return 0
    else
        return 1
    fi
}
```

---

### Step 3: Bash - Write Global Config

**Tests to write first:**

```bash
# test_install.sh (continued)

test_write_global_config_ollama() {
    echo "Test: write_global_config for Ollama"

    TEMP_HOME=$(mktemp -d)
    export HOME="$TEMP_HOME"
    export XDG_CONFIG_HOME=""

    SELECTED_PROVIDER="ollama"
    write_global_config

    config_file="$TEMP_HOME/.config/pommel/config.yaml"
    assert_file_exists "$config_file"
    assert_file_contains "$config_file" "provider: ollama"

    rm -rf "$TEMP_HOME"
}

test_write_global_config_openai() {
    echo "Test: write_global_config for OpenAI with key"

    TEMP_HOME=$(mktemp -d)
    export HOME="$TEMP_HOME"

    SELECTED_PROVIDER="openai"
    OPENAI_API_KEY="sk-test-key"
    write_global_config

    config_file="$TEMP_HOME/.config/pommel/config.yaml"
    assert_file_exists "$config_file"
    assert_file_contains "$config_file" "provider: openai"
    assert_file_contains "$config_file" "api_key: sk-test-key"

    rm -rf "$TEMP_HOME"
}

test_write_global_config_creates_directory() {
    echo "Test: write_global_config creates ~/.config/pommel"

    TEMP_HOME=$(mktemp -d)
    export HOME="$TEMP_HOME"

    SELECTED_PROVIDER="ollama"
    write_global_config

    assert_dir_exists "$TEMP_HOME/.config/pommel"

    rm -rf "$TEMP_HOME"
}

test_write_global_config_xdg() {
    echo "Test: write_global_config respects XDG_CONFIG_HOME"

    TEMP_CONFIG=$(mktemp -d)
    export XDG_CONFIG_HOME="$TEMP_CONFIG"

    SELECTED_PROVIDER="voyage"
    VOYAGE_API_KEY="pa-test"
    write_global_config

    config_file="$TEMP_CONFIG/pommel/config.yaml"
    assert_file_exists "$config_file"

    rm -rf "$TEMP_CONFIG"
}

# Helper
assert_file_exists() {
    if [[ ! -f "$1" ]]; then
        echo "FAIL: File '$1' does not exist"
        exit 1
    fi
    echo "  PASS"
}

assert_file_contains() {
    if ! grep -q "$2" "$1"; then
        echo "FAIL: File '$1' does not contain '$2'"
        exit 1
    fi
    echo "  PASS"
}

assert_dir_exists() {
    if [[ ! -d "$1" ]]; then
        echo "FAIL: Directory '$1' does not exist"
        exit 1
    fi
    echo "  PASS"
}
```

**Implementation:**

```bash
write_global_config() {
    local config_dir
    if [[ -n "$XDG_CONFIG_HOME" ]]; then
        config_dir="$XDG_CONFIG_HOME/pommel"
    else
        config_dir="$HOME/.config/pommel"
    fi

    mkdir -p "$config_dir"

    local config_file="$config_dir/config.yaml"

    cat > "$config_file" << EOF
# Pommel global configuration
# Generated by install script

embedding:
  provider: $SELECTED_PROVIDER
EOF

    case $SELECTED_PROVIDER in
        ollama)
            cat >> "$config_file" << EOF
  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
EOF
            ;;
        ollama-remote)
            cat >> "$config_file" << EOF
  ollama-remote:
    url: "$OLLAMA_REMOTE_URL"
    model: "unclemusclez/jina-embeddings-v2-base-code"
EOF
            ;;
        openai)
            cat >> "$config_file" << EOF
  openai:
    api_key: "$OPENAI_API_KEY"
    model: "text-embedding-3-small"
EOF
            ;;
        voyage)
            cat >> "$config_file" << EOF
  voyage:
    api_key: "$VOYAGE_API_KEY"
    model: "voyage-code-3"
EOF
            ;;
    esac

    success "Configuration saved to $config_file"
}
```

---

### Step 4: Bash - Upgrade Flow

**Tests to write first:**

```bash
test_upgrade_skips_provider_selection() {
    echo "Test: upgrade with existing config skips provider selection"

    TEMP_HOME=$(mktemp -d)
    export HOME="$TEMP_HOME"

    # Create existing global config
    mkdir -p "$TEMP_HOME/.config/pommel"
    echo "embedding: { provider: openai }" > "$TEMP_HOME/.config/pommel/config.yaml"

    IS_UPGRADE=true
    CURRENT_VERSION="0.5.0"

    # Check if should skip
    should_skip=$(should_skip_provider_selection)

    assert_equals "true" "$should_skip"

    rm -rf "$TEMP_HOME"
}

test_upgrade_without_config_shows_selection() {
    echo "Test: upgrade without config shows provider selection"

    TEMP_HOME=$(mktemp -d)
    export HOME="$TEMP_HOME"

    # No existing config
    IS_UPGRADE=true

    should_skip=$(should_skip_provider_selection)

    assert_equals "false" "$should_skip"

    rm -rf "$TEMP_HOME"
}

test_fresh_install_shows_selection() {
    echo "Test: fresh install shows provider selection"

    IS_UPGRADE=false

    should_skip=$(should_skip_provider_selection)

    assert_equals "false" "$should_skip"
}
```

**Implementation:**

```bash
should_skip_provider_selection() {
    # Skip if upgrading and global config already exists
    if [[ "$IS_UPGRADE" == "true" ]]; then
        local config_dir
        if [[ -n "$XDG_CONFIG_HOME" ]]; then
            config_dir="$XDG_CONFIG_HOME/pommel"
        else
            config_dir="$HOME/.config/pommel"
        fi

        if [[ -f "$config_dir/config.yaml" ]]; then
            # Check if provider is configured
            if grep -q "provider:" "$config_dir/config.yaml"; then
                echo "true"
                return
            fi
        fi
    fi

    echo "false"
}
```

---

### Step 5: PowerShell - Equivalent Updates

**File:** `scripts/install.ps1`

**Test file:** `scripts/test_install.ps1`

The PowerShell script needs equivalent updates:

```powershell
# test_install.ps1

Describe "Install Script Tests" {
    BeforeAll {
        . "$PSScriptRoot/install.ps1" -SourceOnly
    }

    Context "Version Detection" {
        It "Detects existing installation" {
            Mock Get-Command { return @{ Source = "pm.exe" } }
            Mock Invoke-Expression { return "pm version 0.5.2" }

            $result = Test-ExistingInstall

            $result.IsUpgrade | Should -Be $true
            $result.CurrentVersion | Should -Be "0.5.2"
        }

        It "Returns false when not installed" {
            Mock Get-Command { throw "Not found" }

            $result = Test-ExistingInstall

            $result.IsUpgrade | Should -Be $false
        }
    }

    Context "Provider Selection" {
        It "Selects Local Ollama with choice 1" {
            Mock Read-Host { return "1" }

            $result = Select-Provider

            $result.Provider | Should -Be "ollama"
        }

        It "Selects OpenAI and validates key" {
            Mock Read-Host { return "sk-test-key" } -ParameterFilter { $Prompt -like "*API key*" }
            Mock Read-Host { return "3" } -ParameterFilter { $Prompt -like "*Choice*" }
            Mock Test-OpenAIKey { return $true }

            $result = Select-Provider

            $result.Provider | Should -Be "openai"
            $result.APIKey | Should -Be "sk-test-key"
        }

        It "Handles invalid API key gracefully" {
            Mock Read-Host { return "invalid-key" } -ParameterFilter { $Prompt -like "*API key*" }
            Mock Read-Host { return "3" } -ParameterFilter { $Prompt -like "*Choice*" }
            Mock Test-OpenAIKey { return $false }

            $result = Select-Provider

            $result.Provider | Should -Be "openai"
            $result.APIKey | Should -BeNullOrEmpty
        }
    }

    Context "Global Config" {
        It "Creates config directory if needed" {
            $tempDir = New-TemporaryDirectory
            $env:APPDATA = $tempDir

            Write-GlobalConfig -Provider "ollama"

            "$tempDir\pommel\config.yaml" | Should -Exist
        }

        It "Writes correct YAML for OpenAI" {
            $tempDir = New-TemporaryDirectory
            $env:APPDATA = $tempDir

            Write-GlobalConfig -Provider "openai" -APIKey "sk-test"

            $content = Get-Content "$tempDir\pommel\config.yaml" -Raw
            $content | Should -Match "provider: openai"
            $content | Should -Match "api_key: sk-test"
        }
    }

    Context "Upgrade Flow" {
        It "Skips provider selection when config exists" {
            $tempDir = New-TemporaryDirectory
            $env:APPDATA = $tempDir
            New-Item -Path "$tempDir\pommel" -ItemType Directory -Force
            "embedding: { provider: openai }" | Out-File "$tempDir\pommel\config.yaml"

            $script:IsUpgrade = $true

            $result = Test-ShouldSkipProviderSelection

            $result | Should -Be $true
        }
    }
}
```

**Implementation updates to `install.ps1`:**

```powershell
function Test-ExistingInstall {
    $result = @{
        IsUpgrade = $false
        CurrentVersion = ""
    }

    try {
        $pmCmd = Get-Command pm -ErrorAction Stop
        $versionOutput = & pm version 2>&1
        if ($versionOutput -match '(\d+\.\d+\.\d+)') {
            $result.IsUpgrade = $true
            $result.CurrentVersion = $Matches[1]
            Write-Step "Previous install detected (v$($result.CurrentVersion)) - upgrading to v$Version"
        }
    }
    catch {
        # Not installed
    }

    return $result
}

function Select-Provider {
    Write-Host ""
    Write-Host "[2/4] How would you like to generate embeddings?" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  1) Local Ollama    - Free, runs on this machine (~300MB model)"
    Write-Host "  2) Remote Ollama   - Free, connect to Ollama on another machine"
    Write-Host "  3) OpenAI API      - Paid, no local setup required"
    Write-Host "  4) Voyage AI       - Paid, optimized for code search"
    Write-Host ""

    $choice = Read-Host "  Choice [1]"
    if ([string]::IsNullOrEmpty($choice)) { $choice = "1" }

    $result = @{
        Provider = ""
        URL = ""
        APIKey = ""
    }

    switch ($choice) {
        "1" {
            $result.Provider = "ollama"
            Write-Step "Selected: Local Ollama"
        }
        "2" {
            $result.Provider = "ollama-remote"
            $url = Read-Host "  Enter Ollama server URL"
            $result.URL = $url
        }
        "3" {
            $result.Provider = "openai"
            $key = Read-Host "  Enter your OpenAI API key (leave blank to configure later)"
            if (-not [string]::IsNullOrEmpty($key)) {
                if (Test-OpenAIKey $key) {
                    $result.APIKey = $key
                    Write-Success "API key validated"
                } else {
                    Write-Warn "Invalid API key. Run 'pm config provider' later."
                }
            }
        }
        "4" {
            $result.Provider = "voyage"
            $key = Read-Host "  Enter your Voyage AI API key (leave blank to configure later)"
            if (-not [string]::IsNullOrEmpty($key)) {
                if (Test-VoyageKey $key) {
                    $result.APIKey = $key
                    Write-Success "API key validated"
                } else {
                    Write-Warn "Invalid API key. Run 'pm config provider' later."
                }
            }
        }
        default {
            Write-Warn "Invalid choice. Please enter 1-4."
            return Select-Provider
        }
    }

    return $result
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

function Write-GlobalConfig {
    param(
        [string]$Provider,
        [string]$URL = "",
        [string]$APIKey = ""
    )

    $configDir = Join-Path $env:APPDATA "pommel"
    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }

    $configPath = Join-Path $configDir "config.yaml"

    $yaml = @"
# Pommel global configuration
# Generated by install script

embedding:
  provider: $Provider
"@

    switch ($Provider) {
        "ollama" {
            $yaml += @"

  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
"@
        }
        "ollama-remote" {
            $yaml += @"

  ollama-remote:
    url: "$URL"
    model: "unclemusclez/jina-embeddings-v2-base-code"
"@
        }
        "openai" {
            $yaml += @"

  openai:
    api_key: "$APIKey"
    model: "text-embedding-3-small"
"@
        }
        "voyage" {
            $yaml += @"

  voyage:
    api_key: "$APIKey"
    model: "voyage-code-3"
"@
        }
    }

    $yaml | Out-File -FilePath $configPath -Encoding utf8

    Write-Success "Configuration saved to $configPath"
}
```

---

## Acceptance Criteria

- [ ] Bash script shows version being installed
- [ ] Bash script detects existing installation and shows upgrade message
- [ ] Bash script provider selection works for all 4 providers
- [ ] Bash script validates API keys before saving
- [ ] Bash script skips provider selection on upgrade if config exists
- [ ] PowerShell script has identical functionality
- [ ] Both scripts handle network errors gracefully
- [ ] Both scripts allow skipping API key entry
- [ ] Global config is written in correct YAML format
- [ ] Scripts work on fresh install and upgrade scenarios

## Dependencies

- Phase 2 (global config schema)
- Phase 3 (CLI commands for validation)

## Estimated Test Count

- Bash upgrade detection: ~5 tests
- Bash provider selection: ~8 tests
- Bash API validation: ~4 tests
- Bash config writing: ~5 tests
- Bash upgrade flow: ~3 tests
- PowerShell equivalents: ~25 tests

**Total: ~50 tests**
