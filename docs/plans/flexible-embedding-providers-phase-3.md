# Phase 3: CLI Commands

**Parent Design:** [2025-01-04-flexible-embedding-providers-design.md](./2025-01-04-flexible-embedding-providers-design.md)

## Overview

This phase implements the CLI commands for provider management and updates existing commands to handle the new provider configuration requirements.

## Deliverables

1. `internal/cli/config_provider.go` - New `pm config provider` command
2. `internal/cli/init.go` updates - Provider not configured warning
3. `internal/cli/start.go` updates - Fail clearly without provider
4. `internal/cli/search.go` updates - Fail clearly without provider
5. `internal/cli/common.go` - Shared provider check logic

## Implementation Order (TDD)

### Step 1: Provider Check Utility

**File:** `internal/cli/common.go`

**Tests to write first:**

```go
// common_test.go

// === Happy Path Tests ===

func TestCheckProviderConfigured_Ollama(t *testing.T) {
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "ollama",
            Ollama: config.OllamaProviderConfig{
                URL: "http://localhost:11434",
            },
        },
    }
    err := CheckProviderConfigured(cfg)
    assert.NoError(t, err)
}

func TestCheckProviderConfigured_OpenAI(t *testing.T) {
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "openai",
            OpenAI: config.OpenAIProviderConfig{
                APIKey: "sk-test",
            },
        },
    }
    err := CheckProviderConfigured(cfg)
    assert.NoError(t, err)
}

func TestCheckProviderConfigured_OpenAI_EnvKey(t *testing.T) {
    t.Setenv("OPENAI_API_KEY", "sk-from-env")

    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "openai",
            OpenAI:   config.OpenAIProviderConfig{}, // Key from env
        },
    }
    err := CheckProviderConfigured(cfg)
    assert.NoError(t, err)
}

// === Failure Scenario Tests ===

func TestCheckProviderConfigured_NotConfigured(t *testing.T) {
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "", // Empty
        },
    }
    err := CheckProviderConfigured(cfg)
    assert.Error(t, err)

    var provErr *ProviderNotConfiguredError
    assert.True(t, errors.As(err, &provErr))
}

func TestCheckProviderConfigured_OpenAI_NoKey(t *testing.T) {
    // Clear env var if set
    t.Setenv("OPENAI_API_KEY", "")

    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "openai",
            OpenAI:   config.OpenAIProviderConfig{}, // No key
        },
    }
    err := CheckProviderConfigured(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "API key")
}

// === Edge Case Tests ===

func TestCheckProviderConfigured_NilConfig(t *testing.T) {
    err := CheckProviderConfigured(nil)
    assert.Error(t, err)
}

func TestProviderNotConfiguredError_Message(t *testing.T) {
    err := &ProviderNotConfiguredError{}
    msg := err.Error()
    assert.Contains(t, msg, "No embedding provider configured")
    assert.Contains(t, msg, "pm config provider")
}
```

**Implementation:**

```go
// common.go

type ProviderNotConfiguredError struct{}

func (e *ProviderNotConfiguredError) Error() string {
    return `No embedding provider configured

  Run 'pm config provider' to set up embeddings.`
}

func CheckProviderConfigured(cfg *config.Config) error {
    if cfg == nil {
        return &ProviderNotConfiguredError{}
    }

    if cfg.Embedding.Provider == "" {
        return &ProviderNotConfiguredError{}
    }

    // Check provider-specific requirements
    switch cfg.Embedding.Provider {
    case "openai":
        if cfg.Embedding.OpenAI.APIKey == "" && os.Getenv("OPENAI_API_KEY") == "" {
            return fmt.Errorf("OpenAI provider requires API key. Set openai.api_key in config or OPENAI_API_KEY environment variable")
        }
    case "voyage":
        if cfg.Embedding.Voyage.APIKey == "" && os.Getenv("VOYAGE_API_KEY") == "" {
            return fmt.Errorf("Voyage provider requires API key. Set voyage.api_key in config or VOYAGE_API_KEY environment variable")
        }
    case "ollama-remote":
        if cfg.Embedding.OllamaRemote.URL == "" {
            return fmt.Errorf("Remote Ollama provider requires URL. Set ollama-remote.url in config")
        }
    }

    return nil
}
```

---

### Step 2: `pm config provider` Command (Interactive)

**File:** `internal/cli/config_provider.go`

**Tests to write first:**

```go
// config_provider_test.go

// === Happy Path Tests ===

func TestConfigProviderCmd_Interactive_Ollama(t *testing.T) {
    // Simulate user selecting option 1 (Local Ollama)
    input := strings.NewReader("1\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    // Verify config was saved
    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "ollama", cfg.Embedding.Provider)

    // Verify output
    assert.Contains(t, output.String(), "Local Ollama")
    assert.Contains(t, output.String(), "Ready!")
}

func TestConfigProviderCmd_Interactive_OpenAI(t *testing.T) {
    // Simulate: select OpenAI (3), enter API key
    input := strings.NewReader("3\nsk-test-key-12345\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Mock API validation to succeed
    mockValidator := &mockAPIValidator{valid: true}

    cmd := NewConfigProviderCmdWithValidator(mockValidator)
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    assert.Equal(t, "sk-test-key-12345", cfg.Embedding.OpenAI.APIKey)

    assert.Contains(t, output.String(), "API key validated")
}

func TestConfigProviderCmd_Interactive_OpenAI_SkipKey(t *testing.T) {
    // Simulate: select OpenAI, leave key blank
    input := strings.NewReader("3\n\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    assert.Empty(t, cfg.Embedding.OpenAI.APIKey)

    assert.Contains(t, output.String(), "configure later")
}

func TestConfigProviderCmd_Interactive_RemoteOllama(t *testing.T) {
    // Simulate: select Remote Ollama (2), enter URL
    input := strings.NewReader("2\nhttp://192.168.1.100:11434\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
    assert.Equal(t, "http://192.168.1.100:11434", cfg.Embedding.OllamaRemote.URL)
}

// === Failure Scenario Tests ===

func TestConfigProviderCmd_Interactive_InvalidAPIKey(t *testing.T) {
    input := strings.NewReader("3\ninvalid-key\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    mockValidator := &mockAPIValidator{valid: false}

    cmd := NewConfigProviderCmdWithValidator(mockValidator)
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err) // Command succeeds but warns

    assert.Contains(t, output.String(), "Invalid API key")
    assert.Contains(t, output.String(), "configure later")
}

func TestConfigProviderCmd_Interactive_InvalidChoice(t *testing.T) {
    // Invalid choice, then valid choice
    input := strings.NewReader("99\n1\n")
    output := &bytes.Buffer{}

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetIn(input)
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    assert.Contains(t, output.String(), "Invalid choice")
}

// === Edge Case Tests ===

func TestConfigProviderCmd_ShowsCurrent(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Pre-create config
    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{
            Provider: "openai",
        },
    }
    config.SaveGlobalConfig(cfg)

    input := strings.NewReader("1\n") // Switch to Ollama
    output := &bytes.Buffer{}

    cmd := NewConfigProviderCmd()
    cmd.SetIn(input)
    cmd.SetOut(output)
    cmd.Execute()

    assert.Contains(t, output.String(), "Current provider: openai")
}

func TestConfigProviderCmd_ProviderChangeWarning(t *testing.T) {
    // When changing providers, warn about reindexing
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cfg := &config.Config{
        Embedding: config.EmbeddingConfig{Provider: "ollama"},
    }
    config.SaveGlobalConfig(cfg)

    input := strings.NewReader("3\nsk-test\n") // Switch to OpenAI
    output := &bytes.Buffer{}

    mockValidator := &mockAPIValidator{valid: true}
    cmd := NewConfigProviderCmdWithValidator(mockValidator)
    cmd.SetIn(input)
    cmd.SetOut(output)
    cmd.Execute()

    assert.Contains(t, output.String(), "reindex")
}
```

---

### Step 3: `pm config provider <name>` Direct Mode

**Tests to write first:**

```go
// config_provider_test.go (continued)

func TestConfigProviderCmd_Direct_Ollama(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetArgs([]string{"ollama"})

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "ollama", cfg.Embedding.Provider)
}

func TestConfigProviderCmd_Direct_OpenAI_WithKey(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    output := &bytes.Buffer{}

    mockValidator := &mockAPIValidator{valid: true}
    cmd := NewConfigProviderCmdWithValidator(mockValidator)
    cmd.SetArgs([]string{"openai", "--api-key", "sk-test"})
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    assert.Equal(t, "sk-test", cfg.Embedding.OpenAI.APIKey)
}

func TestConfigProviderCmd_Direct_OllamaRemote_WithURL(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    cmd := NewConfigProviderCmd()
    cmd.SetArgs([]string{"ollama-remote", "--url", "http://192.168.1.100:11434"})

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
    assert.Equal(t, "http://192.168.1.100:11434", cfg.Embedding.OllamaRemote.URL)
}

func TestConfigProviderCmd_Direct_InvalidProvider(t *testing.T) {
    cmd := NewConfigProviderCmd()
    cmd.SetArgs([]string{"invalid-provider"})

    err := cmd.Execute()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown provider")
}

func TestConfigProviderCmd_Direct_OpenAI_MissingKey(t *testing.T) {
    t.Setenv("OPENAI_API_KEY", "") // Ensure no env key

    cmd := NewConfigProviderCmd()
    cmd.SetArgs([]string{"openai"}) // No --api-key flag

    err := cmd.Execute()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "API key required")
}

func TestConfigProviderCmd_Direct_OpenAI_EnvKey(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)
    t.Setenv("OPENAI_API_KEY", "sk-from-env")

    cmd := NewConfigProviderCmd()
    cmd.SetArgs([]string{"openai"}) // Key from env

    err := cmd.Execute()
    assert.NoError(t, err)

    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "openai", cfg.Embedding.Provider)
    // API key not stored in config (uses env)
    assert.Empty(t, cfg.Embedding.OpenAI.APIKey)
}
```

---

### Step 4: Update `pm init` with Provider Warning

**File:** `internal/cli/init.go`

**Tests to write first:**

```go
// init_test.go (additions)

func TestInitCmd_WarnsNoProvider(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // No global config

    output := &bytes.Buffer{}

    cmd := NewInitCmd()
    cmd.SetArgs([]string{"--project", tempDir})
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    // Verify warning is shown
    out := output.String()
    assert.Contains(t, out, "No embedding provider configured")
    assert.Contains(t, out, "pm config provider")
    assert.Contains(t, out, "pm start")
    assert.Contains(t, out, "will not work")
}

func TestInitCmd_NoWarningWithProvider(t *testing.T) {
    tempDir := t.TempDir()
    globalDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", globalDir)

    // Pre-configure provider
    os.MkdirAll(filepath.Join(globalDir, "pommel"), 0755)
    cfg := `
embedding:
  provider: ollama
`
    os.WriteFile(filepath.Join(globalDir, "pommel", "config.yaml"), []byte(cfg), 0644)

    output := &bytes.Buffer{}

    cmd := NewInitCmd()
    cmd.SetArgs([]string{"--project", tempDir})
    cmd.SetOut(output)

    err := cmd.Execute()
    assert.NoError(t, err)

    // No warning
    out := output.String()
    assert.NotContains(t, out, "No embedding provider configured")
}

func TestInitCmd_SuccessMessageWithProvider(t *testing.T) {
    tempDir := t.TempDir()
    globalDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", globalDir)

    // Pre-configure provider
    os.MkdirAll(filepath.Join(globalDir, "pommel"), 0755)
    cfg := `embedding: { provider: openai, openai: { api_key: sk-test } }`
    os.WriteFile(filepath.Join(globalDir, "pommel", "config.yaml"), []byte(cfg), 0644)

    output := &bytes.Buffer{}

    cmd := NewInitCmd()
    cmd.SetArgs([]string{"--project", tempDir})
    cmd.SetOut(output)
    cmd.Execute()

    assert.Contains(t, output.String(), "pm start")
    assert.NotContains(t, output.String(), "will not work")
}
```

---

### Step 5: Update `pm start` to Fail Without Provider

**File:** `internal/cli/start.go`

**Tests to write first:**

```go
// start_test.go (additions)

func TestStartCmd_FailsWithoutProvider(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // Empty global config

    // Create minimal project
    pommelDir := filepath.Join(tempDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte("version: 1"), 0644)

    output := &bytes.Buffer{}
    errOutput := &bytes.Buffer{}

    cmd := NewStartCmd()
    cmd.SetArgs([]string{"--project", tempDir})
    cmd.SetOut(output)
    cmd.SetErr(errOutput)

    err := cmd.Execute()
    assert.Error(t, err)

    combined := output.String() + errOutput.String()
    assert.Contains(t, combined, "No embedding provider configured")
    assert.Contains(t, combined, "pm config provider")
}

func TestStartCmd_SucceedsWithProvider(t *testing.T) {
    tempDir := t.TempDir()
    globalDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", globalDir)

    // Configure provider
    os.MkdirAll(filepath.Join(globalDir, "pommel"), 0755)
    cfg := `embedding: { provider: ollama }`
    os.WriteFile(filepath.Join(globalDir, "pommel", "config.yaml"), []byte(cfg), 0644)

    // Create project
    pommelDir := filepath.Join(tempDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte("version: 1"), 0644)

    // Note: Full integration test would need mock daemon
    // This tests the provider check passes
    cmd := NewStartCmd()
    cmd.SetArgs([]string{"--project", tempDir})

    // Provider check should pass (daemon start may fail for other reasons)
    // We're testing the provider check specifically
}

func TestStartCmd_ReindexIfNeeded_Flag(t *testing.T) {
    cmd := NewStartCmd()

    // Verify flag exists
    flag := cmd.Flags().Lookup("reindex-if-needed")
    assert.NotNil(t, flag)
    assert.Equal(t, "false", flag.DefValue)
}
```

---

### Step 6: Update `pm search` to Fail Without Provider

**File:** `internal/cli/search.go`

**Tests to write first:**

```go
// search_test.go (additions)

func TestSearchCmd_FailsWithoutProvider(t *testing.T) {
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // Empty

    pommelDir := filepath.Join(tempDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte("version: 1"), 0644)

    output := &bytes.Buffer{}
    errOutput := &bytes.Buffer{}

    cmd := NewSearchCmd()
    cmd.SetArgs([]string{"test query", "--project", tempDir})
    cmd.SetOut(output)
    cmd.SetErr(errOutput)

    err := cmd.Execute()
    assert.Error(t, err)

    combined := output.String() + errOutput.String()
    assert.Contains(t, combined, "No embedding provider configured")
}

func TestSearchCmd_ProviderCheckBeforeDaemonConnect(t *testing.T) {
    // Ensure provider check happens before trying to connect to daemon
    // This gives a better error message than "connection refused"

    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", t.TempDir())

    pommelDir := filepath.Join(tempDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte("version: 1"), 0644)

    errOutput := &bytes.Buffer{}

    cmd := NewSearchCmd()
    cmd.SetArgs([]string{"test", "--project", tempDir})
    cmd.SetErr(errOutput)

    err := cmd.Execute()

    // Should fail with provider error, not connection error
    assert.Error(t, err)
    assert.Contains(t, errOutput.String(), "provider")
    assert.NotContains(t, errOutput.String(), "connection refused")
}
```

---

## Acceptance Criteria

- [ ] `pm config provider` interactive mode works for all 4 providers
- [ ] `pm config provider <name>` direct mode works with flags
- [ ] API key validation happens before saving config
- [ ] Invalid API keys show warning but don't block
- [ ] `pm init` shows clear warning when provider not configured
- [ ] `pm start` fails with helpful message when no provider
- [ ] `pm search` fails with helpful message when no provider
- [ ] Provider change shows reindex warning
- [ ] Current provider displayed when running `pm config provider`

## Dependencies

- Phase 1 (provider types)
- Phase 2 (global config support)

## Estimated Test Count

- Provider check utility: ~7 tests
- `pm config provider` interactive: ~10 tests
- `pm config provider` direct mode: ~7 tests
- `pm init` updates: ~4 tests
- `pm start` updates: ~4 tests
- `pm search` updates: ~3 tests

**Total: ~35 tests**
