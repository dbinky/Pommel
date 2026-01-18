# Jina v4 Embedding Support - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add optional Jina v4 embeddings (1024 dims) alongside default v2 (768 dims), selectable at install time.

**Architecture:** Model registry maps short names (v2/v4) to full Ollama model names and dimensions. Install script prompts for model choice. `pm config model` command allows switching with reindex.

**Tech Stack:** Go, Cobra CLI, testify, bash/PowerShell install scripts

---

## Task 1: Create Model Registry

**Files:**
- Create: `internal/embedder/models.go`
- Create: `internal/embedder/models_test.go`

### Step 1.1: Write failing test for GetModelInfo_V2

```go
// internal/embedder/models_test.go
package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelInfo_V2(t *testing.T) {
	info, err := GetModelInfo("v2")
	require.NoError(t, err)
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", info.Name)
	assert.Equal(t, 768, info.Dimensions)
	assert.Equal(t, 8192, info.ContextSize)
}
```

### Step 1.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_V2 -v
```

Expected: FAIL - `undefined: GetModelInfo`

### Step 1.3: Write minimal implementation

```go
// internal/embedder/models.go
package embedder

import (
	"fmt"
	"strings"
)

// ModelInfo contains metadata about an embedding model.
type ModelInfo struct {
	Name        string // Full Ollama model name
	Dimensions  int    // Embedding vector dimensions
	ContextSize int    // Maximum context window in tokens
	Size        string // Human-readable size (e.g., "~300MB")
}

// EmbeddingModels maps short names to model info.
var EmbeddingModels = map[string]ModelInfo{
	"v2": {
		Name:        "unclemusclez/jina-embeddings-v2-base-code",
		Dimensions:  768,
		ContextSize: 8192,
		Size:        "~300MB",
	},
	"v4": {
		Name:        "sellerscrisp/jina-embeddings-v4-text-code-q4",
		Dimensions:  1024,
		ContextSize: 32768,
		Size:        "~8GB",
	},
}

// DefaultModel is the short name of the default embedding model.
const DefaultModel = "v2"

// GetModelInfo returns model info by short name (v2, v4).
func GetModelInfo(shortName string) (*ModelInfo, error) {
	shortName = strings.ToLower(strings.TrimSpace(shortName))
	if shortName == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	info, ok := EmbeddingModels[shortName]
	if !ok {
		return nil, fmt.Errorf("unknown model '%s', use v2 or v4", shortName)
	}
	return &info, nil
}
```

### Step 1.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_V2 -v
```

Expected: PASS

### Step 1.5: Write failing test for GetModelInfo_V4

```go
func TestGetModelInfo_V4(t *testing.T) {
	info, err := GetModelInfo("v4")
	require.NoError(t, err)
	assert.Equal(t, "sellerscrisp/jina-embeddings-v4-text-code-q4", info.Name)
	assert.Equal(t, 1024, info.Dimensions)
	assert.Equal(t, 32768, info.ContextSize)
}
```

### Step 1.6: Run test to verify it passes (already implemented)

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_V4 -v
```

Expected: PASS

### Step 1.7: Write failing test for unknown model

```go
func TestGetModelInfo_UnknownShortName(t *testing.T) {
	_, err := GetModelInfo("v5")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model 'v5'")
}
```

### Step 1.8: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_UnknownShortName -v
```

Expected: PASS

### Step 1.9: Write failing test for empty name

```go
func TestGetModelInfo_EmptyName(t *testing.T) {
	_, err := GetModelInfo("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}
```

### Step 1.10: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_EmptyName -v
```

Expected: PASS

### Step 1.11: Write failing test for case insensitivity

```go
func TestGetModelInfo_CaseInsensitive(t *testing.T) {
	info, err := GetModelInfo("V4")
	require.NoError(t, err)
	assert.Equal(t, 1024, info.Dimensions)
}
```

### Step 1.12: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelInfo_CaseInsensitive -v
```

Expected: PASS

### Step 1.13: Commit

```bash
git add internal/embedder/models.go internal/embedder/models_test.go
git commit -m "feat(embedder): add model registry for v2/v4 embedding models"
```

---

## Task 2: Add GetModelByFullName Function

**Files:**
- Modify: `internal/embedder/models.go`
- Modify: `internal/embedder/models_test.go`

### Step 2.1: Write failing test

```go
func TestGetModelByFullName_V2(t *testing.T) {
	info := GetModelByFullName("unclemusclez/jina-embeddings-v2-base-code")
	require.NotNil(t, info)
	assert.Equal(t, 768, info.Dimensions)
}

func TestGetModelByFullName_V4(t *testing.T) {
	info := GetModelByFullName("sellerscrisp/jina-embeddings-v4-text-code-q4")
	require.NotNil(t, info)
	assert.Equal(t, 1024, info.Dimensions)
}

func TestGetModelByFullName_Unknown(t *testing.T) {
	info := GetModelByFullName("some-random-model")
	assert.Nil(t, info)
}
```

### Step 2.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelByFullName -v
```

Expected: FAIL - `undefined: GetModelByFullName`

### Step 2.3: Write minimal implementation

Add to `internal/embedder/models.go`:

```go
// GetModelByFullName returns model info by full Ollama model name.
// Returns nil if the model is not in our registry (unknown model).
func GetModelByFullName(fullName string) *ModelInfo {
	fullName = strings.TrimSpace(fullName)
	for _, info := range EmbeddingModels {
		if info.Name == fullName {
			return &info
		}
	}
	return nil
}
```

### Step 2.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetModelByFullName -v
```

Expected: PASS

### Step 2.5: Commit

```bash
git add internal/embedder/models.go internal/embedder/models_test.go
git commit -m "feat(embedder): add GetModelByFullName lookup function"
```

---

## Task 3: Add GetDimensionsForModel Function

**Files:**
- Modify: `internal/embedder/models.go`
- Modify: `internal/embedder/models_test.go`

### Step 3.1: Write failing tests

```go
func TestGetDimensionsForModel_V2(t *testing.T) {
	dims := GetDimensionsForModel("unclemusclez/jina-embeddings-v2-base-code")
	assert.Equal(t, 768, dims)
}

func TestGetDimensionsForModel_V4(t *testing.T) {
	dims := GetDimensionsForModel("sellerscrisp/jina-embeddings-v4-text-code-q4")
	assert.Equal(t, 1024, dims)
}

func TestGetDimensionsForModel_Unknown_DefaultsTo768(t *testing.T) {
	dims := GetDimensionsForModel("some-unknown-model")
	assert.Equal(t, 768, dims, "unknown models should default to 768")
}

func TestGetDimensionsForModel_Empty_DefaultsTo768(t *testing.T) {
	dims := GetDimensionsForModel("")
	assert.Equal(t, 768, dims, "empty model should default to 768")
}
```

### Step 3.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetDimensionsForModel -v
```

Expected: FAIL - `undefined: GetDimensionsForModel`

### Step 3.3: Write minimal implementation

Add to `internal/embedder/models.go`:

```go
// DefaultDimensions is the fallback for unknown models.
const DefaultDimensions = 768

// GetDimensionsForModel returns dimensions for a model by full name.
// Returns DefaultDimensions (768) for unknown models.
func GetDimensionsForModel(fullName string) int {
	info := GetModelByFullName(fullName)
	if info == nil {
		return DefaultDimensions
	}
	return info.Dimensions
}
```

### Step 3.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetDimensionsForModel -v
```

Expected: PASS

### Step 3.5: Commit

```bash
git add internal/embedder/models.go internal/embedder/models_test.go
git commit -m "feat(embedder): add GetDimensionsForModel with safe defaults"
```

---

## Task 4: Update OllamaClient to Use Model Registry

**Files:**
- Modify: `internal/embedder/ollama.go`
- Modify: `internal/embedder/ollama_test.go` (create if needed)

### Step 4.1: Write failing test for dynamic dimensions

```go
// internal/embedder/ollama_test.go
package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOllamaClient_Dimensions_V2Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "unclemusclez/jina-embeddings-v2-base-code",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 768, client.Dimensions())
}

func TestOllamaClient_Dimensions_V4Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "sellerscrisp/jina-embeddings-v4-text-code-q4",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 1024, client.Dimensions())
}

func TestOllamaClient_Dimensions_UnknownModel(t *testing.T) {
	cfg := OllamaConfig{
		Model: "some-random-model",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 768, client.Dimensions(), "unknown models default to 768")
}
```

### Step 4.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestOllamaClient_Dimensions -v
```

Expected: FAIL - V4 test returns 768 instead of 1024

### Step 4.3: Update Dimensions() method

Modify `internal/embedder/ollama.go`:

```go
// Dimensions returns the embedding dimension size based on the configured model.
func (c *OllamaClient) Dimensions() int {
	return GetDimensionsForModel(c.model)
}
```

### Step 4.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestOllamaClient_Dimensions -v
```

Expected: PASS

### Step 4.5: Commit

```bash
git add internal/embedder/ollama.go internal/embedder/ollama_test.go
git commit -m "feat(embedder): dynamic dimensions based on configured model"
```

---

## Task 5: Add ContextSize Method to OllamaClient

**Files:**
- Modify: `internal/embedder/ollama.go`
- Modify: `internal/embedder/ollama_test.go`

### Step 5.1: Write failing tests

```go
func TestOllamaClient_ContextSize_V2Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "unclemusclez/jina-embeddings-v2-base-code",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 8192, client.ContextSize())
}

func TestOllamaClient_ContextSize_V4Model(t *testing.T) {
	cfg := OllamaConfig{
		Model: "sellerscrisp/jina-embeddings-v4-text-code-q4",
	}
	client := NewOllamaClient(cfg)
	assert.Equal(t, 32768, client.ContextSize())
}
```

### Step 5.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestOllamaClient_ContextSize -v
```

Expected: FAIL - `undefined: client.ContextSize`

### Step 5.3: Add GetContextSizeForModel and ContextSize method

Add to `internal/embedder/models.go`:

```go
// DefaultContextSize is the fallback for unknown models.
const DefaultContextSize = 8192

// GetContextSizeForModel returns context size for a model by full name.
// Returns DefaultContextSize (8192) for unknown models.
func GetContextSizeForModel(fullName string) int {
	info := GetModelByFullName(fullName)
	if info == nil {
		return DefaultContextSize
	}
	return info.ContextSize
}
```

Add to `internal/embedder/ollama.go`:

```go
// ContextSize returns the context window size based on the configured model.
func (c *OllamaClient) ContextSize() int {
	return GetContextSizeForModel(c.model)
}
```

### Step 5.4: Update embed() to use dynamic context size

Modify the embed() method in `internal/embedder/ollama.go`:

```go
func (c *OllamaClient) embed(ctx context.Context, input any) ([][]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model: c.model,
		Input: input,
		Options: map[string]interface{}{
			"num_ctx": c.ContextSize(),
		},
	}
	// ... rest unchanged
}
```

### Step 5.5: Remove hardcoded JinaContextSize constant

Delete this line from `internal/embedder/ollama.go`:

```go
// DELETE THIS:
// const JinaContextSize = 8192
```

### Step 5.6: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestOllamaClient_ContextSize -v
```

Expected: PASS

### Step 5.7: Run all embedder tests

```bash
go test -tags fts5 ./internal/embedder/... -v
```

Expected: All PASS

### Step 5.8: Commit

```bash
git add internal/embedder/models.go internal/embedder/ollama.go internal/embedder/ollama_test.go
git commit -m "feat(embedder): dynamic context size based on model"
```

---

## Task 6: Add GetShortNameForModel Function

**Files:**
- Modify: `internal/embedder/models.go`
- Modify: `internal/embedder/models_test.go`

### Step 6.1: Write failing tests

```go
func TestGetShortNameForModel_V2(t *testing.T) {
	name := GetShortNameForModel("unclemusclez/jina-embeddings-v2-base-code")
	assert.Equal(t, "v2", name)
}

func TestGetShortNameForModel_V4(t *testing.T) {
	name := GetShortNameForModel("sellerscrisp/jina-embeddings-v4-text-code-q4")
	assert.Equal(t, "v4", name)
}

func TestGetShortNameForModel_Unknown(t *testing.T) {
	name := GetShortNameForModel("some-unknown-model")
	assert.Equal(t, "", name)
}
```

### Step 6.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetShortNameForModel -v
```

Expected: FAIL - `undefined: GetShortNameForModel`

### Step 6.3: Write minimal implementation

Add to `internal/embedder/models.go`:

```go
// GetShortNameForModel returns the short name (v2, v4) for a full model name.
// Returns empty string if model is not in registry.
func GetShortNameForModel(fullName string) string {
	fullName = strings.TrimSpace(fullName)
	for shortName, info := range EmbeddingModels {
		if info.Name == fullName {
			return shortName
		}
	}
	return ""
}
```

### Step 6.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/embedder/... -run TestGetShortNameForModel -v
```

Expected: PASS

### Step 6.5: Commit

```bash
git add internal/embedder/models.go internal/embedder/models_test.go
git commit -m "feat(embedder): add GetShortNameForModel reverse lookup"
```

---

## Task 7: Add pm config model Command - Show Current

**Files:**
- Create: `internal/cli/config_model.go`
- Create: `internal/cli/config_model_test.go`

### Step 7.1: Write failing test

```go
// internal/cli/config_model_test.go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigModel_ShowCurrent_Default(t *testing.T) {
	// Setup: create temp project with default config
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})
	projectRoot = projectDir

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "v2")
	assert.Contains(t, stdout.String(), "unclemusclez/jina-embeddings-v2-base-code")
}
```

### Step 7.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_ShowCurrent -v
```

Expected: FAIL - `undefined: newConfigModelCmd`

### Step 7.3: Write minimal implementation

```go
// internal/cli/config_model.go
package cli

import (
	"fmt"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(newConfigModelCmd())
}

func newConfigModelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "model [v2|v4]",
		Short: "View or change the embedding model",
		Long: `View or change the embedding model used for code search.

Without arguments, shows the current model.
With an argument, switches to the specified model (requires reindex).

Available models:
  v2  - Jina v2 Code (~300MB, 768 dims) - lightweight, fast
  v4  - Jina v4 Code (~8GB, 1024 dims) - best quality, larger`,
		RunE: runConfigModel,
	}
}

func runConfigModel(cmd *cobra.Command, args []string) error {
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return ErrConfigInvalid(err)
	}

	// No args - show current model
	if len(args) == 0 {
		return showCurrentModel(cmd, cfg)
	}

	// TODO: implement model switching
	return fmt.Errorf("model switching not yet implemented")
}

func showCurrentModel(cmd *cobra.Command, cfg *config.Config) error {
	modelName := cfg.Embedding.Ollama.Model
	if modelName == "" {
		modelName = embedder.EmbeddingModels[embedder.DefaultModel].Name
	}

	shortName := embedder.GetShortNameForModel(modelName)
	if shortName == "" {
		shortName = "custom"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Current model: %s (%s)\n", shortName, modelName)
	return nil
}
```

### Step 7.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_ShowCurrent -v
```

Expected: PASS

### Step 7.5: Commit

```bash
git add internal/cli/config_model.go internal/cli/config_model_test.go
git commit -m "feat(cli): add 'pm config model' to show current model"
```

---

## Task 8: Add Model Switching with Confirmation

**Files:**
- Modify: `internal/cli/config_model.go`
- Modify: `internal/cli/config_model_test.go`

### Step 8.1: Write failing test for switching

```go
func TestConfigModel_SetV4_NoExistingDB(t *testing.T) {
	// Setup: create temp project with v2 config, no database
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"v4"})
	projectRoot = projectDir

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "v4")
	assert.Contains(t, stdout.String(), "pm start")

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "sellerscrisp/jina-embeddings-v4-text-code-q4")
}
```

### Step 8.2: Run test to verify it fails

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_SetV4 -v
```

Expected: FAIL - returns "model switching not yet implemented"

### Step 8.3: Implement model switching

Update `internal/cli/config_model.go`:

```go
func runConfigModel(cmd *cobra.Command, args []string) error {
	loader := config.NewLoader(projectRoot)
	cfg, err := loader.Load()
	if err != nil {
		return ErrConfigInvalid(err)
	}

	// No args - show current model
	if len(args) == 0 {
		return showCurrentModel(cmd, cfg)
	}

	// Switch to requested model
	return switchModel(cmd, loader, cfg, args[0])
}

func switchModel(cmd *cobra.Command, loader *config.Loader, cfg *config.Config, target string) error {
	// Validate target model
	targetInfo, err := embedder.GetModelInfo(target)
	if err != nil {
		return NewCLIError(
			err.Error(),
			"Available models: v2 (lightweight), v4 (best quality)")
	}

	// Check if already using this model
	currentModel := cfg.Embedding.Ollama.Model
	if currentModel == targetInfo.Name {
		shortName := embedder.GetShortNameForModel(currentModel)
		fmt.Fprintf(cmd.OutOrStdout(), "Already using model %s\n", shortName)
		return nil
	}

	// Check for existing database
	dbPath := filepath.Join(projectRoot, ".pommel", "pommel.db")
	dbExists := false
	if _, err := os.Stat(dbPath); err == nil {
		dbExists = true
	}

	// Delete database if it exists
	if dbExists {
		if err := os.Remove(dbPath); err != nil {
			return WrapError(err,
				"Failed to delete existing database",
				"Check file permissions or delete .pommel/pommel.db manually")
		}
	}

	// Update config
	cfg.Embedding.Ollama.Model = targetInfo.Name
	if err := loader.Save(cfg); err != nil {
		return WrapError(err,
			"Failed to save configuration",
			"Check write permissions for the .pommel directory")
	}

	shortName := embedder.GetShortNameForModel(targetInfo.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Switched to model %s (%s)\n", shortName, targetInfo.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'pm start' to reindex with the new model.\n")
	return nil
}
```

Add import at top:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/spf13/cobra"
)
```

### Step 8.4: Run test to verify it passes

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_SetV4 -v
```

Expected: PASS

### Step 8.5: Commit

```bash
git add internal/cli/config_model.go internal/cli/config_model_test.go
git commit -m "feat(cli): implement model switching in 'pm config model'"
```

---

## Task 9: Add Invalid Model Test

**Files:**
- Modify: `internal/cli/config_model_test.go`

### Step 9.1: Write failing test

```go
func TestConfigModel_InvalidModel(t *testing.T) {
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	var stderr bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"v5"})
	projectRoot = projectDir

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model 'v5'")
}
```

### Step 9.2: Run test to verify it passes

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_InvalidModel -v
```

Expected: PASS (already implemented in switchModel)

### Step 9.3: Commit

```bash
git add internal/cli/config_model_test.go
git commit -m "test(cli): add invalid model test for pm config model"
```

---

## Task 10: Add Same Model No-Op Test

**Files:**
- Modify: `internal/cli/config_model_test.go`

### Step 10.1: Write test

```go
func TestConfigModel_SameModel(t *testing.T) {
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"v2"})
	projectRoot = projectDir

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Already using")
}
```

### Step 10.2: Run test to verify it passes

```bash
go test -tags fts5 ./internal/cli/... -run TestConfigModel_SameModel -v
```

Expected: PASS

### Step 10.3: Commit

```bash
git add internal/cli/config_model_test.go
git commit -m "test(cli): add same-model no-op test"
```

---

## Task 11: Update Install Script - Model Selection

**Files:**
- Modify: `scripts/install.sh`

### Step 11.1: Add model selection variable

Add after line 43 (OLLAMA_INSTALLED):

```bash
SELECTED_MODEL="v2"  # Default to v2
```

### Step 11.2: Add select_ollama_model function

Add after `setup_voyage()` function:

```bash
select_ollama_model() {
    echo ""
    echo "  Which embedding model?"
    echo ""
    echo "  1) Standard    - Jina v2 Code (~300MB, faster, good quality) (Recommended)"
    echo "  2) Maximum     - Jina v4 Code (~8GB, slower, best quality)"
    echo ""
    read -p "  Choice [1]: " model_choice < /dev/tty
    model_choice=${model_choice:-1}

    case $model_choice in
        1)
            SELECTED_MODEL="v2"
            success "Selected: Jina v2 Code (Standard)"
            ;;
        2)
            SELECTED_MODEL="v4"
            success "Selected: Jina v4 Code (Maximum quality)"
            ;;
        *)
            warn "Invalid choice. Using Standard (v2)."
            SELECTED_MODEL="v2"
            ;;
    esac
}
```

### Step 11.3: Call select_ollama_model after Ollama setup

Modify `setup_local_ollama()` to call model selection:

```bash
setup_local_ollama() {
    SELECTED_PROVIDER="ollama"
    success "Selected: Local Ollama"

    # Check for Ollama
    if ! command -v ollama &> /dev/null; then
        warn "Ollama not found on this machine."
        echo ""
        echo "  Install Ollama from: https://ollama.ai/download"
        echo ""
        read -p "  Continue anyway? (y/N) " -n 1 -r < /dev/tty
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            select_provider
            return
        fi
        OLLAMA_INSTALLED=false
    else
        success "Ollama found"
        OLLAMA_INSTALLED=true
    fi

    # Select model
    select_ollama_model
}
```

Also modify `setup_remote_ollama()`:

```bash
setup_remote_ollama() {
    SELECTED_PROVIDER="ollama-remote"
    echo ""
    read -p "  Enter Ollama server URL (e.g., http://192.168.1.100:11434): " url < /dev/tty

    if [[ -z "$url" ]]; then
        warn "URL is required for remote Ollama"
        setup_remote_ollama
        return
    fi

    OLLAMA_REMOTE_URL="$url"
    success "Selected: Remote Ollama at $url"

    # Select model
    select_ollama_model
    warn "Make sure the selected model is available on the remote server"
}
```

### Step 11.4: Update write_global_config to use selected model

Modify the ollama and ollama-remote cases in `write_global_config()`:

```bash
    case $SELECTED_PROVIDER in
        ollama)
            local model_name
            if [[ "$SELECTED_MODEL" == "v4" ]]; then
                model_name="sellerscrisp/jina-embeddings-v4-text-code-q4"
            else
                model_name="unclemusclez/jina-embeddings-v2-base-code"
            fi
            cat >> "$config_file" << EOF
  ollama:
    url: "http://localhost:11434"
    model: "$model_name"
EOF
            ;;
        ollama-remote)
            local model_name
            if [[ "$SELECTED_MODEL" == "v4" ]]; then
                model_name="sellerscrisp/jina-embeddings-v4-text-code-q4"
            else
                model_name="unclemusclez/jina-embeddings-v2-base-code"
            fi
            cat >> "$config_file" << EOF
  ollama:
    url: "$OLLAMA_REMOTE_URL"
    model: "$model_name"
EOF
            ;;
        # ... rest unchanged
    esac
```

### Step 11.5: Update setup_embedding_model to use selected model

```bash
setup_embedding_model() {
    if [[ "$SELECTED_PROVIDER" != "ollama" ]] || [[ "$OLLAMA_INSTALLED" != "true" ]]; then
        return
    fi

    step "[5/5] Setting up embedding model..."
    echo ""

    local MODEL
    local SIZE
    if [[ "$SELECTED_MODEL" == "v4" ]]; then
        MODEL="sellerscrisp/jina-embeddings-v4-text-code-q4"
        SIZE="~8GB"
    else
        MODEL="unclemusclez/jina-embeddings-v2-base-code"
        SIZE="~300MB"
    fi

    info "Pulling embedding model: $MODEL"
    info "This may take a few minutes on first run ($SIZE)..."

    # Check if Ollama is running
    if ! curl -s http://localhost:11434/ > /dev/null 2>&1; then
        warn "Ollama is not running. Starting Ollama..."
        if [[ "$OS" == "darwin" ]]; then
            open -a Ollama 2>/dev/null || ollama serve &
        else
            ollama serve &
        fi
        sleep 3
    fi

    ollama pull "$MODEL" || warn "Failed to pull model. Run 'ollama pull $MODEL' manually."
    success "Embedding model ready"
}
```

### Step 11.6: Commit

```bash
git add scripts/install.sh
git commit -m "feat(install): add model selection for Ollama provider"
```

---

## Task 12: Update PowerShell Install Script

**Files:**
- Modify: `scripts/install.ps1`

### Step 12.1: Add model selection variable and function

Add after `$script:OLLAMA_INSTALLED = $false`:

```powershell
$script:SELECTED_MODEL = "v2"

function Select-OllamaModel {
    Write-Host ""
    Write-Host "  Which embedding model?"
    Write-Host ""
    Write-Host "  1) Standard    - Jina v2 Code (~300MB, faster, good quality) (Recommended)"
    Write-Host "  2) Maximum     - Jina v4 Code (~8GB, slower, best quality)"
    Write-Host ""
    $choice = Read-Host "  Choice [1]"
    if ([string]::IsNullOrWhiteSpace($choice)) { $choice = "1" }

    switch ($choice) {
        "1" {
            $script:SELECTED_MODEL = "v2"
            Write-Success "Selected: Jina v2 Code (Standard)"
        }
        "2" {
            $script:SELECTED_MODEL = "v4"
            Write-Success "Selected: Jina v4 Code (Maximum quality)"
        }
        default {
            Write-Warning "Invalid choice. Using Standard (v2)."
            $script:SELECTED_MODEL = "v2"
        }
    }
}
```

### Step 12.2: Call Select-OllamaModel in Setup-LocalOllama and Setup-RemoteOllama

Update the functions similar to bash script.

### Step 12.3: Update Write-GlobalConfig and Setup-EmbeddingModel

Similar updates as bash script.

### Step 12.4: Commit

```bash
git add scripts/install.ps1
git commit -m "feat(install): add model selection for PowerShell installer"
```

---

## Task 13: Update Version to 0.8.0

**Files:**
- Modify: `cmd/pm/main.go`
- Modify: `cmd/pommeld/main.go`
- Modify: `internal/cli/root.go`

### Step 13.1: Update version strings

In all three files, change version from "0.7.3" to "0.8.0".

### Step 13.2: Commit

```bash
git add cmd/pm/main.go cmd/pommeld/main.go internal/cli/root.go
git commit -m "chore: bump version to 0.8.0"
```

---

## Task 14: Run Full Test Suite

### Step 14.1: Run all tests

```bash
go test -tags fts5 ./... -v
```

Expected: All PASS

### Step 14.2: Build binaries

```bash
go build -tags fts5 -o pm ./cmd/pm
go build -tags fts5 -o pommeld ./cmd/pommeld
```

Expected: Success

### Step 14.3: Manual smoke test

```bash
./pm version
./pm config model
```

---

## Task 15: Final Commit and Push

### Step 15.1: Ensure all changes committed

```bash
git status
```

### Step 15.2: Push branch

```bash
git push -u origin dev-jina-4
```

### Step 15.3: Create PR

```bash
gh pr create --base dev --title "feat: add Jina v4 embedding model support" --body "..."
```

---

## Summary

| Task | Description | Tests |
|------|-------------|-------|
| 1 | Model registry basics | 5 |
| 2 | GetModelByFullName | 3 |
| 3 | GetDimensionsForModel | 4 |
| 4 | OllamaClient dimensions | 3 |
| 5 | OllamaClient context size | 2 |
| 6 | GetShortNameForModel | 3 |
| 7 | pm config model (show) | 1 |
| 8 | pm config model (switch) | 1 |
| 9 | Invalid model error | 1 |
| 10 | Same model no-op | 1 |
| 11-12 | Install scripts | Manual |
| 13-15 | Version, tests, PR | - |

**Total unit tests: 24**
