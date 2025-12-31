# Phase 23: Documentation and Release

**Status:** Planning
**Branch:** dev-windows-support
**Depends on:** Phases 18-22 (all implementation complete)

## Objective

Update all documentation to include Windows instructions, prepare for release, and announce Windows support.

## Documentation Updates

### Task 1: Update README.md

**File:** `README.md`

**Sections to update:**

#### Installation Section

Add Windows instructions alongside existing Unix instructions:

```markdown
## Installation

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

### Manual Installation

Download binaries from [releases](https://github.com/dbinky/Pommel/releases):

| Platform | Architecture | Binary |
|----------|--------------|--------|
| macOS | Intel | pm-darwin-amd64, pommeld-darwin-amd64 |
| macOS | Apple Silicon | pm-darwin-arm64, pommeld-darwin-arm64 |
| Linux | x64 | pm-linux-amd64, pommeld-linux-amd64 |
| Linux | ARM64 | pm-linux-arm64, pommeld-linux-arm64 |
| Windows | x64 | pm-windows-amd64.exe, pommeld-windows-amd64.exe |
| Windows | ARM64 | pm-windows-arm64.exe, pommeld-windows-arm64.exe |
```

#### Requirements Section

Update requirements to include Windows:

```markdown
## Requirements

- **Operating System:** macOS, Linux, or Windows 10/11
- **Ollama:** For embedding generation
- **Embedding Model:** `unclemusclez/jina-embeddings-v2-base-code`
```

#### Platform Notes Section

Add new section:

```markdown
## Platform Notes

### Windows

- Pommel runs natively on Windows (not WSL required)
- PowerShell 5.1+ required for install script
- Ollama installed via winget (Windows 10 1709+)
- Data stored in project `.pommel/` directory
- Daemon runs as background process (not Windows Service)

### macOS / Linux

- Standard Unix process management
- Install script uses curl + bash
```

### Task 2: Update CLAUDE.md

**File:** `CLAUDE.md`

Update status and platform information:

```markdown
**Status:** v0.3.0 - Full cross-platform support (macOS, Linux, Windows)
```

Update technology stack table:

```markdown
| Platform Support | macOS (Intel/ARM), Linux (x64/ARM64), Windows (x64/ARM64) |
```

### Task 3: Update pm init --claude Output

**File:** `internal/cli/init.go`

Update `pommelClaudeInstructions` to mention Windows:

```go
**Supported platforms:** macOS, Linux, Windows
**Supported languages** (full AST-aware chunking): Go, Java, C#, Python, JavaScript, TypeScript, JSX, TSX
```

### Task 4: Create Windows-Specific Troubleshooting

**File:** `docs/troubleshooting-windows.md`

```markdown
# Windows Troubleshooting

## Common Issues

### "pm" is not recognized as a command

**Cause:** PATH not updated or terminal not restarted.

**Solution:**
1. Close and reopen your terminal
2. Or run: `$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "User")`
3. Verify: `pm version`

### Ollama not found after installation

**Cause:** Ollama PATH not in current session.

**Solution:**
1. Close and reopen terminal
2. Or refresh PATH: `$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")`
3. Verify: `ollama --version`

### File changes not detected

**Cause:** File may be locked by editor or antivirus.

**Solution:**
1. Save and close the file in your editor
2. Wait a few seconds for lock release
3. Run `pm reindex` if changes still not detected

### Daemon won't start

**Cause:** Port conflict or previous daemon still running.

**Solution:**
1. Check if already running: `pm status`
2. Stop existing: `pm stop`
3. Check port 7890 not in use: `netstat -an | findstr 7890`
4. Start fresh: `pm start`

### Permission denied errors

**Cause:** Antivirus or security software blocking access.

**Solution:**
1. Add Pommel install directory to antivirus exclusions
2. Add project `.pommel/` directory to exclusions
3. Run terminal as administrator (temporary workaround)

### Long path errors

**Cause:** Windows 260 character path limit.

**Solution:**
1. Enable long paths in Windows:
   - Run `regedit` as administrator
   - Navigate to `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\FileSystem`
   - Set `LongPathsEnabled` to `1`
   - Restart computer
2. Or use shorter project paths

## Getting Help

If issues persist:
1. Check daemon logs (future feature)
2. Run with verbose output: `pm search "query" -v`
3. Report issue: https://github.com/dbinky/Pommel/issues
```

### Task 5: Update Release Notes Template

**File:** `.github/RELEASE_TEMPLATE.md` (or in workflow)

```markdown
## What's New

[Description of changes]

## Installation

### macOS / Linux
```bash
curl -fsSL https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.sh | bash
```

### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

## Downloads

| Platform | Architecture | CLI | Daemon |
|----------|--------------|-----|--------|
| macOS | Intel | [pm-darwin-amd64]() | [pommeld-darwin-amd64]() |
| macOS | Apple Silicon | [pm-darwin-arm64]() | [pommeld-darwin-arm64]() |
| Linux | x64 | [pm-linux-amd64]() | [pommeld-linux-amd64]() |
| Linux | ARM64 | [pm-linux-arm64]() | [pommeld-linux-arm64]() |
| Windows | x64 | [pm-windows-amd64.exe]() | [pommeld-windows-amd64.exe]() |
| Windows | ARM64 | [pm-windows-arm64.exe]() | [pommeld-windows-arm64.exe]() |

## Checksums

[SHA256 checksums]
```

### Task 6: Create Windows Testing Context Document

**File:** `docs/plans/windows_testing.md`

Context document for Windows Claude instance:

```markdown
# Windows Testing Context

This document provides context for testing Pommel on Windows.

## Project Overview

Pommel is a semantic code search tool. This branch (dev-windows-support)
adds native Windows support.

## What to Test

### Installation Script
1. Open PowerShell
2. Run: `irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex`
3. Expected: Binaries download, PATH updated, Ollama installed, model pulled
4. Verify: `pm version` works

### Basic Workflow
1. Create test project: `mkdir C:\temp\pommel-test && cd C:\temp\pommel-test`
2. Initialize: `pm init --auto --start`
3. Create test file: `echo "package main`nfunc main() {}" > main.go`
4. Wait 2 seconds for indexing
5. Search: `pm search "main function"`
6. Expected: Results returned with main.go

### Daemon Management
1. Check status: `pm status`
2. Stop daemon: `pm stop`
3. Verify stopped: `pm status`
4. Start daemon: `pm start`
5. Verify running: `pm status`

### File Watching
1. With daemon running, modify a file
2. Wait 2 seconds
3. Search for new content
4. Expected: New content found

### Edge Cases
1. Path with spaces: `C:\Users\Name\My Projects\test`
2. Deep nesting: Create 10+ level deep directory
3. Rapid saves: Save file 5 times quickly, verify single reindex

## Reporting Issues

If tests fail:
1. Note exact error message
2. Note Windows version (`winver`)
3. Note PowerShell version (`$PSVersionTable`)
4. Create beads issue with details

## Beads Integration

This machine has beads installed. Check for open issues:
```bash
bd list --status=open
bd ready
```

Update issues as you test:
```bash
bd update <id> --status=in_progress
bd close <id> --reason="Tested successfully on Windows 11"
```
```

## Release Tasks

### Task 7: Version Bump

Update version to 0.3.0 (or appropriate version):

**Files:**
- `internal/cli/version.go` - Version constant
- `CLAUDE.md` - Status line

### Task 8: Final Testing Checklist

Before release:

- [ ] All CI tests pass (Linux, macOS, Windows)
- [ ] Manual testing on Windows complete
- [ ] README updated with Windows instructions
- [ ] CLAUDE.md updated
- [ ] Troubleshooting guide created
- [ ] Release workflow produces all 12 binaries
- [ ] Install script works on Windows

### Task 9: Create Release

1. Merge dev-windows-support to dev
2. Create PR from dev to main
3. Merge PR
4. Tag release: `git tag v0.3.0`
5. Push tag: `git push origin v0.3.0`
6. Verify release assets created
7. Update release notes

### Task 10: Announcement

Prepare announcement for:
- GitHub release notes
- Any relevant communities

```markdown
## Pommel v0.3.0 - Windows Support

Pommel now runs natively on Windows!

### Installation

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/dbinky/Pommel/main/scripts/install.ps1 | iex
```

### What's New
- Native Windows support (x64 and ARM64)
- PowerShell installation script
- Cross-platform daemon management
- Windows-optimized file watching

### Full Changelog
[link to changelog]
```

## Acceptance Criteria

- [ ] README includes Windows installation instructions
- [ ] CLAUDE.md updated with Windows support
- [ ] Windows troubleshooting guide created
- [ ] pm init --claude mentions Windows support
- [ ] Version bumped appropriately
- [ ] Release workflow produces 12 binaries
- [ ] Release notes prepared
- [ ] All documentation reviewed for accuracy

## Files Changed

| File | Change |
|------|--------|
| `README.md` | Windows installation instructions |
| `CLAUDE.md` | Platform support update |
| `internal/cli/init.go` | Windows in generated CLAUDE.md |
| `internal/cli/version.go` | Version bump |
| `docs/troubleshooting-windows.md` | New troubleshooting guide |
| `docs/plans/windows_testing.md` | Testing context document |

## Notes

- Consider creating video/GIF showing Windows installation
- May want blog post or detailed announcement
- Monitor issues after release for Windows-specific problems
- Plan for quick patch release if critical issues found
