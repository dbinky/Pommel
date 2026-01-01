# Phase 24: Config-Driven Language Infrastructure

**Parent Design:** [2026-01-01-config-driven-languages-design.md](2026-01-01-config-driven-languages-design.md)
**Version Target:** v0.4.0
**Estimated Complexity:** Medium-High

## Overview

Replace hardcoded language chunkers with a configuration-driven system. This phase implements the core infrastructure: YAML config parsing, platform-specific path resolution, a generic config-driven chunker, and registry refactoring.

## Prerequisites

- Go 1.21+
- Existing chunker tests passing
- Understanding of tree-sitter AST node types

## TDD Approach

**CRITICAL: All implementation follows strict TDD:**
1. Write failing test first
2. Verify test fails for the right reason
3. Write minimal code to pass
4. Refactor if needed
5. Repeat

## Tasks

### 24.1: Platform Path Resolution

Create cross-platform path resolution for finding language config directories.

**File:** `internal/config/paths.go`

#### 24.1.1: Write Tests First

**File:** `internal/config/paths_test.go`

```go
// Tests to implement:

// Happy Path
func TestLanguagesDir_Unix(t *testing.T)
  // On macOS/Linux: returns ~/.local/share/pommel/languages

func TestLanguagesDir_Windows(t *testing.T)
  // On Windows: returns %LOCALAPPDATA%\Pommel\languages

func TestLanguagesDir_WithEnvOverride(t *testing.T)
  // POMMEL_LANGUAGES_DIR env var overrides default

// Success Conditions
func TestLanguagesDir_CreatesIfNotExists(t *testing.T)
  // Directory is created if missing (with proper permissions)

func TestLanguagesDir_ExistingDirUnchanged(t *testing.T)
  // Existing directory is returned unchanged

// Failure Conditions
func TestLanguagesDir_InvalidHomeDir(t *testing.T)
  // Returns error if HOME/LOCALAPPDATA not set and no override

func TestLanguagesDir_PermissionDenied(t *testing.T)
  // Returns error if cannot create directory

// Edge Cases
func TestLanguagesDir_TrailingSlash(t *testing.T)
  // Handles paths with/without trailing slashes

func TestLanguagesDir_SymlinkDirectory(t *testing.T)
  // Works correctly with symlinked directories

func TestLanguagesDir_UnicodeInPath(t *testing.T)
  // Handles unicode characters in path (Windows usernames)
```

#### 24.1.2: Run Tests - Verify Failures

```bash
go test -v ./internal/config/... -run TestLanguagesDir
# All tests should fail (functions don't exist yet)
```

#### 24.1.3: Implement paths.go

```go
// Key functions to implement:
func LanguagesDir() (string, error)
func EnsureLanguagesDir() (string, error)
func getDefaultLanguagesDir() string  // platform-specific
```

#### 24.1.4: Run Tests - Verify Pass

```bash
go test -v ./internal/config/... -run TestLanguagesDir
```

---

### 24.2: YAML Config Schema and Parser

Define the config schema and implement YAML parsing with validation.

**File:** `internal/chunker/langconfig.go`

#### 24.2.1: Write Tests First

**File:** `internal/chunker/langconfig_test.go`

```go
// Tests to implement:

// Happy Path
func TestParseLanguageConfig_ValidYAML(t *testing.T)
  // Parses well-formed YAML into LanguageConfig struct

func TestParseLanguageConfig_AllFields(t *testing.T)
  // All schema fields are correctly parsed

func TestParseLanguageConfig_MinimalConfig(t *testing.T)
  // Config with only required fields works

// Success Conditions
func TestLoadLanguageConfig_FromFile(t *testing.T)
  // Loads config from filesystem path

func TestLoadAllLanguageConfigs_Directory(t *testing.T)
  // Loads all *.yaml files from directory

func TestLoadAllLanguageConfigs_SkipsMalformed(t *testing.T)
  // Skips malformed files, continues loading others, returns warnings

// Failure Conditions
func TestParseLanguageConfig_InvalidYAML(t *testing.T)
  // Returns parse error for malformed YAML

func TestParseLanguageConfig_MissingRequiredFields(t *testing.T)
  // Returns validation error if 'language' field missing

func TestParseLanguageConfig_MissingExtensions(t *testing.T)
  // Returns validation error if 'extensions' empty

func TestParseLanguageConfig_MissingGrammar(t *testing.T)
  // Returns validation error if tree_sitter.grammar missing

func TestLoadLanguageConfig_FileNotFound(t *testing.T)
  // Returns appropriate error for missing file

func TestLoadAllLanguageConfigs_EmptyDirectory(t *testing.T)
  // Returns empty slice (not error) for empty directory

func TestLoadAllLanguageConfigs_DirectoryNotFound(t *testing.T)
  // Returns error for missing directory

// Edge Cases
func TestParseLanguageConfig_ExtraFields(t *testing.T)
  // Ignores unknown fields (forward compatibility)

func TestParseLanguageConfig_EmptyChunkMappings(t *testing.T)
  // Handles config with no chunk_mappings (file-level only)

func TestParseLanguageConfig_DuplicateNodeTypes(t *testing.T)
  // Handles duplicate node types in mappings

func TestParseLanguageConfig_CaseInsensitiveExtensions(t *testing.T)
  // Extensions are normalized to lowercase

func TestLoadAllLanguageConfigs_DuplicateLanguages(t *testing.T)
  // Last loaded wins if same language defined twice
```

#### 24.2.2: Define Config Structs

```go
// LanguageConfig represents a language configuration from YAML
type LanguageConfig struct {
    Language    string            `yaml:"language"`
    DisplayName string            `yaml:"display_name"`
    Extensions  []string          `yaml:"extensions"`
    TreeSitter  TreeSitterConfig  `yaml:"tree_sitter"`
    ChunkMappings ChunkMappings   `yaml:"chunk_mappings"`
    Extraction  ExtractionConfig  `yaml:"extraction"`
}

type TreeSitterConfig struct {
    Grammar string `yaml:"grammar"`
}

type ChunkMappings struct {
    Class  []string `yaml:"class"`
    Method []string `yaml:"method"`
    Block  []string `yaml:"block"`
}

type ExtractionConfig struct {
    NameField          string   `yaml:"name_field"`
    DocComments        []string `yaml:"doc_comments"`
    DocCommentPosition string   `yaml:"doc_comment_position"`
}
```

#### 24.2.3: Implement Parser

```go
func ParseLanguageConfig(data []byte) (*LanguageConfig, error)
func LoadLanguageConfig(path string) (*LanguageConfig, error)
func LoadAllLanguageConfigs(dir string) ([]*LanguageConfig, []error)
func (c *LanguageConfig) Validate() error
```

#### 24.2.4: Run Tests - Verify Pass

```bash
go test -v ./internal/chunker/... -run TestParseLanguageConfig
go test -v ./internal/chunker/... -run TestLoadLanguageConfig
go test -v ./internal/chunker/... -run TestLoadAllLanguageConfigs
```

---

### 24.3: Language Config Unit Tests (All 13 Languages)

Test that each shipped language config is valid and correctly structured.

**File:** `internal/chunker/languages_test.go`

#### 24.3.1: Write Tests First

```go
// Individual language config tests

func TestLanguageConfig_CSharp(t *testing.T)
  // Validates csharp.yaml: extensions, grammar, chunk mappings

func TestLanguageConfig_Dart(t *testing.T)
  // Validates dart.yaml

func TestLanguageConfig_Elixir(t *testing.T)
  // Validates elixir.yaml

func TestLanguageConfig_Go(t *testing.T)
  // Validates go.yaml

func TestLanguageConfig_Java(t *testing.T)
  // Validates java.yaml

func TestLanguageConfig_JavaScript(t *testing.T)
  // Validates javascript.yaml

func TestLanguageConfig_Kotlin(t *testing.T)
  // Validates kotlin.yaml

func TestLanguageConfig_PHP(t *testing.T)
  // Validates php.yaml

func TestLanguageConfig_Python(t *testing.T)
  // Validates python.yaml

func TestLanguageConfig_Rust(t *testing.T)
  // Validates rust.yaml

func TestLanguageConfig_Solidity(t *testing.T)
  // Validates solidity.yaml

func TestLanguageConfig_Swift(t *testing.T)
  // Validates swift.yaml

func TestLanguageConfig_TypeScript(t *testing.T)
  // Validates typescript.yaml

// Aggregate tests
func TestAllLanguageConfigs_Load(t *testing.T)
  // All 13 configs load without error

func TestAllLanguageConfigs_UniqueLanguageIDs(t *testing.T)
  // No duplicate language identifiers

func TestAllLanguageConfigs_UniqueExtensions(t *testing.T)
  // No overlapping extensions between languages

func TestAllLanguageConfigs_ValidGrammars(t *testing.T)
  // All grammar names are valid tree-sitter grammars

func TestAllLanguageConfigs_HaveChunkMappings(t *testing.T)
  // All configs have at least class or method mappings

// Per-language validation helper
func validateLanguageConfig(t *testing.T, name string, cfg *LanguageConfig) {
    t.Helper()

    assert.NotEmpty(t, cfg.Language)
    assert.NotEmpty(t, cfg.DisplayName)
    assert.NotEmpty(t, cfg.Extensions)
    assert.NotEmpty(t, cfg.TreeSitter.Grammar)

    // All extensions should start with '.'
    for _, ext := range cfg.Extensions {
        assert.True(t, strings.HasPrefix(ext, "."))
    }

    // Should have at least one chunk mapping level
    hasMapping := len(cfg.ChunkMappings.Class) > 0 ||
                  len(cfg.ChunkMappings.Method) > 0
    assert.True(t, hasMapping, "config should have chunk mappings")
}
```

#### 24.3.2: Run Tests - Verify Pass

```bash
go test -v ./internal/chunker/... -run TestLanguageConfig
go test -v ./internal/chunker/... -run TestAllLanguageConfigs
```

---

### 24.4: Generic Config-Driven Chunker

Implement a single chunker that uses LanguageConfig to drive AST traversal.

**File:** `internal/chunker/generic.go`

#### 24.4.1: Write Tests First

**File:** `internal/chunker/generic_test.go`

```go
// Happy Path
func TestGenericChunker_ChunksGoFile(t *testing.T)
  // Using go.yaml config, chunks a Go file correctly

func TestGenericChunker_ChunksJavaFile(t *testing.T)
  // Using java.yaml config, chunks a Java file correctly

func TestGenericChunker_ChunksPythonFile(t *testing.T)
  // Using python.yaml config, chunks a Python file correctly

func TestGenericChunker_FindsClasses(t *testing.T)
  // Correctly identifies class-level chunks

func TestGenericChunker_FindsMethods(t *testing.T)
  // Correctly identifies method-level chunks

func TestGenericChunker_FindsNestedClasses(t *testing.T)
  // Handles nested class definitions

func TestGenericChunker_ExtractsName(t *testing.T)
  // Correctly extracts chunk names using name_field config

func TestGenericChunker_ExtractsSignature(t *testing.T)
  // Correctly extracts signatures

// Success Conditions
func TestGenericChunker_FileChunkAlwaysCreated(t *testing.T)
  // File-level chunk is always created

func TestGenericChunker_ParentChildRelationships(t *testing.T)
  // Parent IDs are correctly set

func TestGenericChunker_LineNumbers(t *testing.T)
  // Start/end lines are correct

// Failure Conditions
func TestGenericChunker_ParseError(t *testing.T)
  // Returns error for unparseable code

func TestGenericChunker_NilConfig(t *testing.T)
  // Returns error if config is nil

func TestGenericChunker_UnsupportedGrammar(t *testing.T)
  // Returns error for unknown tree-sitter grammar

// Edge Cases
func TestGenericChunker_EmptyFile(t *testing.T)
  // Handles empty file (returns empty chunks, no error)

func TestGenericChunker_NoMatchingNodes(t *testing.T)
  // File with no matching node types returns file chunk only

func TestGenericChunker_VeryLargeFile(t *testing.T)
  // Handles large files without stack overflow

func TestGenericChunker_UnicodeContent(t *testing.T)
  // Handles unicode in code (comments, strings, identifiers)

func TestGenericChunker_MixedIndentation(t *testing.T)
  // Handles mixed tabs/spaces

func TestGenericChunker_ContextCancellation(t *testing.T)
  // Respects context cancellation
```

#### 24.4.2: Implement GenericChunker

```go
type GenericChunker struct {
    config *LanguageConfig
    parser *Parser
}

func NewGenericChunker(parser *Parser, config *LanguageConfig) (*GenericChunker, error)
func (c *GenericChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error)
func (c *GenericChunker) Language() Language
func (c *GenericChunker) isClassNode(nodeType string) bool
func (c *GenericChunker) isMethodNode(nodeType string) bool
func (c *GenericChunker) extractName(node *sitter.Node, source []byte) string
```

#### 24.4.3: Run Tests - Verify Pass

```bash
go test -v ./internal/chunker/... -run TestGenericChunker
```

---

### 24.5: Registry Refactoring

Update the chunker registry to load from config files instead of hardcoded registration.

**File:** `internal/chunker/registry.go` (modify existing)

#### 24.5.1: Write Tests First

**File:** `internal/chunker/registry_test.go` (add to existing)

```go
// Happy Path
func TestRegistry_LoadFromConfig(t *testing.T)
  // Registry loads chunkers from config directory

func TestRegistry_GetChunkerByExtension(t *testing.T)
  // GetChunker returns correct chunker for file extension

func TestRegistry_GetChunkerByLanguage(t *testing.T)
  // GetChunkerByLanguage returns correct chunker

func TestRegistry_AllLanguagesRegistered(t *testing.T)
  // All 13 languages have registered chunkers

// Success Conditions
func TestRegistry_MultipleExtensionsPerLanguage(t *testing.T)
  // TypeScript registers for .ts, .tsx, .mts, .cts

func TestRegistry_SupportedLanguages(t *testing.T)
  // SupportedLanguages() returns all 13 languages

func TestRegistry_IsSupported(t *testing.T)
  // IsSupported returns true for all configured extensions

// Failure Conditions
func TestRegistry_UnsupportedExtension(t *testing.T)
  // Returns nil/error for unsupported extension

func TestRegistry_EmptyConfigDir(t *testing.T)
  // Handles empty config directory gracefully

func TestRegistry_MissingConfigDir(t *testing.T)
  // Returns clear error if config dir missing

// Edge Cases
func TestRegistry_CaseInsensitiveExtensions(t *testing.T)
  // .GO and .go both resolve to Go chunker

func TestRegistry_ReloadConfigs(t *testing.T)
  // Can reload configs without restart (for future hot-reload)
```

#### 24.5.2: Implement Registry Changes

```go
// New/modified functions:
func NewRegistryFromConfig(configDir string, parser *Parser) (*ChunkerRegistry, error)
func (r *ChunkerRegistry) loadConfigs(configDir string) error
func (r *ChunkerRegistry) registerFromConfig(config *LanguageConfig) error
```

#### 24.5.3: Update Existing Code Paths

- Modify `NewChunkerRegistry` to call `NewRegistryFromConfig`
- Remove hardcoded language registrations
- Update daemon startup to use config-based registry

#### 24.5.4: Run Tests - Verify Pass

```bash
go test -v ./internal/chunker/... -run TestRegistry
```

---

### 24.6: Integration Testing

End-to-end tests verifying the complete config-driven chunking pipeline.

**File:** `internal/chunker/integration_test.go`

```go
func TestIntegration_ConfigDrivenChunking(t *testing.T)
  // Full pipeline: load configs → create registry → chunk files

func TestIntegration_GoFileChunking(t *testing.T)
  // Chunk actual Go file, verify results match legacy chunker

func TestIntegration_JavaFileChunking(t *testing.T)
  // Chunk actual Java file, verify expected chunks

func TestIntegration_PythonFileChunking(t *testing.T)
  // Chunk actual Python file, verify expected chunks

func TestIntegration_TypeScriptFileChunking(t *testing.T)
  // Chunk actual TypeScript file, verify expected chunks

func TestIntegration_AllLanguagesSample(t *testing.T)
  // Sample file for each of 13 languages, verify chunking works
```

---

### 24.7: Remove Legacy Chunkers

After all tests pass, remove the hardcoded language-specific chunkers.

**Files to Delete:**
- `internal/chunker/go.go`
- `internal/chunker/csharp.go`
- `internal/chunker/java.go`
- `internal/chunker/javascript.go`
- `internal/chunker/python.go`
- `internal/chunker/rust.go`

**Note:** Keep test files temporarily for comparison testing, then remove.

#### 24.7.1: Verify All Tests Still Pass

```bash
go test -v ./...
```

---

## Verification Checklist

- [ ] All path resolution tests pass
- [ ] All config parsing tests pass
- [ ] All 13 language config tests pass
- [ ] Generic chunker tests pass
- [ ] Registry tests pass
- [ ] Integration tests pass
- [ ] Legacy chunker files removed
- [ ] No regressions in existing functionality
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes

## Files Changed

### New Files
- `internal/config/paths.go`
- `internal/config/paths_test.go`
- `internal/chunker/langconfig.go`
- `internal/chunker/langconfig_test.go`
- `internal/chunker/languages_test.go`
- `internal/chunker/generic.go`
- `internal/chunker/generic_test.go`
- `internal/chunker/integration_test.go`

### Modified Files
- `internal/chunker/registry.go`
- `internal/chunker/registry_test.go`

### Deleted Files
- `internal/chunker/go.go`
- `internal/chunker/csharp.go`
- `internal/chunker/java.go`
- `internal/chunker/javascript.go`
- `internal/chunker/python.go`
- `internal/chunker/rust.go`
