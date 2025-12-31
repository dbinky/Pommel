# Windows Troubleshooting

This guide covers common issues when running Pommel on Windows.

## Common Issues

### "pm" is not recognized as a command

**Cause:** PATH not updated or terminal not restarted.

**Solution:**
1. Close and reopen your terminal
2. Or refresh PATH in current session:
   ```powershell
   $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "User")
   ```
3. Verify installation: `pm version`

If still not working, check the install location:
```powershell
dir "$env:LOCALAPPDATA\Pommel\bin"
```

### Ollama not found after installation

**Cause:** Ollama PATH not in current session.

**Solution:**
1. Close and reopen terminal
2. Or refresh PATH:
   ```powershell
   $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
   ```
3. Verify: `ollama --version`

If Ollama still isn't found, reinstall via winget:
```powershell
winget install Ollama.Ollama
```

### File changes not detected

**Cause:** File may be locked by editor or antivirus.

**Solution:**
1. Save and close the file in your editor
2. Wait a few seconds for lock release
3. Run `pm reindex` if changes still not detected

Pommel includes retry logic for locked files, but persistent locks may require manual intervention.

### Daemon won't start

**Cause:** Port conflict or previous daemon still running.

**Solution:**
1. Check if already running:
   ```powershell
   pm status
   ```
2. Stop existing daemon:
   ```powershell
   pm stop
   ```
3. Check port 7420 not in use:
   ```powershell
   netstat -an | findstr 7420
   ```
4. Start fresh:
   ```powershell
   pm start
   ```

If the port is in use by another process, configure a different port:
```powershell
pm config set daemon.port 7421
pm start
```

### Permission denied errors

**Cause:** Antivirus or security software blocking access.

**Solution:**
1. Add Pommel install directory to antivirus exclusions:
   - `%LOCALAPPDATA%\Pommel`
2. Add project `.pommel/` directory to exclusions
3. Temporarily disable real-time protection to test
4. Run terminal as administrator (temporary workaround)

### Long path errors

**Cause:** Windows 260 character path limit.

**Solution:**
1. Enable long paths in Windows:
   - Run `regedit` as administrator
   - Navigate to `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\FileSystem`
   - Set `LongPathsEnabled` to `1`
   - Restart computer
2. Or use shorter project paths

### Slow initial indexing

**Cause:** Embedding generation is CPU-intensive.

**Solution:**
1. Initial indexing may take 2-5 minutes for ~1000 files
2. Subsequent updates are incremental and much faster
3. Add large generated directories to `.pommelignore`:
   ```
   node_modules/
   bin/
   obj/
   packages/
   ```

### Search returns empty results

**Cause:** Indexing not complete or daemon not running.

**Solution:**
1. Check status:
   ```powershell
   pm status --json
   ```
2. Wait for `pending_changes` to reach 0
3. Force reindex if needed:
   ```powershell
   pm reindex
   ```

### Database corruption

**Cause:** Unclean daemon shutdown or disk issues.

**Solution:**
```powershell
# Stop the daemon
pm stop

# Remove the database (will be rebuilt)
Remove-Item .pommel\index.db

# Restart and reindex
pm start
pm reindex
```

## PowerShell Execution Policy

If you can't run `install.ps1`, you may need to adjust the execution policy:

```powershell
# Check current policy
Get-ExecutionPolicy

# Allow running scripts (current user only)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

Or run the script bypassing the policy:
```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

## Windows Defender SmartScreen

When running downloaded executables, Windows may show a SmartScreen warning.

**Solution:**
1. Click "More info"
2. Click "Run anyway"

Or download via the install script which handles this automatically.

## Getting Help

If issues persist:
1. Check daemon logs: `type .pommel\daemon.log`
2. Run with verbose output: `pm search "query" -v`
3. Report issue: https://github.com/dbinky/Pommel/issues

When reporting issues, include:
- Windows version (`winver`)
- PowerShell version (`$PSVersionTable`)
- Error messages
- Steps to reproduce
