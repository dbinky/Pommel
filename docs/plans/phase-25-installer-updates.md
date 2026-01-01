# Phase 25: Installer Updates and Cleanup

**Parent Design:** [2026-01-01-config-driven-languages-design.md](2026-01-01-config-driven-languages-design.md)
**Depends On:** Phase 24 (Config Infrastructure)
**Version Target:** v0.4.0
**Estimated Complexity:** Medium

## Overview

Update installation scripts to deploy language configuration files, update documentation, and perform final cleanup. This phase completes the config-driven language support feature.

## Prerequisites

- Phase 24 complete and merged
- All 13 language YAML configs validated
- Generic chunker working correctly

## TDD Approach

**CRITICAL: All implementation follows strict TDD:**
1. Write failing test first
2. Verify test fails for the right reason
3. Write minimal code to pass
4. Refactor if needed
5. Repeat

## Tasks

### 25.1: Installer Script Updates (Unix)

Update `scripts/install.sh` to install language configuration files.

**File:** `scripts/install.sh`

#### 25.1.1: Write Tests First

**File:** `scripts/install_test.sh` (bash test script)

```bash
#!/usr/bin/env bash
# Installer test suite

# Test Setup
setup_test_env() {
    TEST_DIR=$(mktemp -d)
    export HOME="$TEST_DIR/home"
    export XDG_DATA_HOME="$HOME/.local/share"
    mkdir -p "$HOME"
}

teardown_test_env() {
    rm -rf "$TEST_DIR"
}

# Happy Path Tests
test_install_creates_languages_dir() {
    # Languages directory is created at ~/.local/share/pommel/languages
    setup_test_env
    # Run install function (source and call)
    source scripts/install.sh --dry-run
    install_language_configs

    if [[ -d "$XDG_DATA_HOME/pommel/languages" ]]; then
        echo "PASS: Languages directory created"
    else
        echo "FAIL: Languages directory not created"
        exit 1
    fi
    teardown_test_env
}

test_install_copies_all_configs() {
    # All 13 YAML files are copied
    setup_test_env
    source scripts/install.sh --dry-run
    install_language_configs

    expected_files=(
        "csharp.yaml" "dart.yaml" "elixir.yaml" "go.yaml"
        "java.yaml" "javascript.yaml" "kotlin.yaml" "php.yaml"
        "python.yaml" "rust.yaml" "solidity.yaml" "swift.yaml"
        "typescript.yaml"
    )

    for f in "${expected_files[@]}"; do
        if [[ ! -f "$XDG_DATA_HOME/pommel/languages/$f" ]]; then
            echo "FAIL: Missing $f"
            exit 1
        fi
    done
    echo "PASS: All 13 configs installed"
    teardown_test_env
}

test_install_sets_correct_permissions() {
    # Config files have 644 permissions
    setup_test_env
    source scripts/install.sh --dry-run
    install_language_configs

    perms=$(stat -f "%Lp" "$XDG_DATA_HOME/pommel/languages/go.yaml" 2>/dev/null || \
            stat -c "%a" "$XDG_DATA_HOME/pommel/languages/go.yaml")
    if [[ "$perms" == "644" ]]; then
        echo "PASS: Correct permissions"
    else
        echo "FAIL: Wrong permissions: $perms"
        exit 1
    fi
    teardown_test_env
}

# Success Conditions
test_install_overwrites_existing() {
    # Existing configs are overwritten with newer versions
    setup_test_env
    mkdir -p "$XDG_DATA_HOME/pommel/languages"
    echo "old content" > "$XDG_DATA_HOME/pommel/languages/go.yaml"

    source scripts/install.sh --dry-run
    install_language_configs

    content=$(cat "$XDG_DATA_HOME/pommel/languages/go.yaml")
    if [[ "$content" != "old content" ]]; then
        echo "PASS: Config overwritten"
    else
        echo "FAIL: Config not overwritten"
        exit 1
    fi
    teardown_test_env
}

test_install_preserves_custom_configs() {
    # User-added configs are not deleted
    setup_test_env
    mkdir -p "$XDG_DATA_HOME/pommel/languages"
    echo "custom: true" > "$XDG_DATA_HOME/pommel/languages/custom-lang.yaml"

    source scripts/install.sh --dry-run
    install_language_configs

    if [[ -f "$XDG_DATA_HOME/pommel/languages/custom-lang.yaml" ]]; then
        echo "PASS: Custom config preserved"
    else
        echo "FAIL: Custom config deleted"
        exit 1
    fi
    teardown_test_env
}

# Failure Conditions
test_install_handles_readonly_dir() {
    # Returns error if directory is read-only
    setup_test_env
    mkdir -p "$XDG_DATA_HOME/pommel"
    chmod 555 "$XDG_DATA_HOME/pommel"

    if ! source scripts/install.sh --dry-run; install_language_configs 2>/dev/null; then
        echo "PASS: Correctly failed on read-only dir"
    else
        echo "FAIL: Should have failed"
        exit 1
    fi
    chmod 755 "$XDG_DATA_HOME/pommel"
    teardown_test_env
}

# Edge Cases
test_install_xdg_not_set() {
    # Falls back to ~/.local/share when XDG_DATA_HOME not set
    setup_test_env
    unset XDG_DATA_HOME

    source scripts/install.sh --dry-run
    install_language_configs

    if [[ -d "$HOME/.local/share/pommel/languages" ]]; then
        echo "PASS: Fallback to ~/.local/share"
    else
        echo "FAIL: Fallback failed"
        exit 1
    fi
    teardown_test_env
}

test_install_from_github_release() {
    # Downloads configs from GitHub release if not bundled
    # (For curl|bash installation method)
    echo "SKIP: Requires network mock"
}

# Run all tests
run_all_tests() {
    test_install_creates_languages_dir
    test_install_copies_all_configs
    test_install_sets_correct_permissions
    test_install_overwrites_existing
    test_install_preserves_custom_configs
    test_install_handles_readonly_dir
    test_install_xdg_not_set
    echo "All installer tests passed!"
}

run_all_tests
```

#### 25.1.2: Implement install.sh Changes

```bash
# Add to install.sh:

# Language config installation
get_languages_dir() {
    if [[ -n "${POMMEL_LANGUAGES_DIR:-}" ]]; then
        echo "$POMMEL_LANGUAGES_DIR"
    elif [[ -n "${XDG_DATA_HOME:-}" ]]; then
        echo "$XDG_DATA_HOME/pommel/languages"
    else
        echo "$HOME/.local/share/pommel/languages"
    fi
}

install_language_configs() {
    local lang_dir
    lang_dir=$(get_languages_dir)

    step "Installing language configurations..."

    # Create directory
    if ! mkdir -p "$lang_dir"; then
        fail "Cannot create languages directory: $lang_dir"
        return 1
    fi

    # Download or copy configs
    local configs=(
        csharp dart elixir go java javascript
        kotlin php python rust solidity swift typescript
    )

    for lang in "${configs[@]}"; do
        local url="https://raw.githubusercontent.com/dbinky/Pommel/main/languages/${lang}.yaml"
        local dest="$lang_dir/${lang}.yaml"

        if ! curl -fsSL "$url" -o "$dest"; then
            warn "Failed to download ${lang}.yaml"
        fi
    done

    success "Installed ${#configs[@]} language configurations"
}
```

#### 25.1.3: Run Tests

```bash
bash scripts/install_test.sh
```

---

### 25.2: Installer Script Updates (Windows)

Update `scripts/install.ps1` to install language configuration files.

**File:** `scripts/install.ps1`

#### 25.2.1: Write Tests First

**File:** `scripts/install.tests.ps1` (Pester tests)

```powershell
#Requires -Modules Pester

Describe "Pommel Installer - Language Configs" {
    BeforeAll {
        # Create test environment
        $script:TestDir = Join-Path $env:TEMP "PommelInstallerTest"
        $script:OriginalLocalAppData = $env:LOCALAPPDATA
        New-Item -ItemType Directory -Path $TestDir -Force | Out-Null
        $env:LOCALAPPDATA = $TestDir

        # Source the installer functions
        . $PSScriptRoot\install.ps1 -WhatIf
    }

    AfterAll {
        $env:LOCALAPPDATA = $script:OriginalLocalAppData
        Remove-Item -Path $script:TestDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    # Happy Path
    Context "Happy Path" {
        It "Creates languages directory" {
            Install-LanguageConfigs

            $langDir = Join-Path $env:LOCALAPPDATA "Pommel\languages"
            Test-Path $langDir | Should -BeTrue
        }

        It "Installs all 13 language configs" {
            Install-LanguageConfigs

            $langDir = Join-Path $env:LOCALAPPDATA "Pommel\languages"
            $expectedFiles = @(
                "csharp.yaml", "dart.yaml", "elixir.yaml", "go.yaml",
                "java.yaml", "javascript.yaml", "kotlin.yaml", "php.yaml",
                "python.yaml", "rust.yaml", "solidity.yaml", "swift.yaml",
                "typescript.yaml"
            )

            foreach ($file in $expectedFiles) {
                $path = Join-Path $langDir $file
                Test-Path $path | Should -BeTrue -Because "$file should exist"
            }
        }
    }

    # Success Conditions
    Context "Success Conditions" {
        It "Overwrites existing configs" {
            $langDir = Join-Path $env:LOCALAPPDATA "Pommel\languages"
            New-Item -ItemType Directory -Path $langDir -Force | Out-Null
            "old content" | Out-File (Join-Path $langDir "go.yaml")

            Install-LanguageConfigs

            $content = Get-Content (Join-Path $langDir "go.yaml") -Raw
            $content | Should -Not -Be "old content"
        }

        It "Preserves custom configs" {
            $langDir = Join-Path $env:LOCALAPPDATA "Pommel\languages"
            $customFile = Join-Path $langDir "my-custom.yaml"
            "custom: true" | Out-File $customFile

            Install-LanguageConfigs

            Test-Path $customFile | Should -BeTrue
        }
    }

    # Failure Conditions
    Context "Failure Conditions" {
        It "Reports error on permission denied" {
            # Mock a read-only scenario
            Mock New-Item { throw "Access denied" }

            { Install-LanguageConfigs } | Should -Throw
        }
    }

    # Edge Cases
    Context "Edge Cases" {
        It "Handles LOCALAPPDATA with spaces" {
            $env:LOCALAPPDATA = Join-Path $script:TestDir "Path With Spaces"
            New-Item -ItemType Directory -Path $env:LOCALAPPDATA -Force | Out-Null

            { Install-LanguageConfigs } | Should -Not -Throw

            $langDir = Join-Path $env:LOCALAPPDATA "Pommel\languages"
            Test-Path $langDir | Should -BeTrue
        }

        It "Handles unicode in path" {
            $env:LOCALAPPDATA = Join-Path $script:TestDir "Ûñíçödé"
            New-Item -ItemType Directory -Path $env:LOCALAPPDATA -Force | Out-Null

            { Install-LanguageConfigs } | Should -Not -Throw
        }
    }
}
```

#### 25.2.2: Implement install.ps1 Changes

```powershell
# Add to install.ps1:

function Get-LanguagesDir {
    if ($env:POMMEL_LANGUAGES_DIR) {
        return $env:POMMEL_LANGUAGES_DIR
    }
    return Join-Path $env:LOCALAPPDATA "Pommel\languages"
}

function Install-LanguageConfigs {
    $langDir = Get-LanguagesDir

    Write-Step "Installing language configurations..."

    # Create directory
    if (-not (Test-Path $langDir)) {
        New-Item -ItemType Directory -Path $langDir -Force | Out-Null
    }

    # Language list
    $configs = @(
        "csharp", "dart", "elixir", "go", "java", "javascript",
        "kotlin", "php", "python", "rust", "solidity", "swift", "typescript"
    )

    foreach ($lang in $configs) {
        $url = "https://raw.githubusercontent.com/dbinky/Pommel/main/languages/${lang}.yaml"
        $dest = Join-Path $langDir "${lang}.yaml"

        try {
            Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
        }
        catch {
            Write-Warn "Failed to download ${lang}.yaml: $_"
        }
    }

    Write-Success "Installed $($configs.Count) language configurations"
}
```

#### 25.2.3: Run Tests

```powershell
Invoke-Pester scripts/install.tests.ps1
```

---

### 25.3: Startup Validation Tests

Test that the daemon correctly loads language configs on startup.

**File:** `internal/daemon/startup_test.go`

#### 25.3.1: Write Tests First

```go
// Happy Path
func TestDaemonStartup_LoadsLanguageConfigs(t *testing.T)
  // Daemon loads all configs from languages directory on startup

func TestDaemonStartup_LogsLoadedLanguages(t *testing.T)
  // Startup logs which languages were loaded

// Failure Conditions
func TestDaemonStartup_MissingLanguagesDir(t *testing.T)
  // Clear error message if languages directory missing

func TestDaemonStartup_EmptyLanguagesDir(t *testing.T)
  // Warning but continues if directory empty

func TestDaemonStartup_PartialLoadFailure(t *testing.T)
  // Continues with valid configs if some fail to load

// Edge Cases
func TestDaemonStartup_HotReload(t *testing.T)
  // Future: SIGHUP reloads configs without restart
```

#### 25.3.2: Implement Startup Changes

Update daemon startup to:
1. Call `config.EnsureLanguagesDir()`
2. Call `chunker.NewRegistryFromConfig(langDir, parser)`
3. Log loaded languages
4. Handle missing/empty directory gracefully

---

### 25.4: CLI Help Updates

Update CLI help text to reflect config-driven languages.

**Files:** Various CLI command files

#### 25.4.1: Update Help Text

```go
// pm init --help
"Supported languages are determined by configuration files in the languages directory."

// pm status --json
// Add "languages" field showing loaded language configs:
{
  "languages": {
    "loaded": ["csharp", "dart", "elixir", "go", ...],
    "config_dir": "/home/user/.local/share/pommel/languages"
  }
}

// pm config
// Add ability to show languages config path:
// pm config get languages.dir
```

#### 25.4.2: Write Tests for Help Changes

```go
func TestStatusOutput_IncludesLanguages(t *testing.T)
  // pm status --json includes languages section

func TestConfigGet_LanguagesDir(t *testing.T)
  // pm config get languages.dir returns correct path
```

---

### 25.5: Documentation Updates

Ensure all documentation reflects the new config-driven approach.

#### 25.5.1: README.md Updates

- [x] Supported languages table (already updated)
- [ ] Add section on adding custom languages
- [ ] Update troubleshooting for missing configs

#### 25.5.2: New Documentation

**File:** `docs/custom-languages.md`

```markdown
# Adding Custom Language Support

Pommel supports adding new languages via YAML configuration files.

## Quick Start

1. Create a YAML file in the languages directory:
   - macOS/Linux: `~/.local/share/pommel/languages/`
   - Windows: `%LOCALAPPDATA%\Pommel\languages\`

2. Define your language configuration:

\`\`\`yaml
language: mylang
display_name: My Language
extensions:
  - .ml

tree_sitter:
  grammar: mylang  # Must be a valid tree-sitter grammar

chunk_mappings:
  class:
    - class_definition
  method:
    - function_definition

extraction:
  name_field: name
  doc_comments:
    - comment
  doc_comment_position: preceding_siblings
\`\`\`

3. Restart the Pommel daemon:
\`\`\`bash
pm stop && pm start
\`\`\`

## Finding Tree-sitter Node Types

To determine the correct node types for your language:

1. Install tree-sitter CLI: `npm install -g tree-sitter-cli`
2. Parse a sample file: `tree-sitter parse myfile.ml`
3. Examine the AST output to identify node types

## Validation

Check your config is loaded:
\`\`\`bash
pm status --json | jq '.languages.loaded'
\`\`\`
```

---

### 25.6: CI/CD Updates

Update GitHub Actions to include language configs in releases.

**File:** `.github/workflows/release.yml`

#### 25.6.1: Add Language Configs to Release

```yaml
# Add step to bundle language configs
- name: Package language configs
  run: |
    mkdir -p dist/languages
    cp languages/*.yaml dist/languages/
    tar -czvf pommel-languages-${{ github.ref_name }}.tar.gz -C dist languages

- name: Upload language configs
  uses: softprops/action-gh-release@v1
  with:
    files: |
      pommel-languages-${{ github.ref_name }}.tar.gz
```

---

### 25.7: Final Cleanup

Remove any remaining legacy code and verify everything works.

#### 25.7.1: Remove Legacy Test Fixtures

Delete test fixtures that reference old chunker implementations.

#### 25.7.2: Update CLAUDE.md Template

Ensure `pm init --claude` generates correct documentation.

#### 25.7.3: Final Test Run

```bash
# Full test suite
go test -v -race ./...

# Linting
golangci-lint run

# Build for all platforms
make build-all

# Manual smoke test
pm init --auto --start
pm status --json
pm search "test query"
```

---

## Verification Checklist

- [ ] Unix installer tests pass
- [ ] Windows installer tests pass (Pester)
- [ ] Daemon startup tests pass
- [ ] CLI help text updated
- [ ] README.md updated
- [ ] Custom languages documentation created
- [ ] CI/CD packages language configs
- [ ] All platform builds succeed
- [ ] Smoke tests pass on macOS, Linux, Windows
- [ ] No regressions from Phase 24

## Files Changed

### New Files
- `scripts/install_test.sh`
- `scripts/install.tests.ps1`
- `docs/custom-languages.md`

### Modified Files
- `scripts/install.sh`
- `scripts/install.ps1`
- `internal/daemon/server.go` (startup)
- `internal/cli/status.go` (languages output)
- `internal/cli/init.go` (help text)
- `.github/workflows/release.yml`
- `README.md`

### Deleted Files
- Legacy chunker test fixtures (if any remain)
