# Configurable Embedding Dimensions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to specify custom embedding dimensions for arbitrary Ollama models via config.

**Architecture:** Add `dimensions` field to Ollama config, create `ResolveDimensions()` function that prioritizes registry lookup over config, integrate into daemon startup.

**Tech Stack:** Go 1.24+, testify for assertions, YAML config via Viper

---

## Task 1: Add ResolveDimensions Function - Happy Path Tests

**Files:**
- Test: `internal/embedder/models_test.go`
- Create: `internal/embedder/models.go` (add function)

**Step 1: Write the failing tests for known models**

Add to `internal/embedder/models_test.go`:

```go
func TestResolveDimensions_V2Model_Returns768(t *testing.T) {
	dims, err := ResolveDimensions("unclemusclez/jina-embeddings-v2-base-code", 0)
	require.NoError(t, err)
	assert.Equal(t, 768, dims)
}

func TestResolveDimensions_V4Model_Returns1024(t *testing.T) {
	dims, err := ResolveDimensions("sellerscrisp/jina-embeddings-v4-text-code-q4", 0)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims)
}

func TestResolveDimensions_UnknownModel_WithDimensions_ReturnsConfigValue(t *testing.T) {
	dims, err := ResolveDimensions("qwen3-embedding:0.6b", 1024)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions -v
```

Expected: FAIL - `undefined: ResolveDimensions`

**Step 3: Write minimal implementation**

Add to `internal/embedder/models.go`:

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

**Step 4: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/embedder/models.go internal/embedder/models_test.go
git commit -m "feat(embedder): add ResolveDimensions happy path"
```

---

## Task 2: Add ResolveDimensions - Success Scenarios

**Files:**
- Test: `internal/embedder/models_test.go`

**Step 1: Write tests for config override being ignored for known models**

Add to `internal/embedder/models_test.go`:

```go
func TestResolveDimensions_V2Model_WithConfigOverride_ReturnsRegistryValue(t *testing.T) {
	// Registry should take precedence over config
	dims, err := ResolveDimensions("unclemusclez/jina-embeddings-v2-base-code", 1024)
	require.NoError(t, err)
	assert.Equal(t, 768, dims, "Registry value should override config")
}

func TestResolveDimensions_V4Model_WithConfigOverride_ReturnsRegistryValue(t *testing.T) {
	dims, err := ResolveDimensions("sellerscrisp/jina-embeddings-v4-text-code-q4", 768)
	require.NoError(t, err)
	assert.Equal(t, 1024, dims, "Registry value should override config")
}

func TestResolveDimensions_UnknownModel_Dimensions256_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-256", 256)
	require.NoError(t, err)
	assert.Equal(t, 256, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions512_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-512", 512)
	require.NoError(t, err)
	assert.Equal(t, 512, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions1536_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-1536", 1536)
	require.NoError(t, err)
	assert.Equal(t, 1536, dims)
}

func TestResolveDimensions_UnknownModel_Dimensions4096_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("some-model-4096", 4096)
	require.NoError(t, err)
	assert.Equal(t, 4096, dims)
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions -v
```

Expected: PASS (implementation already handles these cases)

**Step 3: Commit**

```bash
git add internal/embedder/models_test.go
git commit -m "test(embedder): add ResolveDimensions success scenarios"
```

---

## Task 3: Add ResolveDimensions - Failure Scenarios

**Files:**
- Test: `internal/embedder/models_test.go`

**Step 1: Write tests for failure cases**

Add to `internal/embedder/models_test.go`:

```go
func TestResolveDimensions_UnknownModel_NoDimensions_ReturnsError(t *testing.T) {
	dims, err := ResolveDimensions("unknown-model", 0)
	require.Error(t, err)
	assert.Equal(t, 0, dims)
}

func TestResolveDimensions_UnknownModel_ZeroDimensions_ReturnsError(t *testing.T) {
	dims, err := ResolveDimensions("another-unknown-model", 0)
	require.Error(t, err)
	assert.Equal(t, 0, dims)
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/embedder/models_test.go
git commit -m "test(embedder): add ResolveDimensions failure scenarios"
```

---

## Task 4: Add ResolveDimensions - Error Message Tests

**Files:**
- Test: `internal/embedder/models_test.go`

**Step 1: Write tests for error message content**

Add to `internal/embedder/models_test.go`:

```go
func TestResolveDimensions_Error_ContainsModelName(t *testing.T) {
	_, err := ResolveDimensions("my-custom-model", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-custom-model")
}

func TestResolveDimensions_Error_ContainsConfigInstructions(t *testing.T) {
	_, err := ResolveDimensions("unknown-model", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".pommel/config.yaml")
	assert.Contains(t, err.Error(), "embedding:")
	assert.Contains(t, err.Error(), "ollama:")
}

func TestResolveDimensions_Error_MentionsDimensionsField(t *testing.T) {
	_, err := ResolveDimensions("unknown-model", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dimensions")
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions_Error -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/embedder/models_test.go
git commit -m "test(embedder): add ResolveDimensions error message tests"
```

---

## Task 5: Add ResolveDimensions - Edge Cases

**Files:**
- Test: `internal/embedder/models_test.go`

**Step 1: Write edge case tests**

Add to `internal/embedder/models_test.go`:

```go
func TestResolveDimensions_ModelNameWithWhitespace_Trimmed(t *testing.T) {
	// Leading/trailing whitespace should be trimmed
	dims, err := ResolveDimensions("  unclemusclez/jina-embeddings-v2-base-code  ", 0)
	require.NoError(t, err)
	assert.Equal(t, 768, dims)
}

func TestResolveDimensions_EmptyModelName_ReturnsError(t *testing.T) {
	dims, err := ResolveDimensions("", 1024)
	require.Error(t, err)
	assert.Equal(t, 0, dims)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestResolveDimensions_ModelNameCaseSensitive(t *testing.T) {
	// Model names are case-sensitive (different from short names like v2/V2)
	dims, err := ResolveDimensions("Unclemusclez/Jina-Embeddings-V2-Base-Code", 512)
	require.NoError(t, err)
	// Should use config value since case doesn't match registry
	assert.Equal(t, 512, dims)
}

func TestResolveDimensions_NegativeDimensions_ReturnsError(t *testing.T) {
	dims, err := ResolveDimensions("unknown-model", -100)
	require.Error(t, err)
	assert.Equal(t, 0, dims)
}

func TestResolveDimensions_Dimensions1_Succeeds(t *testing.T) {
	// Minimum valid dimension
	dims, err := ResolveDimensions("tiny-model", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, dims)
}

func TestResolveDimensions_VeryLargeDimensions_Succeeds(t *testing.T) {
	dims, err := ResolveDimensions("huge-model", 8192)
	require.NoError(t, err)
	assert.Equal(t, 8192, dims)
}

func TestResolveDimensions_PartialModelNameMatch_NotFound(t *testing.T) {
	// Partial match should NOT find the model
	dims, err := ResolveDimensions("jina-embeddings-v2", 512)
	require.NoError(t, err)
	// Should use config value since partial name doesn't match
	assert.Equal(t, 512, dims)
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/embedder/... -run TestResolveDimensions -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/embedder/models_test.go
git commit -m "test(embedder): add ResolveDimensions edge case tests"
```

---

## Task 6: Add Dimensions Field to Config Structs

**Files:**
- Modify: `internal/config/config.go:51-54`
- Modify: `internal/embedder/provider.go:113-116`

**Step 1: Add Dimensions to OllamaProviderConfig**

Edit `internal/config/config.go`, change `OllamaProviderConfig`:

```go
// OllamaProviderConfig contains Ollama-specific settings
type OllamaProviderConfig struct {
	URL        string `yaml:"url" json:"url" mapstructure:"url"`
	Model      string `yaml:"model" json:"model" mapstructure:"model"`
	Dimensions int    `yaml:"dimensions" json:"dimensions" mapstructure:"dimensions"`
}
```

**Step 2: Add Dimensions to OllamaProviderSettings**

Edit `internal/embedder/provider.go`, change `OllamaProviderSettings`:

```go
// OllamaProviderSettings holds Ollama-specific settings
type OllamaProviderSettings struct {
	URL        string
	Model      string
	Dimensions int
}
```

**Step 3: Run existing tests to ensure no regression**

```bash
go test -tags fts5 ./internal/config/... -v
go test -tags fts5 ./internal/embedder/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/config/config.go internal/embedder/provider.go
git commit -m "feat(config): add Dimensions field to Ollama config"
```

---

## Task 7: Add Config Parsing Tests for Dimensions

**Files:**
- Test: `internal/config/config_test.go`

**Step 1: Write config parsing tests**

Add to `internal/config/config_test.go`:

```go
func TestConfig_OllamaDimensions_ParsesFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
embedding:
  provider: ollama
  ollama:
    url: http://localhost:11434
    model: qwen3-embedding:0.6b
    dimensions: 1024
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, 1024, cfg.Embedding.Ollama.Dimensions)
}

func TestConfig_FullOllamaConfig_WithDimensions(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
embedding:
  provider: ollama
  batch_size: 32
  cache_size: 1000
  ollama:
    url: http://remote-server:11434
    model: gemmaembedding
    dimensions: 768
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "ollama", cfg.Embedding.Provider)
	assert.Equal(t, "http://remote-server:11434", cfg.Embedding.Ollama.URL)
	assert.Equal(t, "gemmaembedding", cfg.Embedding.Ollama.Model)
	assert.Equal(t, 768, cfg.Embedding.Ollama.Dimensions)
}

func TestConfig_OllamaDimensions_Omitted_ReturnsZero(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
embedding:
  provider: ollama
  ollama:
    url: http://localhost:11434
    model: some-model
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, 0, cfg.Embedding.Ollama.Dimensions, "Omitted dimensions should be zero")
}

func TestConfig_OllamaDimensions_WithCustomURL(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
embedding:
  provider: ollama-remote
  ollama:
    url: http://192.168.1.100:11434
    model: custom-embedder
    dimensions: 512
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
	assert.Equal(t, "http://192.168.1.100:11434", cfg.Embedding.Ollama.URL)
	assert.Equal(t, 512, cfg.Embedding.Ollama.Dimensions)
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/config/... -run TestConfig_Ollama -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test(config): add Ollama dimensions parsing tests"
```

---

## Task 8: Integrate ResolveDimensions into Daemon

**Files:**
- Modify: `internal/daemon/daemon.go:145-149`

**Step 1: Update daemon to use ResolveDimensions**

Edit `internal/daemon/daemon.go`. Find the section around line 145-149 that looks like:

```go
// Get embedding dimensions from provider before opening database
dims := embedder.ProviderType(providerCfg.Provider).DefaultDimensions()
```

Replace with:

```go
// Get embedding dimensions - use ResolveDimensions for Ollama providers
var dims int
if providerCfg.Provider == "ollama" || providerCfg.Provider == "ollama-remote" {
	var err error
	dims, err = embedder.ResolveDimensions(providerCfg.Ollama.Model, cfg.Embedding.Ollama.Dimensions)
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

**Step 2: Run existing daemon tests to ensure no regression**

```bash
go test -tags fts5 ./internal/daemon/... -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/daemon/daemon.go
git commit -m "feat(daemon): integrate ResolveDimensions for Ollama providers"
```

---

## Task 9: Add Daemon Tests for Unknown Model Handling

**Files:**
- Test: `internal/daemon/daemon_test.go`

**Step 1: Write test for unknown model without dimensions**

Add to `internal/daemon/daemon_test.go`:

```go
func TestNew_UnknownModel_NoDimensions_FailsToStart(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "unknown-custom-model"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions configured
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)

	var daemonErr *DaemonError
	require.ErrorAs(t, err, &daemonErr)
	assert.Equal(t, "UNKNOWN_MODEL_DIMENSIONS", daemonErr.Code)
	assert.Contains(t, daemonErr.Message, "unknown-custom-model")
}

func TestNew_UnknownModel_WithDimensions_Starts(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "custom-embedding-model"
	cfg.Embedding.Ollama.Dimensions = 512
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_KnownModel_StartsWithoutDimensionsConfig(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "unclemusclez/jina-embeddings-v2-base-code"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions needed for known model
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_OllamaRemote_UnknownModel_RequiresDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama-remote"
	cfg.Embedding.Ollama.URL = "http://remote-server:11434"
	cfg.Embedding.Ollama.Model = "remote-custom-model"
	cfg.Embedding.Ollama.Dimensions = 0 // No dimensions
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)

	// Assert
	require.Error(t, err)
	require.Nil(t, daemon)

	var daemonErr *DaemonError
	require.ErrorAs(t, err, &daemonErr)
	assert.Equal(t, "UNKNOWN_MODEL_DIMENSIONS", daemonErr.Code)
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/daemon/... -run TestNew_UnknownModel -v
go test -tags fts5 ./internal/daemon/... -run TestNew_KnownModel -v
go test -tags fts5 ./internal/daemon/... -run TestNew_OllamaRemote -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/daemon/daemon_test.go
git commit -m "test(daemon): add unknown model dimension handling tests"
```

---

## Task 10: Add Database Dimension Verification Tests

**Files:**
- Test: `internal/daemon/daemon_test.go`

**Step 1: Write test verifying database has correct dimensions**

Add to `internal/daemon/daemon_test.go`:

```go
func TestNew_UnknownModel_DatabaseHasCorrectDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "custom-model-1024"
	cfg.Embedding.Ollama.Dimensions = 1024
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Assert - verify database was created with correct dimensions
	assert.Equal(t, 1024, daemon.db.Dimensions())

	// Cleanup
	require.NoError(t, daemon.Close())
}

func TestNew_KnownModel_DatabaseHasRegistryDimensions(t *testing.T) {
	// Arrange
	projectRoot := t.TempDir()
	cfg := daemonTestConfig()
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "sellerscrisp/jina-embeddings-v4-text-code-q4"
	cfg.Embedding.Ollama.Dimensions = 768 // Wrong dimensions in config - should be ignored
	logger := daemonTestLogger()

	// Act
	daemon, err := New(projectRoot, cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, daemon)

	// Assert - registry dimensions (1024) should be used, not config (768)
	assert.Equal(t, 1024, daemon.db.Dimensions())

	// Cleanup
	require.NoError(t, daemon.Close())
}
```

**Step 2: Run tests to verify they pass**

```bash
go test -tags fts5 ./internal/daemon/... -run TestNew_UnknownModel_Database -v
go test -tags fts5 ./internal/daemon/... -run TestNew_KnownModel_Database -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/daemon/daemon_test.go
git commit -m "test(daemon): verify database dimensions match config/registry"
```

---

## Task 11: Run Full Test Suite

**Files:** None (verification only)

**Step 1: Run all tests**

```bash
go test -tags fts5 ./... -v
```

Expected: All tests PASS

**Step 2: Run tests with race detector**

```bash
go test -tags fts5 -race ./internal/embedder/... ./internal/config/... ./internal/daemon/...
```

Expected: No race conditions

**Step 3: Build to verify compilation**

```bash
go build -tags fts5 ./cmd/pm
go build -tags fts5 ./cmd/pommeld
```

Expected: Successful build

---

## Task 12: Final Commit and Summary

**Step 1: Review all changes**

```bash
git log --oneline HEAD~10..HEAD
git diff HEAD~10..HEAD --stat
```

**Step 2: Create summary commit if needed**

If there are any uncommitted changes:

```bash
git status
git add -A
git commit -m "chore: finalize configurable dimensions implementation"
```

**Step 3: Tag the completion**

```bash
git log --oneline -1
```

---

## Summary

| Task | Description | Test Count |
|------|-------------|------------|
| 1 | ResolveDimensions happy path | 3 |
| 2 | ResolveDimensions success scenarios | 6 |
| 3 | ResolveDimensions failure scenarios | 2 |
| 4 | ResolveDimensions error messages | 3 |
| 5 | ResolveDimensions edge cases | 7 |
| 6 | Config struct changes | 0 (regression) |
| 7 | Config parsing tests | 4 |
| 8 | Daemon integration | 0 (regression) |
| 9 | Daemon unknown model tests | 4 |
| 10 | Database dimension tests | 2 |
| 11 | Full test suite | 0 (verification) |
| 12 | Final commit | 0 |

**Total new tests:** ~31 test cases
