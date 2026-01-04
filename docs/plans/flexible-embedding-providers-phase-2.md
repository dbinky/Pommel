# Phase 2: Global Config Support

**Parent Design:** [2025-01-04-flexible-embedding-providers-design.md](./2025-01-04-flexible-embedding-providers-design.md)

## Overview

This phase implements global configuration support, allowing users to set their embedding provider once in `~/.config/pommel/config.yaml` and have it apply to all projects. Per-project configs can override global settings.

## Deliverables

1. `internal/config/global.go` - Global config loading from `~/.config/pommel/`
2. `internal/config/loader.go` updates - Config precedence logic
3. `internal/config/validate.go` updates - Provider-specific validation
4. `internal/config/migrate.go` - Legacy config migration

## Implementation Order (TDD)

### Step 1: Global Config Path Resolution

**File:** `internal/config/global.go`

**Tests to write first:**

```go
// global_test.go

// === Happy Path Tests ===

func TestGlobalConfigPath_Default(t *testing.T) {
    // Happy path: returns ~/.config/pommel/config.yaml
    path := GlobalConfigPath()
    homeDir, _ := os.UserHomeDir()
    expected := filepath.Join(homeDir, ".config", "pommel", "config.yaml")
    assert.Equal(t, expected, path)
}

func TestGlobalConfigPath_XDGConfigHome(t *testing.T) {
    // Success scenario: respects XDG_CONFIG_HOME
    t.Setenv("XDG_CONFIG_HOME", "/custom/config")

    path := GlobalConfigPath()
    assert.Equal(t, "/custom/config/pommel/config.yaml", path)
}

func TestGlobalConfigDir(t *testing.T) {
    dir := GlobalConfigDir()
    homeDir, _ := os.UserHomeDir()
    expected := filepath.Join(homeDir, ".config", "pommel")
    assert.Equal(t, expected, dir)
}

// === Edge Case Tests ===

func TestGlobalConfigPath_WindowsStyle(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows-only test")
    }
    // On Windows, should use %APPDATA%\pommel
    path := GlobalConfigPath()
    assert.Contains(t, path, "pommel")
    assert.Contains(t, path, "config.yaml")
}

func TestGlobalConfigPath_EmptyHome(t *testing.T) {
    // Edge case: HOME not set (shouldn't happen but handle gracefully)
    t.Setenv("HOME", "")
    t.Setenv("XDG_CONFIG_HOME", "")

    // Should not panic, may return empty or fallback
    path := GlobalConfigPath()
    assert.NotPanics(t, func() { GlobalConfigPath() })
    _ = path
}
```

**Implementation:**

```go
// global.go

func GlobalConfigDir() string {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, "pommel")
    }

    if runtime.GOOS == "windows" {
        if appData := os.Getenv("APPDATA"); appData != "" {
            return filepath.Join(appData, "pommel")
        }
    }

    homeDir, err := os.UserHomeDir()
    if err != nil || homeDir == "" {
        return ""
    }
    return filepath.Join(homeDir, ".config", "pommel")
}

func GlobalConfigPath() string {
    dir := GlobalConfigDir()
    if dir == "" {
        return ""
    }
    return filepath.Join(dir, "config.yaml")
}
```

---

### Step 2: Global Config Loading

**Tests to write first:**

```go
// global_test.go (continued)

func TestLoadGlobalConfig_Success(t *testing.T) {
    // Happy path: load valid global config
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    configDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(configDir, 0755)

    configContent := `
embedding:
  provider: openai
  openai:
    api_key: sk-test
    model: text-embedding-3-small
`
    os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644)

    cfg, err := LoadGlobalConfig()
    assert.NoError(t, err)
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    assert.Equal(t, "sk-test", cfg.Embedding.OpenAI.APIKey)
}

func TestLoadGlobalConfig_NotExists(t *testing.T) {
    // Success scenario: no global config returns nil (not error)
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cfg, err := LoadGlobalConfig()
    assert.NoError(t, err)
    assert.Nil(t, cfg)
}

func TestLoadGlobalConfig_InvalidYAML(t *testing.T) {
    // Failure scenario: invalid YAML returns error
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    configDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(configDir, 0755)
    os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("invalid: yaml: content:"), 0644)

    _, err := LoadGlobalConfig()
    assert.Error(t, err)
}

func TestLoadGlobalConfig_PermissionDenied(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("Permission test unreliable on Windows")
    }

    // Error scenario: unreadable config file
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    configDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(configDir, 0755)
    configPath := filepath.Join(configDir, "config.yaml")
    os.WriteFile(configPath, []byte("embedding:\n  provider: openai"), 0000)
    defer os.Chmod(configPath, 0644) // Cleanup

    _, err := LoadGlobalConfig()
    assert.Error(t, err)
}

func TestLoadGlobalConfig_EmptyFile(t *testing.T) {
    // Edge case: empty config file
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    configDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(configDir, 0755)
    os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(""), 0644)

    cfg, err := LoadGlobalConfig()
    assert.NoError(t, err)
    assert.Nil(t, cfg) // Empty file treated as no config
}
```

---

### Step 3: Config Precedence (Merge Logic)

**File:** `internal/config/loader.go`

**Tests to write first:**

```go
// loader_test.go (additions)

func TestLoader_MergeConfigs_ProjectOverridesGlobal(t *testing.T) {
    // Happy path: project config overrides global
    global := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI: OpenAIProviderConfig{
                APIKey: "sk-global",
            },
        },
    }

    project := &Config{
        Embedding: EmbeddingConfig{
            Provider: "voyage",
        },
    }

    merged := MergeConfigs(global, project)
    assert.Equal(t, "voyage", merged.Embedding.Provider)
    // API key should still come from global if not overridden
    assert.Equal(t, "sk-global", merged.Embedding.OpenAI.APIKey)
}

func TestLoader_MergeConfigs_GlobalOnly(t *testing.T) {
    // Success scenario: only global config exists
    global := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
        },
    }

    merged := MergeConfigs(global, nil)
    assert.Equal(t, "openai", merged.Embedding.Provider)
}

func TestLoader_MergeConfigs_ProjectOnly(t *testing.T) {
    // Success scenario: only project config exists
    project := &Config{
        Embedding: EmbeddingConfig{
            Provider: "ollama",
        },
    }

    merged := MergeConfigs(nil, project)
    assert.Equal(t, "ollama", merged.Embedding.Provider)
}

func TestLoader_MergeConfigs_NeitherExists(t *testing.T) {
    // Edge case: no configs at all returns defaults
    merged := MergeConfigs(nil, nil)
    assert.NotNil(t, merged)
    // Provider should be empty (not configured)
    assert.Empty(t, merged.Embedding.Provider)
}

func TestLoader_MergeConfigs_PartialProjectOverride(t *testing.T) {
    // Success scenario: project only overrides specific fields
    global := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI: OpenAIProviderConfig{
                APIKey: "sk-global",
                Model:  "text-embedding-3-small",
            },
        },
        Search: SearchConfig{
            DefaultLimit: 10,
        },
    }

    project := &Config{
        Search: SearchConfig{
            DefaultLimit: 20, // Only override search limit
        },
    }

    merged := MergeConfigs(global, project)
    assert.Equal(t, "openai", merged.Embedding.Provider) // From global
    assert.Equal(t, 20, merged.Search.DefaultLimit)      // From project
}

func TestLoader_Load_WithGlobalConfig(t *testing.T) {
    // Integration test: full load with global + project
    tempDir := t.TempDir()
    projectDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Create global config
    globalDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(globalDir, 0755)
    globalCfg := `
embedding:
  provider: openai
  openai:
    api_key: sk-global-key
`
    os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(globalCfg), 0644)

    // Create minimal project config
    pommelDir := filepath.Join(projectDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    projectCfg := `
version: 1
`
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(projectCfg), 0644)

    loader := NewLoader(projectDir)
    cfg, err := loader.Load()

    assert.NoError(t, err)
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    assert.Equal(t, "sk-global-key", cfg.Embedding.OpenAI.APIKey)
}
```

---

### Step 4: Provider-Specific Validation

**File:** `internal/config/validate.go`

**Tests to write first:**

```go
// validate_test.go (additions)

// === Happy Path Tests ===

func TestValidate_OpenAI_ValidConfig(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI: OpenAIProviderConfig{
                APIKey: "sk-test-key",
                Model:  "text-embedding-3-small",
            },
        },
    }
    assert.NoError(t, ValidateConfig(cfg))
}

func TestValidate_Voyage_ValidConfig(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "voyage",
            Voyage: VoyageProviderConfig{
                APIKey: "pa-test-key",
            },
        },
    }
    assert.NoError(t, ValidateConfig(cfg))
}

func TestValidate_Ollama_ValidConfig(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "ollama",
            Ollama: OllamaProviderConfig{
                URL: "http://localhost:11434",
            },
        },
    }
    assert.NoError(t, ValidateConfig(cfg))
}

// === Failure Scenario Tests ===

func TestValidate_OpenAI_MissingAPIKey(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI:   OpenAIProviderConfig{}, // No API key
        },
    }
    err := ValidateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "API key")
}

func TestValidate_Voyage_MissingAPIKey(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "voyage",
            Voyage:   VoyageProviderConfig{},
        },
    }
    err := ValidateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "API key")
}

func TestValidate_OllamaRemote_MissingURL(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider:     "ollama-remote",
            OllamaRemote: OllamaProviderConfig{}, // No URL
        },
    }
    err := ValidateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "URL")
}

func TestValidate_InvalidProvider(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "invalid-provider",
        },
    }
    err := ValidateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown provider")
}

// === Edge Case Tests ===

func TestValidate_EmptyProvider(t *testing.T) {
    // Edge case: no provider configured - this is allowed (not validated)
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "",
        },
    }
    // Empty provider is valid (means "not configured yet")
    assert.NoError(t, ValidateConfig(cfg))
}

func TestValidate_OpenAI_APIKeyFromEnv(t *testing.T) {
    // Success scenario: API key in env var passes validation
    t.Setenv("OPENAI_API_KEY", "sk-from-env")

    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI:   OpenAIProviderConfig{}, // Key will come from env
        },
    }
    assert.NoError(t, ValidateConfig(cfg))
}

func TestValidate_OllamaRemote_InvalidURL(t *testing.T) {
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "ollama-remote",
            OllamaRemote: OllamaProviderConfig{
                URL: "not-a-valid-url",
            },
        },
    }
    err := ValidateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid URL")
}
```

---

### Step 5: Legacy Config Migration

**File:** `internal/config/migrate.go`

**Tests to write first:**

```go
// migrate_test.go

func TestMigrateLegacyConfig_OllamaURL(t *testing.T) {
    // Happy path: old ollama_url format migrates to new structure
    oldCfg := &Config{
        Embedding: EmbeddingConfig{
            Model:     "unclemusclez/jina-embeddings-v2-base-code",
            OllamaURL: "http://localhost:11434", // Legacy field
        },
    }

    migrated := MigrateLegacyConfig(oldCfg)

    assert.Equal(t, "ollama", migrated.Embedding.Provider)
    assert.Equal(t, "http://localhost:11434", migrated.Embedding.Ollama.URL)
    assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", migrated.Embedding.Ollama.Model)
}

func TestMigrateLegacyConfig_RemoteOllama(t *testing.T) {
    // Success scenario: non-localhost URL becomes ollama-remote
    oldCfg := &Config{
        Embedding: EmbeddingConfig{
            OllamaURL: "http://192.168.1.100:11434",
        },
    }

    migrated := MigrateLegacyConfig(oldCfg)

    assert.Equal(t, "ollama-remote", migrated.Embedding.Provider)
    assert.Equal(t, "http://192.168.1.100:11434", migrated.Embedding.OllamaRemote.URL)
}

func TestMigrateLegacyConfig_AlreadyMigrated(t *testing.T) {
    // Edge case: config with provider set doesn't get re-migrated
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider:  "openai",
            OllamaURL: "http://localhost:11434", // Leftover, ignored
        },
    }

    migrated := MigrateLegacyConfig(cfg)

    assert.Equal(t, "openai", migrated.Embedding.Provider)
}

func TestMigrateLegacyConfig_NoLegacyFields(t *testing.T) {
    // Edge case: new config without legacy fields
    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "voyage",
        },
    }

    migrated := MigrateLegacyConfig(cfg)
    assert.Equal(t, cfg, migrated) // No changes
}

func TestMigrateLegacyConfig_NilConfig(t *testing.T) {
    // Edge case: nil config
    assert.Nil(t, MigrateLegacyConfig(nil))
}
```

---

### Step 6: Save Global Config

**Tests to write first:**

```go
// global_test.go (continued)

func TestSaveGlobalConfig_Success(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cfg := &Config{
        Embedding: EmbeddingConfig{
            Provider: "openai",
            OpenAI: OpenAIProviderConfig{
                APIKey: "sk-test",
            },
        },
    }

    err := SaveGlobalConfig(cfg)
    assert.NoError(t, err)

    // Verify file was created
    configPath := filepath.Join(tempDir, "pommel", "config.yaml")
    assert.FileExists(t, configPath)

    // Verify content
    loaded, err := LoadGlobalConfig()
    assert.NoError(t, err)
    assert.Equal(t, "openai", loaded.Embedding.Provider)
}

func TestSaveGlobalConfig_CreatesDirectory(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Directory doesn't exist yet
    configDir := filepath.Join(tempDir, "pommel")
    assert.NoDirExists(t, configDir)

    cfg := &Config{Embedding: EmbeddingConfig{Provider: "openai"}}
    err := SaveGlobalConfig(cfg)

    assert.NoError(t, err)
    assert.DirExists(t, configDir)
}

func TestSaveGlobalConfig_PermissionDenied(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("Permission test unreliable on Windows")
    }

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Create read-only directory
    configDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(configDir, 0555)
    defer os.Chmod(configDir, 0755)

    cfg := &Config{Embedding: EmbeddingConfig{Provider: "openai"}}
    err := SaveGlobalConfig(cfg)

    assert.Error(t, err)
}

func TestSaveGlobalConfig_PreservesOtherSettings(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Create initial config with multiple settings
    initial := &Config{
        Embedding: EmbeddingConfig{Provider: "openai"},
        Search:    SearchConfig{DefaultLimit: 20},
    }
    SaveGlobalConfig(initial)

    // Update only embedding
    updated := &Config{
        Embedding: EmbeddingConfig{Provider: "voyage"},
        Search:    SearchConfig{DefaultLimit: 20}, // Preserved
    }
    SaveGlobalConfig(updated)

    loaded, _ := LoadGlobalConfig()
    assert.Equal(t, "voyage", loaded.Embedding.Provider)
    assert.Equal(t, 20, loaded.Search.DefaultLimit)
}
```

---

## Acceptance Criteria

- [ ] Global config loads from `~/.config/pommel/config.yaml` (or XDG_CONFIG_HOME)
- [ ] Windows uses `%APPDATA%\pommel\config.yaml`
- [ ] Project config overrides global config when both exist
- [ ] Missing global config is not an error
- [ ] Provider-specific validation catches missing API keys
- [ ] Legacy `ollama_url` format migrates automatically
- [ ] `SaveGlobalConfig` creates directory if needed
- [ ] Environment variables work as API key fallback

## Dependencies

- Phase 1 (provider types and config structures)

## Estimated Test Count

- Global path resolution: ~5 tests
- Global config loading: ~6 tests
- Config merge/precedence: ~7 tests
- Provider validation: ~10 tests
- Legacy migration: ~5 tests
- Save global config: ~5 tests

**Total: ~38 tests**
