# Configurable Embedding Dimensions for Ollama

**Issue**: [#54 - Allow configurable dimensions to be able to use different models](https://github.com/dbinky/Pommel/issues/54)
**Date**: 2026-01-27
**Status**: Design Complete

## Overview

Allow users to specify custom embedding dimensions for arbitrary Ollama models that aren't in Pommel's built-in registry.

### Problem

Users want to use Ollama embedding models like `qwen3-embedding:0.6b`, `gemmaembedding`, or `jinaai_jina-code-embeddings-1.5b`, but Pommel hardcodes dimensions at 768 for all Ollama models. The 0.8.0 release added a model registry for Jina v2/v4, but unknown models still default to 768 with no override.

### Solution

Add a `dimensions` field to the Ollama provider config. For unknown models, dimensions must be specified or the daemon fails with a helpful error.

## Config Changes

Add `dimensions` field to `embedding.ollama`:

```yaml
# .pommel/config.yaml
embedding:
  provider: ollama
  ollama:
    url: http://localhost:11434
    model: qwen3-embedding:0.6b
    dimensions: 1024  # NEW - required for unknown models
```

### Behavior

| Model Type | Dimensions Config | Result |
|------------|------------------|--------|
| Known (v2, v4) | Not set | Use registry dimensions |
| Known (v2, v4) | Set | Use registry dimensions (config ignored) |
| Unknown | Set | Use config dimensions |
| Unknown | Not set | Error with helpful message |

Registry takes precedence to prevent accidental misconfiguration of known models.

## Code Changes

### 1. Config Struct (`internal/config/config.go`)

```go
type OllamaProviderConfig struct {
    URL        string `yaml:"url" json:"url" mapstructure:"url"`
    Model      string `yaml:"model" json:"model" mapstructure:"model"`
    Dimensions int    `yaml:"dimensions" json:"dimensions" mapstructure:"dimensions"` // NEW
}
```

### 2. Provider Settings (`internal/embedder/provider.go`)

```go
type OllamaProviderSettings struct {
    URL        string
    Model      string
    Dimensions int  // NEW
}
```

### 3. Dimension Resolution (`internal/embedder/models.go`)

```go
// ResolveDimensions returns embedding dimensions for a model.
// Priority: 1) Registry lookup, 2) Config override, 3) Error
func ResolveDimensions(modelName string, configDimensions int) (int, error) {
    modelName = strings.TrimSpace(modelName)
    if modelName == "" {
        return 0, fmt.Errorf("model name cannot be empty")
    }

    // Check registry first
    if info := GetModelByFullName(modelName); info != nil {
        return info.Dimensions, nil
    }

    // Unknown model - require config dimensions
    if configDimensions <= 0 {
        return 0, fmt.Errorf(`Unknown embedding model '%s' requires dimensions.
Add to .pommel/config.yaml:

  embedding:
    ollama:
      dimensions: <your-model-dimensions>

Check your model's documentation for the correct dimension count.`, modelName)
    }

    return configDimensions, nil
}
```

### 4. Daemon Integration (`internal/daemon/daemon.go`)

Replace line ~146:

```go
// Current:
dims := embedder.ProviderType(providerCfg.Provider).DefaultDimensions()

// New:
var dims int
var err error
if providerCfg.Provider == "ollama" || providerCfg.Provider == "ollama-remote" {
    dims, err = embedder.ResolveDimensions(
        providerCfg.Ollama.Model,
        cfg.Embedding.Ollama.Dimensions,
    )
    if err != nil {
        return nil, &DaemonError{
            Code:       "UNKNOWN_MODEL_DIMENSIONS",
            Message:    err.Error(),
            Suggestion: "See error message above for config instructions",
        }
    }
} else {
    dims = embedder.ProviderType(providerCfg.Provider).DefaultDimensions()
}
```

## Dimension Mismatch Handling

Existing infrastructure handles this - no changes needed.

When dimensions change:
1. User edits config (e.g., `dimensions: 1024` → `dimensions: 512`)
2. Daemon detects mismatch on startup
3. Shows warning: "Embedding dimensions changed (was 1024, now 512). Database must be rebuilt. Run: pm reindex"
4. User runs `pm reindex`, database rebuilds with new dimensions

## Test Strategy

### ResolveDimensions Unit Tests (`internal/embedder/models_test.go`)

**Happy Path Scenarios**:
```go
TestResolveDimensions_V2Model_Returns768
TestResolveDimensions_V4Model_Returns1024
TestResolveDimensions_UnknownModel_WithDimensions_ReturnsConfigValue
```

**Success Scenarios**:
```go
TestResolveDimensions_V2Model_WithConfigOverride_ReturnsRegistryValue
TestResolveDimensions_V4Model_WithConfigOverride_ReturnsRegistryValue
TestResolveDimensions_UnknownModel_Dimensions256_Succeeds
TestResolveDimensions_UnknownModel_Dimensions512_Succeeds
TestResolveDimensions_UnknownModel_Dimensions1024_Succeeds
TestResolveDimensions_UnknownModel_Dimensions1536_Succeeds
TestResolveDimensions_UnknownModel_Dimensions4096_Succeeds
```

**Failure Scenarios**:
```go
TestResolveDimensions_UnknownModel_NoDimensions_ReturnsError
TestResolveDimensions_UnknownModel_ZeroDimensions_ReturnsError
```

**Error Scenarios**:
```go
TestResolveDimensions_Error_ContainsModelName
TestResolveDimensions_Error_ContainsConfigInstructions
TestResolveDimensions_Error_MentionsDimensionsField
```

**Edge Cases**:
```go
TestResolveDimensions_ModelNameWithWhitespace_Trimmed
TestResolveDimensions_EmptyModelName_ReturnsError
TestResolveDimensions_ModelNameCaseSensitive
TestResolveDimensions_NegativeDimensions_ReturnsError
TestResolveDimensions_Dimensions1_Succeeds
TestResolveDimensions_VeryLargeDimensions_Succeeds
TestResolveDimensions_PartialModelNameMatch_NotFound
```

### Config Parsing Tests (`internal/config/config_test.go`)

**Happy Path**:
```go
TestConfig_OllamaDimensions_ParsesFromYAML
TestConfig_FullOllamaConfig_WithDimensions
```

**Success Scenarios**:
```go
TestConfig_OllamaDimensions_WithoutModel_UsesDefault
TestConfig_OllamaDimensions_WithCustomURL
TestConfig_OllamaDimensions_Omitted_ReturnsZero
```

**Edge Cases**:
```go
TestConfig_OllamaDimensions_AsString_ParseError
TestConfig_OllamaDimensions_AsFloat_Truncated
```

### Daemon Integration Tests (`internal/daemon/daemon_test.go`)

**Happy Path**:
```go
TestDaemon_KnownModel_StartsWithoutDimensionsConfig
TestDaemon_UnknownModel_WithDimensions_Starts
```

**Success Scenarios**:
```go
TestDaemon_UnknownModel_DatabaseHasCorrectDimensions
TestDaemon_UnknownModel_EmbedderConfiguredCorrectly
```

**Failure Scenarios**:
```go
TestDaemon_UnknownModel_NoDimensions_FailsToStart
TestDaemon_UnknownModel_NoDimensions_ReturnsDaemonError
```

**Error Scenarios**:
```go
TestDaemon_UnknownModel_ErrorContainsConfigPath
```

**Edge Cases**:
```go
TestDaemon_EmptyModel_WithDimensions_UsesDefaultModel
TestDaemon_OllamaRemote_UnknownModel_RequiresDimensions
```

### Dimension Mismatch Tests (`internal/db/metadata_test.go`)

**Happy Path**:
```go
TestDimensionMismatch_DetectedOnStartup
TestDimensionMismatch_ReindexRebuildsWithNewDimensions
```

**Success Scenarios**:
```go
TestDimensionMismatch_SameDimensions_NoWarning
TestDimensionMismatch_NewDatabase_NoMismatch
```

**Failure Scenarios**:
```go
TestDimensionMismatch_SearchReturnsError
TestDimensionMismatch_IndexingReturnsError
```

**Edge Cases**:
```go
TestDimensionMismatch_DifferentModel_SameDimensions_NoMismatch
TestDimensionMismatch_UnknownToKnown_DifferentDimensions_Detected
```

## Files Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Dimensions int` to `OllamaProviderConfig` |
| `internal/embedder/provider.go` | Add `Dimensions int` to `OllamaProviderSettings` |
| `internal/embedder/models.go` | Add `ResolveDimensions()` function |
| `internal/daemon/daemon.go` | Use `ResolveDimensions()` for Ollama providers |

## Not In Scope

- CLI command for setting dimensions (can be added later)
- Dimensions config for OpenAI/Voyage providers (fixed dimensions)
- Auto-detection of dimensions from Ollama API

## Test Count

~35 test cases across 4 test files.
