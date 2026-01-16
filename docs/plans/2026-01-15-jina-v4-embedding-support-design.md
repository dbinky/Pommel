# Jina v4 Embedding Support Design

**Date:** 2026-01-15
**Branch:** dev-jina-4
**Version:** 0.8.0

## Overview

Add support for Jina v4 embeddings as an optional upgrade from the default v2 model, selectable at install time. This provides users with better code search quality while keeping the lightweight v2 as the default for most users.

## Models

| Model | Ollama Name | Size | Dimensions | Notes |
|-------|-------------|------|------------|-------|
| v2 (default) | `unclemusclez/jina-embeddings-v2-base-code` | ~300MB | 768 | Lightweight, well-tested |
| v4 (optional) | `sellerscrisp/jina-embeddings-v4-text-code-q4` | ~8GB | 1024 | Better quality, larger |

## User Experience

### Install Script Flow

When Ollama is selected as the provider, prompt for model choice:

```
[2/5] Configure embedding provider

  How would you like to generate embeddings?

  1) Local Ollama    - Free, runs on this machine
  2) Remote Ollama   - Free, connect to Ollama on another machine
  3) OpenAI API      - Paid, no local setup required
  4) Voyage AI       - Paid, optimized for code search

  Choice [1]: 1

  Which embedding model?

  1) Standard    - Jina v2 Code (~300MB, faster, good quality) (Recommended)
  2) Maximum     - Jina v4 Code (~8GB, slower, best quality)

  Choice [1]: _
```

### Per-Project Override

```bash
pm config model v4   # Switch this project to v4 (triggers reindex prompt)
pm config model v2   # Switch back to v2
```

## Configuration Storage

### Global Config (`~/.config/pommel/config.yaml`)

```yaml
embedding:
  provider: ollama
  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"
```

### Per-Project Override (`.pommel/config.yaml`)

```yaml
embedding:
  ollama:
    model: "sellerscrisp/jina-embeddings-v4-text-code-q4"
```

### Model Metadata (Hardcoded in Go)

```go
var EmbeddingModels = map[string]ModelInfo{
    "v2": {
        Name:       "unclemusclez/jina-embeddings-v2-base-code",
        Dimensions: 768,
        Size:       "~300MB",
    },
    "v4": {
        Name:       "sellerscrisp/jina-embeddings-v4-text-code-q4",
        Dimensions: 1024,
        Size:       "~8GB",
    },
}
```

## Code Changes

### Files to Modify

1. **`scripts/install.sh`** (and `install.ps1`)
   - Add model selection prompt after provider selection
   - Store model choice in global config
   - Pull the selected model

2. **`internal/embedder/ollama.go`**
   - Add `ModelInfo` struct and model registry
   - Update `Dimensions()` to return based on configured model
   - Remove hardcoded `JinaContextSize` constant, make it model-aware

3. **`internal/cli/config.go`**
   - Add `pm config model [v2|v4]` subcommand
   - Warn user that changing model requires reindex
   - Update config file with new model

### No Changes Needed

- `internal/config/config.go` - Already supports `embedding.ollama.model`
- `internal/db/vectors.go` - Already supports dynamic dimensions via `db.Open(path, dimensions)`
- Daemon already reads model from config

## Model Switching Behavior

### When user runs `pm config model v4`

1. Check if daemon is running - if yes, prompt to stop it first
2. Check current model in config vs requested model
3. If different:
   - Warn: "Switching models requires a full reindex. The existing index will be deleted."
   - Prompt: "Continue? (y/N)"
   - If yes: Update config, delete `.pommel/pommel.db`
   - Print: "Model changed to v4. Run `pm start` to reindex with the new model."

### When daemon starts with mismatched dimensions

If the database exists but has different dimensions than the configured model:
- Log error: "Database dimensions (768) don't match configured model (1024). Run `pm reindex --force` or `pm config model v2` to fix."
- Exit with error (don't silently corrupt data)

### Fresh project (`pm init`)

Uses whatever model is in global config. No special handling needed.

## Edge Cases

1. **Remote Ollama with model choice:** Same prompt, but warn that the model must also be available on the remote server

2. **Upgrade from v0.7.3:** Existing installs keep their current model (v2). No automatic migration.

3. **Unknown model in config:** If user manually sets a model not in our registry, fall back to querying Ollama for dimensions or default to 768

4. **Model pull fails:** Same behavior as today - warn and continue, user can pull manually

## Testing Strategy

Strict TDD: All tests must be written and fail before implementation code is written.

### Test Files to Create/Modify

| File | Purpose |
|------|---------|
| `internal/embedder/models_test.go` | Model registry tests |
| `internal/embedder/ollama_test.go` | Ollama client dimension tests |
| `internal/cli/config_model_test.go` | `pm config model` command tests |
| `internal/daemon/daemon_test.go` | Dimension mismatch detection tests |
| `internal/config/config_test.go` | Config loading/override tests |

---

### 1. Model Registry Tests (`internal/embedder/models_test.go`)

#### Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestGetModelInfo_V2` | Look up v2 model by short name | Returns correct name, 768 dims |
| `TestGetModelInfo_V4` | Look up v4 model by short name | Returns correct name, 1024 dims |
| `TestGetModelInfo_ByFullName` | Look up by full Ollama model name | Returns matching ModelInfo |
| `TestGetDimensions_V2` | Get dimensions for v2 model | Returns 768 |
| `TestGetDimensions_V4` | Get dimensions for v4 model | Returns 1024 |

#### Failure Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestGetModelInfo_UnknownShortName` | Look up "v5" (doesn't exist) | Returns error |
| `TestGetModelInfo_EmptyName` | Look up empty string | Returns error |

#### Edge Cases

| Test | Description | Expected |
|------|-------------|----------|
| `TestGetModelInfo_UnknownFullName` | Look up unregistered Ollama model name | Returns nil, no error (unknown model) |
| `TestGetDimensions_UnknownModel_DefaultsTo768` | Get dims for unknown model | Returns 768 (safe default) |
| `TestIsKnownModel_V2` | Check if v2 full name is known | Returns true |
| `TestIsKnownModel_Unknown` | Check if random model is known | Returns false |

---

### 2. Ollama Client Tests (`internal/embedder/ollama_test.go`)

#### Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestOllamaClient_Dimensions_V2Model` | Client configured with v2 model | `Dimensions()` returns 768 |
| `TestOllamaClient_Dimensions_V4Model` | Client configured with v4 model | `Dimensions()` returns 1024 |
| `TestOllamaClient_ContextSize_V2Model` | Client configured with v2 model | Context size is 8192 |
| `TestOllamaClient_ContextSize_V4Model` | Client configured with v4 model | Context size is 32768 |

#### Edge Cases

| Test | Description | Expected |
|------|-------------|----------|
| `TestOllamaClient_Dimensions_UnknownModel` | Client with unregistered model | Returns 768 (safe default) |
| `TestOllamaClient_Dimensions_EmptyModel` | Client with empty model string | Returns 768 (default) |

---

### 3. Config Model Command Tests (`internal/cli/config_model_test.go`)

#### Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfigModel_ShowCurrent` | Run `pm config model` with no args | Prints current model (v2 or v4) |
| `TestConfigModel_SetV4_Confirmed` | Set to v4, user confirms | Config updated, db deleted, success message |
| `TestConfigModel_SetV2_Confirmed` | Set to v2, user confirms | Config updated, db deleted, success message |
| `TestConfigModel_SameModel` | Set to already-configured model | No-op, prints "already using v2" |

#### Success Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfigModel_SetV4_NoExistingDB` | Set to v4, no db exists | Config updated, no deletion needed |
| `TestConfigModel_SetV4_ProjectConfigCreated` | Set in project dir | `.pommel/config.yaml` created/updated |
| `TestConfigModel_OutputIncludesReindexInstructions` | After successful switch | Output includes "Run `pm start` to reindex" |

#### Failure Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfigModel_SetV4_Declined` | Set to v4, user says no | Config unchanged, db unchanged |
| `TestConfigModel_InvalidModel` | Run `pm config model v5` | Error: "unknown model 'v5', use v2 or v4" |
| `TestConfigModel_DaemonRunning` | Switch while daemon runs | Error: "stop daemon first with `pm stop`" |

#### Error Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfigModel_ConfigWriteError` | Config file not writable | Error with helpful message |
| `TestConfigModel_DBDeleteError` | DB file locked/not deletable | Error with helpful message |
| `TestConfigModel_NoProjectRoot` | Run outside any project | Uses global config or errors appropriately |

#### Edge Cases

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfigModel_CaseInsensitive` | Run `pm config model V4` | Works same as `v4` |
| `TestConfigModel_FullModelName` | Run `pm config model unclemusclez/...` | Recognizes and maps to v2 |
| `TestConfigModel_ProjectOverridesGlobal` | Global=v2, project=v4 | Shows v4 as current |

---

### 4. Daemon Dimension Mismatch Tests (`internal/daemon/daemon_test.go`)

#### Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestDaemon_Start_V2Config_V2DB` | Config=v2, DB has 768 dims | Starts successfully |
| `TestDaemon_Start_V4Config_V4DB` | Config=v4, DB has 1024 dims | Starts successfully |
| `TestDaemon_Start_V2Config_NoDB` | Config=v2, no existing DB | Creates DB with 768 dims, starts |
| `TestDaemon_Start_V4Config_NoDB` | Config=v4, no existing DB | Creates DB with 1024 dims, starts |

#### Failure Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestDaemon_Start_V4Config_V2DB` | Config=v4, DB has 768 dims | Error: dimension mismatch |
| `TestDaemon_Start_V2Config_V4DB` | Config=v2, DB has 1024 dims | Error: dimension mismatch |

#### Error Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestDaemon_DimensionMismatch_ErrorMessage` | Mismatch detected | Error includes both dimensions and fix instructions |
| `TestDaemon_DimensionMismatch_ExitCode` | Mismatch detected | Non-zero exit code |

#### Edge Cases

| Test | Description | Expected |
|------|-------------|----------|
| `TestDaemon_Start_UnknownModel_ExistingDB` | Unknown model, DB exists | Uses DB's dimensions, warns |
| `TestDaemon_Start_CorruptedDB` | DB exists but unreadable | Error with helpful message |

---

### 5. Config Loading Tests (`internal/config/config_test.go`)

#### Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfig_LoadGlobalModel` | Global config has model set | Returns correct model name |
| `TestConfig_LoadProjectModel` | Project config has model set | Returns project model |
| `TestConfig_ProjectOverridesGlobal` | Both set, different values | Project model wins |
| `TestConfig_DefaultModel_NoConfig` | No config files exist | Returns v2 model name |

#### Success Scenarios

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfig_GlobalOnly_NoProjectConfig` | Global set, no project config | Returns global model |
| `TestConfig_PartialProjectConfig` | Project config exists but no model | Falls back to global model |

#### Edge Cases

| Test | Description | Expected |
|------|-------------|----------|
| `TestConfig_EmptyModelString` | Model set to empty string | Falls back to default v2 |
| `TestConfig_WhitespaceModelString` | Model set to whitespace | Falls back to default v2 |
| `TestConfig_OllamaRemote_ModelConfig` | Remote Ollama with model | Model config still applies |

---

### 6. Integration Tests

#### End-to-End Happy Path

| Test | Description | Expected |
|------|-------------|----------|
| `TestE2E_FreshInstall_DefaultV2` | New install, accept defaults | v2 model configured, 768-dim DB |
| `TestE2E_FreshInstall_SelectV4` | New install, select v4 | v4 model configured, 1024-dim DB |
| `TestE2E_SwitchV2ToV4_Reindex` | Start v2, switch to v4, reindex | Search works with new model |
| `TestE2E_ProjectOverride` | Global v2, project v4 | Project uses v4, other projects use v2 |

#### End-to-End Failure Recovery

| Test | Description | Expected |
|------|-------------|----------|
| `TestE2E_MismatchDetection_Recovery` | Create mismatch, run daemon | Clear error, user can fix with suggested command |
| `TestE2E_SwitchModel_CancelMidway` | Start switch, cancel | Original config/db preserved |

---

### Test Utilities to Create

```go
// internal/testutil/models.go

// CreateTestDBWithDimensions creates a test database with specified dimensions
func CreateTestDBWithDimensions(t *testing.T, path string, dims int) *db.DB

// CreateTestConfig creates a test config with specified model
func CreateTestConfig(t *testing.T, dir string, model string) *config.Config

// MockUserInput simulates user input for interactive prompts
func MockUserInput(t *testing.T, inputs ...string) func()
```

---

### TDD Workflow

For each component:

1. **Write failing test** - Test must fail with clear "not implemented" or wrong value
2. **Verify failure reason** - Confirm test fails for the right reason
3. **Write minimal code** - Just enough to pass the test
4. **Verify pass** - All tests green
5. **Refactor** - Clean up while keeping tests green
6. **Repeat** - Next test case

### Test Execution Order

1. Model registry tests (foundation)
2. Config loading tests (depends on registry)
3. Ollama client dimension tests (depends on registry)
4. Config model command tests (depends on config loading)
5. Daemon mismatch tests (depends on all above)
6. Integration tests (full system)

## Summary

| Aspect | Decision |
|--------|----------|
| Default model | v2 (~300MB, 768 dims) |
| Optional model | v4 (~8GB, 1024 dims) |
| Choice timing | Install script, after Ollama provider selection |
| Storage | Global default + per-project override |
| Model switching | Requires explicit reindex, warns user |
| Dimension mismatch | Hard error, don't corrupt data |
