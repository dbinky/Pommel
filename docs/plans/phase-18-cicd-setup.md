# Phase 18: CI/CD Setup for Windows

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** None (first phase)

## Objective

Add Windows runners to GitHub Actions CI and release workflows, enabling automated testing and binary production for Windows x64 and ARM64.

## Background

Pommel currently builds and tests on Linux and macOS. Adding Windows support requires:
1. Running tests on Windows to catch platform-specific issues
2. Producing Windows binaries (.exe) for releases
3. Supporting both x64 and ARM64 architectures

## Current State

### CI Workflow (`.github/workflows/ci.yml`)
- Runs on: ubuntu-latest, macos-latest
- Executes: `go test ./...`
- No Windows coverage

### Release Workflow (`.github/workflows/release.yml`)
- Builds for: darwin-amd64, darwin-arm64, linux-amd64
- No Windows builds
- No linux-arm64 builds

## Implementation Tasks

### Task 1: Read and Understand Current CI Workflow

**Files to read:**
- `.github/workflows/ci.yml`

**Understand:**
- Current matrix configuration
- Test execution steps
- Any OS-specific conditionals

### Task 2: Add Windows to CI Test Matrix

**File:** `.github/workflows/ci.yml`

**Changes:**
```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
  fail-fast: false  # Don't cancel other jobs if one fails
```

**Considerations:**
- `fail-fast: false` allows seeing all failures, not just first
- Windows runners may need different shell configuration
- CGO compilation requires proper setup on Windows

### Task 3: Handle CGO on Windows

**Issue:** Pommel uses sqlite-vec which requires CGO. Windows needs MinGW or MSVC.

**Solution:** GitHub's windows-latest runners include MinGW-w64 by default.

**Verify by adding step:**
```yaml
- name: Check CGO availability (Windows)
  if: runner.os == 'Windows'
  run: |
    gcc --version
    go env CGO_ENABLED
  shell: bash
```

### Task 4: Read and Understand Current Release Workflow

**Files to read:**
- `.github/workflows/release.yml`

**Understand:**
- How binaries are built
- How release assets are uploaded
- Naming conventions

### Task 5: Add Windows Builds to Release Workflow

**File:** `.github/workflows/release.yml`

**Add to build matrix:**
```yaml
matrix:
  include:
    # Existing
    - os: ubuntu-latest
      goos: linux
      goarch: amd64
      binary_suffix: ""
    - os: ubuntu-latest
      goos: linux
      goarch: arm64
      binary_suffix: ""
    - os: macos-latest
      goos: darwin
      goarch: amd64
      binary_suffix: ""
    - os: macos-latest
      goos: darwin
      goarch: arm64
      binary_suffix: ""
    # New Windows builds
    - os: windows-latest
      goos: windows
      goarch: amd64
      binary_suffix: ".exe"
    - os: windows-latest
      goos: windows
      goarch: arm64
      binary_suffix: ".exe"
```

**Binary naming:**
- `pm-windows-amd64.exe`
- `pm-windows-arm64.exe`
- `pommeld-windows-amd64.exe`
- `pommeld-windows-arm64.exe`

### Task 6: Add Linux ARM64 Builds

While we're updating the release workflow, also add linux-arm64:
- `pm-linux-arm64`
- `pommeld-linux-arm64`

### Task 7: Test CI Changes

**Process:**
1. Push branch to GitHub
2. Verify CI runs on all three platforms
3. Check for any Windows-specific test failures
4. Document any failures for subsequent phases

### Task 8: Test Release Workflow (Dry Run)

**Process:**
1. Create a test tag (e.g., `v0.0.0-windows-test`)
2. Verify release workflow produces all expected binaries
3. Download and verify Windows binaries have `.exe` extension
4. Delete test release after verification

## Test Cases

### CI Workflow Tests

| Test | Expected Result |
|------|-----------------|
| Push to branch triggers CI | All three OS runners start |
| Go tests run on Windows | Tests execute (may have failures) |
| CGO compiles on Windows | sqlite-vec builds successfully |
| Matrix failure isolation | One OS failing doesn't cancel others |

### Release Workflow Tests

| Test | Expected Result |
|------|-----------------|
| Tag triggers release | Release workflow starts |
| Windows x64 binary built | `pm-windows-amd64.exe` in assets |
| Windows ARM64 binary built | `pm-windows-arm64.exe` in assets |
| Linux ARM64 binary built | `pm-linux-arm64` in assets |
| Binary is executable | Downloaded .exe runs on Windows |

## Acceptance Criteria

- [ ] CI workflow runs tests on ubuntu-latest, macos-latest, windows-latest
- [ ] All existing tests pass on Linux and macOS (baseline)
- [ ] Windows test run completes (failures documented for later phases)
- [ ] Release workflow produces 12 binaries (2 binaries x 6 platform/arch combos)
- [ ] Windows binaries have `.exe` extension
- [ ] Binaries are correctly named per convention

## Files Changed

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | Add windows-latest to matrix |
| `.github/workflows/release.yml` | Add Windows and Linux ARM64 builds |

## Rollback Plan

If Windows CI causes issues:
1. Remove windows-latest from CI matrix
2. Keep release workflow changes (builds don't affect CI)
3. Document issues for investigation

## Notes

- Windows ARM64 builds may need cross-compilation since GitHub doesn't have ARM64 Windows runners yet
- If ARM64 cross-compilation fails, fall back to x64 only initially
- CGO cross-compilation for ARM64 may require additional setup
