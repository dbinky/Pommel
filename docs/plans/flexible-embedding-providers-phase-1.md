# Phase 1: Provider Abstraction (Foundation)

**Parent Design:** [2025-01-04-flexible-embedding-providers-design.md](./2025-01-04-flexible-embedding-providers-design.md)

## Overview

This phase establishes the foundation for multiple embedding providers by creating a provider abstraction layer, implementing OpenAI and Voyage AI clients, and adding provider metadata tracking to the database.

## Deliverables

1. `internal/embedder/provider.go` - Provider types and factory function
2. `internal/embedder/openai.go` - OpenAI API client
3. `internal/embedder/voyage.go` - Voyage AI API client
4. `internal/embedder/errors.go` - Shared error types
5. Updated `internal/config/config.go` - New provider config fields
6. Database schema updates for provider metadata

## Implementation Order (TDD)

### Step 1: Provider Types and Factory Interface

**File:** `internal/embedder/provider.go`

**Tests to write first:**

```go
// provider_test.go

func TestProviderType_String(t *testing.T) {
    // Happy path: all provider types have correct string representation
    tests := []struct {
        provider ProviderType
        expected string
    }{
        {ProviderOllama, "ollama"},
        {ProviderOllamaRemote, "ollama-remote"},
        {ProviderOpenAI, "openai"},
        {ProviderVoyage, "voyage"},
    }
    for _, tt := range tests {
        assert.Equal(t, tt.expected, string(tt.provider))
    }
}

func TestProviderType_IsValid(t *testing.T) {
    // Happy path: valid providers
    assert.True(t, ProviderOllama.IsValid())
    assert.True(t, ProviderOpenAI.IsValid())

    // Failure scenario: invalid provider
    assert.False(t, ProviderType("invalid").IsValid())
    assert.False(t, ProviderType("").IsValid())
}

func TestNewFromConfig_UnknownProvider(t *testing.T) {
    // Error scenario: unknown provider type
    cfg := &config.EmbeddingConfig{Provider: "unknown"}
    _, err := NewFromConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown provider")
}

func TestNewFromConfig_EmptyProvider(t *testing.T) {
    // Edge case: empty provider string
    cfg := &config.EmbeddingConfig{Provider: ""}
    _, err := NewFromConfig(cfg)
    assert.Error(t, err)
}
```

**Implementation:**

```go
// provider.go

type ProviderType string

const (
    ProviderOllama       ProviderType = "ollama"
    ProviderOllamaRemote ProviderType = "ollama-remote"
    ProviderOpenAI       ProviderType = "openai"
    ProviderVoyage       ProviderType = "voyage"
)

func (p ProviderType) IsValid() bool {
    switch p {
    case ProviderOllama, ProviderOllamaRemote, ProviderOpenAI, ProviderVoyage:
        return true
    default:
        return false
    }
}

func NewFromConfig(cfg *config.EmbeddingConfig) (Embedder, error) {
    provider := ProviderType(cfg.Provider)
    if !provider.IsValid() {
        return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
    }

    switch provider {
    case ProviderOllama, ProviderOllamaRemote:
        return NewOllamaClient(cfg.OllamaConfig())
    case ProviderOpenAI:
        return NewOpenAIClient(cfg.OpenAIConfig())
    case ProviderVoyage:
        return NewVoyageClient(cfg.VoyageConfig())
    default:
        return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
    }
}
```

---

### Step 2: Shared Error Types

**File:** `internal/embedder/errors.go`

**Tests to write first:**

```go
// errors_test.go

func TestEmbeddingError_Error(t *testing.T) {
    // Happy path: error message formatting
    err := &EmbeddingError{
        Code:       "TEST_ERROR",
        Message:    "Test error message",
        Suggestion: "Try again",
    }
    assert.Contains(t, err.Error(), "Test error message")
    assert.Contains(t, err.Error(), "Try again")
}

func TestEmbeddingError_Error_NoSuggestion(t *testing.T) {
    // Edge case: no suggestion provided
    err := &EmbeddingError{
        Code:    "TEST_ERROR",
        Message: "Test error message",
    }
    assert.Equal(t, "Test error message", err.Error())
}

func TestEmbeddingError_Unwrap(t *testing.T) {
    // Success scenario: unwrap returns cause
    cause := errors.New("underlying error")
    err := &EmbeddingError{
        Code:    "WRAPPED",
        Message: "Wrapped error",
        Cause:   cause,
    }
    assert.Equal(t, cause, errors.Unwrap(err))
}

func TestEmbeddingError_Is(t *testing.T) {
    // Success scenario: errors.Is works correctly
    err := ErrRateLimited
    assert.True(t, errors.Is(err, ErrRateLimited))
    assert.False(t, errors.Is(err, ErrAuthFailed))
}

func TestEmbeddingError_Retryable(t *testing.T) {
    // Happy path: rate limited is retryable
    assert.True(t, ErrRateLimited.Retryable)

    // Failure scenario: auth failed is not retryable
    assert.False(t, ErrAuthFailed.Retryable)
    assert.False(t, ErrQuotaExceeded.Retryable)
}
```

**Implementation:**

```go
// errors.go

type EmbeddingError struct {
    Code       string
    Message    string
    Suggestion string
    Retryable  bool
    RetryAfter time.Duration
    Cause      error
}

func (e *EmbeddingError) Error() string {
    if e.Suggestion == "" {
        return e.Message
    }
    return e.Message + ". " + e.Suggestion
}

func (e *EmbeddingError) Unwrap() error {
    return e.Cause
}

var (
    ErrRateLimited = &EmbeddingError{
        Code:       "RATE_LIMITED",
        Message:    "API rate limit exceeded",
        Suggestion: "Waiting to retry automatically...",
        Retryable:  true,
    }

    ErrAuthFailed = &EmbeddingError{
        Code:       "AUTH_FAILED",
        Message:    "Invalid API key",
        Suggestion: "Run 'pm config provider' to update your API key",
        Retryable:  false,
    }

    ErrQuotaExceeded = &EmbeddingError{
        Code:       "QUOTA_EXCEEDED",
        Message:    "API quota exhausted",
        Suggestion: "Check your billing at the provider's dashboard",
        Retryable:  false,
    }

    ErrProviderUnavailable = &EmbeddingError{
        Code:       "PROVIDER_UNAVAILABLE",
        Message:    "Embedding provider is not responding",
        Suggestion: "Check your network connection and provider status",
        Retryable:  true,
    }
)
```

---

### Step 3: OpenAI Client

**File:** `internal/embedder/openai.go`

**Tests to write first:**

```go
// openai_test.go

// === Happy Path Tests ===

func TestOpenAIClient_EmbedSingle_Success(t *testing.T) {
    // Mock server returns valid embedding
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "/v1/embeddings", r.URL.Path)
        assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
        assert.Contains(t, r.Header.Get("Authorization"), "Bearer sk-test")

        json.NewEncoder(w).Encode(map[string]any{
            "data": []map[string]any{
                {"embedding": make([]float32, 1536)},
            },
        })
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{
        APIKey:  "sk-test",
        Model:   "text-embedding-3-small",
        BaseURL: server.URL,
    })

    embedding, err := client.EmbedSingle(context.Background(), "test text")
    assert.NoError(t, err)
    assert.Len(t, embedding, 1536)
}

func TestOpenAIClient_Embed_MultiplTexts(t *testing.T) {
    // Success scenario: batch embedding
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req map[string]any
        json.NewDecoder(r.Body).Decode(&req)
        inputs := req["input"].([]any)

        data := make([]map[string]any, len(inputs))
        for i := range inputs {
            data[i] = map[string]any{"embedding": make([]float32, 1536)}
        }
        json.NewEncoder(w).Encode(map[string]any{"data": data})
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{
        APIKey:  "sk-test",
        BaseURL: server.URL,
    })

    embeddings, err := client.Embed(context.Background(), []string{"text1", "text2", "text3"})
    assert.NoError(t, err)
    assert.Len(t, embeddings, 3)
}

func TestOpenAIClient_Health_Success(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
    assert.NoError(t, client.Health(context.Background()))
}

// === Failure Scenario Tests ===

func TestOpenAIClient_EmbedSingle_InvalidAPIKey(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]any{
            "error": map[string]any{"message": "Invalid API key"},
        })
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "invalid", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.Equal(t, "AUTH_FAILED", embErr.Code)
    assert.False(t, embErr.Retryable)
}

func TestOpenAIClient_EmbedSingle_RateLimited(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "2")
        w.WriteHeader(http.StatusTooManyRequests)
        json.NewEncoder(w).Encode(map[string]any{
            "error": map[string]any{"message": "Rate limit exceeded"},
        })
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.Equal(t, "RATE_LIMITED", embErr.Code)
    assert.True(t, embErr.Retryable)
    assert.Equal(t, 2*time.Second, embErr.RetryAfter)
}

func TestOpenAIClient_EmbedSingle_QuotaExceeded(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusPaymentRequired)
        json.NewEncoder(w).Encode(map[string]any{
            "error": map[string]any{"message": "Quota exceeded"},
        })
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.Equal(t, "QUOTA_EXCEEDED", embErr.Code)
}

// === Error Scenario Tests ===

func TestOpenAIClient_EmbedSingle_NetworkError(t *testing.T) {
    client := NewOpenAIClient(OpenAIConfig{
        APIKey:  "sk-test",
        BaseURL: "http://localhost:99999", // Invalid port
    })

    _, err := client.EmbedSingle(context.Background(), "test")
    assert.Error(t, err)

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.True(t, embErr.Retryable)
}

func TestOpenAIClient_EmbedSingle_InvalidResponse(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("not json"))
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")
    assert.Error(t, err)
}

func TestOpenAIClient_EmbedSingle_ContextCanceled(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(5 * time.Second) // Slow response
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    _, err := client.EmbedSingle(ctx, "test")
    assert.ErrorIs(t, err, context.Canceled)
}

// === Edge Case Tests ===

func TestOpenAIClient_Embed_EmptyInput(t *testing.T) {
    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test"})

    embeddings, err := client.Embed(context.Background(), []string{})
    assert.NoError(t, err)
    assert.Empty(t, embeddings)
}

func TestOpenAIClient_EmbedSingle_EmptyText(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "data": []map[string]any{
                {"embedding": make([]float32, 1536)},
            },
        })
    }))
    defer server.Close()

    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test", BaseURL: server.URL})
    embedding, err := client.EmbedSingle(context.Background(), "")
    assert.NoError(t, err) // OpenAI accepts empty strings
    assert.Len(t, embedding, 1536)
}

func TestOpenAIClient_ModelName(t *testing.T) {
    client := NewOpenAIClient(OpenAIConfig{Model: "text-embedding-3-small"})
    assert.Equal(t, "text-embedding-3-small", client.ModelName())
}

func TestOpenAIClient_Dimensions(t *testing.T) {
    client := NewOpenAIClient(OpenAIConfig{})
    assert.Equal(t, 1536, client.Dimensions())
}

func TestOpenAIClient_DefaultConfig(t *testing.T) {
    // Edge case: minimal config uses defaults
    client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test"})
    assert.Equal(t, "text-embedding-3-small", client.ModelName())
    assert.Equal(t, "https://api.openai.com", client.baseURL)
}
```

---

### Step 4: Voyage AI Client

**File:** `internal/embedder/voyage.go`

**Tests to write first:**

Similar structure to OpenAI tests, but with Voyage-specific details:

```go
// voyage_test.go

// === Happy Path Tests ===

func TestVoyageClient_EmbedSingle_Success(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "/v1/embeddings", r.URL.Path)
        assert.Contains(t, r.Header.Get("Authorization"), "Bearer pa-test")

        json.NewEncoder(w).Encode(map[string]any{
            "data": []map[string]any{
                {"embedding": make([]float32, 1024)},
            },
        })
    }))
    defer server.Close()

    client := NewVoyageClient(VoyageConfig{
        APIKey:  "pa-test",
        Model:   "voyage-code-3",
        BaseURL: server.URL,
    })

    embedding, err := client.EmbedSingle(context.Background(), "test text")
    assert.NoError(t, err)
    assert.Len(t, embedding, 1024)
}

func TestVoyageClient_Embed_WithInputType(t *testing.T) {
    // Success scenario: Voyage supports input_type parameter
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req map[string]any
        json.NewDecoder(r.Body).Decode(&req)

        // Voyage uses input_type for query vs document
        assert.Equal(t, "document", req["input_type"])

        json.NewEncoder(w).Encode(map[string]any{
            "data": []map[string]any{{"embedding": make([]float32, 1024)}},
        })
    }))
    defer server.Close()

    client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")
    assert.NoError(t, err)
}

// === Failure Scenario Tests ===

func TestVoyageClient_EmbedSingle_InvalidAPIKey(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]any{
            "detail": "Invalid API key",
        })
    }))
    defer server.Close()

    client := NewVoyageClient(VoyageConfig{APIKey: "invalid", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.Equal(t, "AUTH_FAILED", embErr.Code)
}

func TestVoyageClient_EmbedSingle_RateLimited(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "5")
        w.WriteHeader(http.StatusTooManyRequests)
    }))
    defer server.Close()

    client := NewVoyageClient(VoyageConfig{APIKey: "pa-test", BaseURL: server.URL})
    _, err := client.EmbedSingle(context.Background(), "test")

    var embErr *EmbeddingError
    assert.True(t, errors.As(err, &embErr))
    assert.True(t, embErr.Retryable)
}

// === Edge Case Tests ===

func TestVoyageClient_Dimensions(t *testing.T) {
    client := NewVoyageClient(VoyageConfig{})
    assert.Equal(t, 1024, client.Dimensions()) // voyage-code-3 is 1024 dims
}

func TestVoyageClient_DefaultModel(t *testing.T) {
    client := NewVoyageClient(VoyageConfig{APIKey: "pa-test"})
    assert.Equal(t, "voyage-code-3", client.ModelName())
}
```

---

### Step 5: Config Updates

**File:** `internal/config/config.go`

**Tests to write first:**

```go
// config_test.go (additions)

func TestEmbeddingConfig_OllamaConfig(t *testing.T) {
    cfg := &EmbeddingConfig{
        Provider: "ollama",
        Ollama: OllamaProviderConfig{
            URL:   "http://localhost:11434",
            Model: "test-model",
        },
    }

    ollamaCfg := cfg.OllamaConfig()
    assert.Equal(t, "http://localhost:11434", ollamaCfg.BaseURL)
    assert.Equal(t, "test-model", ollamaCfg.Model)
}

func TestEmbeddingConfig_OpenAIConfig(t *testing.T) {
    cfg := &EmbeddingConfig{
        Provider: "openai",
        OpenAI: OpenAIProviderConfig{
            APIKey: "sk-test",
            Model:  "text-embedding-3-small",
        },
    }

    openaiCfg := cfg.OpenAIConfig()
    assert.Equal(t, "sk-test", openaiCfg.APIKey)
    assert.Equal(t, "text-embedding-3-small", openaiCfg.Model)
}

func TestEmbeddingConfig_OpenAIConfig_EnvFallback(t *testing.T) {
    // Edge case: API key from environment variable
    t.Setenv("OPENAI_API_KEY", "sk-from-env")

    cfg := &EmbeddingConfig{
        Provider: "openai",
        OpenAI:   OpenAIProviderConfig{}, // No API key in config
    }

    openaiCfg := cfg.OpenAIConfig()
    assert.Equal(t, "sk-from-env", openaiCfg.APIKey)
}

func TestEmbeddingConfig_VoyageConfig_EnvFallback(t *testing.T) {
    t.Setenv("VOYAGE_API_KEY", "pa-from-env")

    cfg := &EmbeddingConfig{
        Provider: "voyage",
        Voyage:   VoyageProviderConfig{},
    }

    voyageCfg := cfg.VoyageConfig()
    assert.Equal(t, "pa-from-env", voyageCfg.APIKey)
}
```

---

### Step 6: Database Metadata

**File:** `internal/db/metadata.go`

**Tests to write first:**

```go
// metadata_test.go

func TestDB_SetMetadata(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    err := db.SetMetadata("provider", "openai")
    assert.NoError(t, err)

    value, err := db.GetMetadata("provider")
    assert.NoError(t, err)
    assert.Equal(t, "openai", value)
}

func TestDB_GetMetadata_NotFound(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    value, err := db.GetMetadata("nonexistent")
    assert.NoError(t, err)
    assert.Empty(t, value)
}

func TestDB_SetMetadata_Update(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    db.SetMetadata("provider", "ollama")
    db.SetMetadata("provider", "openai")

    value, _ := db.GetMetadata("provider")
    assert.Equal(t, "openai", value)
}

func TestDB_GetProviderInfo(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    db.SetMetadata("provider", "openai")
    db.SetMetadata("model", "text-embedding-3-small")
    db.SetMetadata("dimensions", "1536")

    info, err := db.GetProviderInfo()
    assert.NoError(t, err)
    assert.Equal(t, "openai", info.Provider)
    assert.Equal(t, "text-embedding-3-small", info.Model)
    assert.Equal(t, 1536, info.Dimensions)
}
```

---

## Acceptance Criteria

- [ ] All tests pass (`go test ./internal/embedder/...`)
- [ ] `NewFromConfig` correctly creates Ollama, OpenAI, and Voyage clients
- [ ] OpenAI client handles all error responses correctly
- [ ] Voyage client handles all error responses correctly
- [ ] Config supports environment variable fallback for API keys
- [ ] Database can store and retrieve provider metadata
- [ ] No regressions in existing Ollama functionality

## Dependencies

- None (this phase is foundational)

## Estimated Test Count

- Provider types/factory: ~8 tests
- Error types: ~6 tests
- OpenAI client: ~15 tests
- Voyage client: ~12 tests
- Config additions: ~6 tests
- Database metadata: ~5 tests

**Total: ~52 tests**
