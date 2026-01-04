# Phase 5: Polish

**Parent Design:** [2025-01-04-flexible-embedding-providers-design.md](./2025-01-04-flexible-embedding-providers-design.md)

## Overview

This phase adds polish and robustness: adaptive ETA calculation based on observed throughput, automatic retry with backoff for rate limits, migration prompts for upgrading users, and documentation updates.

## Deliverables

1. `internal/daemon/progress.go` - Adaptive ETA calculation
2. `internal/embedder/retry.go` - Retry logic with backoff
3. `internal/cli/migrate.go` - Migration prompts for upgrading users
4. Updated `README.md` - Provider documentation
5. Updated `CLAUDE.md` - Agent instructions for providers

## Implementation Order (TDD)

### Step 1: Adaptive ETA Calculation

**File:** `internal/daemon/progress.go`

**Tests to write first:**

```go
// progress_test.go

// === Happy Path Tests ===

func TestIndexProgress_ETA_SteadyRate(t *testing.T) {
    // Happy path: consistent batch timing gives accurate ETA
    p := NewIndexProgress(1000)

    // Simulate 10 batches of 10 chunks, each taking 1 second
    for i := 0; i < 10; i++ {
        p.RecordBatch(10, 1*time.Second)
    }

    // 100 done, 900 remaining, at 10/sec = 90 seconds
    eta := p.ETA()
    assert.InDelta(t, 90*time.Second, eta, float64(5*time.Second))
}

func TestIndexProgress_ETA_VariableRate(t *testing.T) {
    // Success scenario: variable timing uses recent average
    p := NewIndexProgress(1000)

    // Older batches: slow (ignored in rolling window)
    for i := 0; i < 5; i++ {
        p.RecordBatch(10, 5*time.Second) // 2 chunks/sec
    }

    // Recent batches: fast
    for i := 0; i < 10; i++ {
        p.RecordBatch(10, 500*time.Millisecond) // 20 chunks/sec
    }

    // Should use recent rate (~20/sec), not overall average
    eta := p.ETA()
    remaining := 1000 - 150 // 850 chunks
    expectedETA := time.Duration(float64(remaining) / 20.0 * float64(time.Second))

    assert.InDelta(t, expectedETA, eta, float64(10*time.Second))
}

func TestIndexProgress_Rate(t *testing.T) {
    p := NewIndexProgress(100)

    p.RecordBatch(10, 1*time.Second)
    p.RecordBatch(10, 1*time.Second)

    rate := p.Rate()
    assert.InDelta(t, 10.0, rate, 0.5) // ~10 chunks/sec
}

func TestIndexProgress_Percentage(t *testing.T) {
    p := NewIndexProgress(200)

    p.RecordBatch(50, time.Second)

    assert.Equal(t, 25.0, p.Percentage())
}

// === Edge Case Tests ===

func TestIndexProgress_ETA_NoData(t *testing.T) {
    // Edge case: no batches recorded yet
    p := NewIndexProgress(100)

    eta := p.ETA()
    assert.Equal(t, time.Duration(0), eta) // Calculating...
}

func TestIndexProgress_ETA_OneBatch(t *testing.T) {
    // Edge case: only one batch (not enough for average)
    p := NewIndexProgress(100)
    p.RecordBatch(10, time.Second)

    eta := p.ETA()
    assert.Equal(t, time.Duration(0), eta) // Need more data
}

func TestIndexProgress_ETA_Complete(t *testing.T) {
    // Edge case: all chunks done
    p := NewIndexProgress(100)

    for i := 0; i < 10; i++ {
        p.RecordBatch(10, time.Second)
    }

    eta := p.ETA()
    assert.Equal(t, time.Duration(0), eta) // Nothing remaining
}

func TestIndexProgress_ETA_ZeroTotal(t *testing.T) {
    // Edge case: zero total chunks
    p := NewIndexProgress(0)

    eta := p.ETA()
    assert.Equal(t, time.Duration(0), eta)
}

func TestIndexProgress_RollingWindow_Size(t *testing.T) {
    // Edge case: rolling window limits data points
    p := NewIndexProgress(10000)

    // Record more batches than window size
    for i := 0; i < 50; i++ {
        p.RecordBatch(10, time.Second)
    }

    // Internal window should be capped
    assert.LessOrEqual(t, len(p.recentBatches), 10)
}

// === Failure Scenario Tests ===

func TestIndexProgress_RecordBatch_ZeroDuration(t *testing.T) {
    // Failure scenario: zero duration batch (shouldn't panic)
    p := NewIndexProgress(100)

    assert.NotPanics(t, func() {
        p.RecordBatch(10, 0)
    })

    // Should still track chunks
    assert.Equal(t, 10, p.CompletedChunks)
}

func TestIndexProgress_RecordBatch_Negative(t *testing.T) {
    // Failure scenario: negative values (shouldn't happen but handle gracefully)
    p := NewIndexProgress(100)

    assert.NotPanics(t, func() {
        p.RecordBatch(-5, time.Second)
    })
}
```

**Implementation:**

```go
// progress.go

const rollingWindowSize = 10

type BatchTiming struct {
    Chunks   int
    Duration time.Duration
}

type IndexProgress struct {
    TotalChunks     int
    CompletedChunks int
    StartTime       time.Time
    recentBatches   []BatchTiming

    mu sync.RWMutex
}

func NewIndexProgress(totalChunks int) *IndexProgress {
    return &IndexProgress{
        TotalChunks:   totalChunks,
        StartTime:     time.Now(),
        recentBatches: make([]BatchTiming, 0, rollingWindowSize),
    }
}

func (p *IndexProgress) RecordBatch(chunks int, duration time.Duration) {
    if chunks <= 0 || duration <= 0 {
        // Still count chunks even if timing is invalid
        p.mu.Lock()
        if chunks > 0 {
            p.CompletedChunks += chunks
        }
        p.mu.Unlock()
        return
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    p.CompletedChunks += chunks

    p.recentBatches = append(p.recentBatches, BatchTiming{
        Chunks:   chunks,
        Duration: duration,
    })

    // Keep only recent batches
    if len(p.recentBatches) > rollingWindowSize {
        p.recentBatches = p.recentBatches[len(p.recentBatches)-rollingWindowSize:]
    }
}

func (p *IndexProgress) ETA() time.Duration {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if p.TotalChunks == 0 {
        return 0
    }

    remaining := p.TotalChunks - p.CompletedChunks
    if remaining <= 0 {
        return 0
    }

    // Need at least 2 data points for meaningful estimate
    if len(p.recentBatches) < 2 {
        return 0
    }

    // Calculate rate from recent batches
    var totalChunks int
    var totalDuration time.Duration
    for _, b := range p.recentBatches {
        totalChunks += b.Chunks
        totalDuration += b.Duration
    }

    if totalDuration == 0 {
        return 0
    }

    chunksPerSec := float64(totalChunks) / totalDuration.Seconds()
    if chunksPerSec <= 0 {
        return 0
    }

    return time.Duration(float64(remaining)/chunksPerSec) * time.Second
}

func (p *IndexProgress) Rate() float64 {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if len(p.recentBatches) == 0 {
        return 0
    }

    var totalChunks int
    var totalDuration time.Duration
    for _, b := range p.recentBatches {
        totalChunks += b.Chunks
        totalDuration += b.Duration
    }

    if totalDuration == 0 {
        return 0
    }

    return float64(totalChunks) / totalDuration.Seconds()
}

func (p *IndexProgress) Percentage() float64 {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if p.TotalChunks == 0 {
        return 100.0
    }

    return float64(p.CompletedChunks) / float64(p.TotalChunks) * 100.0
}
```

---

### Step 2: Retry Logic with Backoff

**File:** `internal/embedder/retry.go`

**Tests to write first:**

```go
// retry_test.go

// === Happy Path Tests ===

func TestRetry_SucceedsFirstTry(t *testing.T) {
    attempts := 0
    fn := func() error {
        attempts++
        return nil
    }

    err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

    assert.NoError(t, err)
    assert.Equal(t, 1, attempts)
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
    attempts := 0
    fn := func() error {
        attempts++
        if attempts < 3 {
            return &EmbeddingError{Retryable: true}
        }
        return nil
    }

    err := WithRetry(context.Background(), fn, RetryConfig{
        MaxRetries:  5,
        BaseBackoff: 10 * time.Millisecond,
    })

    assert.NoError(t, err)
    assert.Equal(t, 3, attempts)
}

func TestRetry_RespectsRetryAfter(t *testing.T) {
    start := time.Now()
    attempts := 0

    fn := func() error {
        attempts++
        if attempts == 1 {
            return &EmbeddingError{
                Retryable:  true,
                RetryAfter: 100 * time.Millisecond,
            }
        }
        return nil
    }

    WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

    elapsed := time.Since(start)
    assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

// === Failure Scenario Tests ===

func TestRetry_NonRetryableError(t *testing.T) {
    attempts := 0
    fn := func() error {
        attempts++
        return &EmbeddingError{
            Code:      "AUTH_FAILED",
            Retryable: false,
        }
    }

    err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 5})

    assert.Error(t, err)
    assert.Equal(t, 1, attempts) // No retries for non-retryable
}

func TestRetry_MaxRetriesExceeded(t *testing.T) {
    attempts := 0
    fn := func() error {
        attempts++
        return &EmbeddingError{Retryable: true}
    }

    err := WithRetry(context.Background(), fn, RetryConfig{
        MaxRetries:  3,
        BaseBackoff: 10 * time.Millisecond,
    })

    assert.Error(t, err)
    assert.Equal(t, 4, attempts) // Initial + 3 retries
    assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestRetry_ContextCanceled(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    attempts := 0
    fn := func() error {
        attempts++
        if attempts == 2 {
            cancel()
        }
        return &EmbeddingError{Retryable: true}
    }

    err := WithRetry(ctx, fn, RetryConfig{
        MaxRetries:  10,
        BaseBackoff: 10 * time.Millisecond,
    })

    assert.ErrorIs(t, err, context.Canceled)
    assert.LessOrEqual(t, attempts, 3)
}

func TestRetry_ContextTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    fn := func() error {
        return &EmbeddingError{Retryable: true}
    }

    err := WithRetry(ctx, fn, RetryConfig{
        MaxRetries:  100,
        BaseBackoff: 100 * time.Millisecond, // Longer than timeout
    })

    assert.Error(t, err)
}

// === Edge Case Tests ===

func TestRetry_ExponentialBackoff(t *testing.T) {
    var backoffs []time.Duration
    lastCall := time.Now()

    attempts := 0
    fn := func() error {
        attempts++
        now := time.Now()
        if attempts > 1 {
            backoffs = append(backoffs, now.Sub(lastCall))
        }
        lastCall = now
        if attempts < 4 {
            return &EmbeddingError{Retryable: true}
        }
        return nil
    }

    WithRetry(context.Background(), fn, RetryConfig{
        MaxRetries:  5,
        BaseBackoff: 20 * time.Millisecond,
    })

    // Verify backoffs increase (exponential)
    for i := 1; i < len(backoffs); i++ {
        assert.Greater(t, backoffs[i], backoffs[i-1]*time.Duration(0.8)) // Allow some variance
    }
}

func TestRetry_RegularErrorWrapped(t *testing.T) {
    fn := func() error {
        return errors.New("regular error")
    }

    err := WithRetry(context.Background(), fn, RetryConfig{MaxRetries: 3})

    // Regular errors are not retried
    assert.Error(t, err)
}

func TestRetry_NilConfig(t *testing.T) {
    fn := func() error { return nil }

    // Should use defaults
    assert.NotPanics(t, func() {
        WithRetry(context.Background(), fn, RetryConfig{})
    })
}
```

**Implementation:**

```go
// retry.go

type RetryConfig struct {
    MaxRetries  int
    BaseBackoff time.Duration
    MaxBackoff  time.Duration
}

func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries:  3,
        BaseBackoff: 1 * time.Second,
        MaxBackoff:  30 * time.Second,
    }
}

func WithRetry(ctx context.Context, fn func() error, cfg RetryConfig) error {
    if cfg.MaxRetries == 0 {
        cfg = DefaultRetryConfig()
    }

    var lastErr error

    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        // Check context before each attempt
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        var embErr *EmbeddingError
        if !errors.As(err, &embErr) || !embErr.Retryable {
            return err // Non-retryable, return immediately
        }

        if attempt == cfg.MaxRetries {
            break // No more retries
        }

        // Calculate backoff
        backoff := embErr.RetryAfter
        if backoff == 0 {
            // Exponential backoff: base * 2^attempt
            backoff = cfg.BaseBackoff * time.Duration(1<<attempt)
            if cfg.MaxBackoff > 0 && backoff > cfg.MaxBackoff {
                backoff = cfg.MaxBackoff
            }
        }

        // Wait with context cancellation support
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

---

### Step 3: Migration Prompts for Upgrading Users

**File:** `internal/cli/migrate.go`

**Tests to write first:**

```go
// migrate_test.go

func TestShowMigrationPrompt_NoExistingConfig(t *testing.T) {
    // Happy path: new user, no migration needed
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    output := &bytes.Buffer{}
    input := strings.NewReader("")

    needed := ShowMigrationPromptIfNeeded(input, output)

    assert.False(t, needed)
    assert.Empty(t, output.String())
}

func TestShowMigrationPrompt_LegacyConfig(t *testing.T) {
    // Happy path: user has legacy config, shows prompt
    tempDir := t.TempDir()
    projectDir := t.TempDir()

    // Create legacy project config
    pommelDir := filepath.Join(projectDir, ".pommel")
    os.MkdirAll(pommelDir, 0755)
    legacyCfg := `
embedding:
  model: "unclemusclez/jina-embeddings-v2-base-code"
  ollama_url: "http://localhost:11434"
`
    os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(legacyCfg), 0644)

    output := &bytes.Buffer{}
    input := strings.NewReader("1\n") // Keep using Ollama

    t.Setenv("XDG_CONFIG_HOME", tempDir)

    ShowMigrationPromptIfNeeded(input, output)

    assert.Contains(t, output.String(), "multiple embedding providers")
    assert.Contains(t, output.String(), "Keep using local Ollama")
}

func TestShowMigrationPrompt_KeepOllama(t *testing.T) {
    // User chooses to keep Ollama
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    output := &bytes.Buffer{}
    input := strings.NewReader("1\n")

    // Simulate legacy detection
    handleMigrationChoice(input, output, "ollama", "http://localhost:11434")

    // Should create global config
    cfg, err := config.LoadGlobalConfig()
    assert.NoError(t, err)
    assert.Equal(t, "ollama", cfg.Embedding.Provider)
}

func TestShowMigrationPrompt_SwitchProvider(t *testing.T) {
    // User chooses to switch
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    output := &bytes.Buffer{}
    input := strings.NewReader("2\n") // Switch to different provider

    handleMigrationChoice(input, output, "ollama", "http://localhost:11434")

    assert.Contains(t, output.String(), "pm config provider")
}

// === Edge Case Tests ===

func TestShowMigrationPrompt_AlreadyMigrated(t *testing.T) {
    // Edge case: global config already exists
    tempDir := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tempDir)

    // Create existing global config
    globalDir := filepath.Join(tempDir, "pommel")
    os.MkdirAll(globalDir, 0755)
    os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte("embedding: { provider: openai }"), 0644)

    output := &bytes.Buffer{}
    input := strings.NewReader("")

    needed := ShowMigrationPromptIfNeeded(input, output)

    assert.False(t, needed) // Already migrated
}

func TestShowMigrationPrompt_RemoteOllama(t *testing.T) {
    // Edge case: legacy config with remote URL
    output := &bytes.Buffer{}
    input := strings.NewReader("1\n")

    handleMigrationChoice(input, output, "ollama-remote", "http://192.168.1.100:11434")

    tempDir := os.Getenv("XDG_CONFIG_HOME")
    cfg, _ := config.LoadGlobalConfig()
    assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
}
```

---

### Step 4: Integration with Daemon

**File:** `internal/daemon/indexer.go` updates

**Tests to write first:**

```go
// indexer_test.go (additions)

func TestIndexer_UsesRetryForEmbeddings(t *testing.T) {
    // Happy path: retries on rate limit
    attempts := 0
    mockEmbedder := &mockEmbedder{
        embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
            attempts++
            if attempts < 3 {
                return nil, &embedder.EmbeddingError{
                    Code:       "RATE_LIMITED",
                    Retryable:  true,
                    RetryAfter: 10 * time.Millisecond,
                }
            }
            return make([][]float32, len(texts)), nil
        },
    }

    indexer := NewIndexer(mockEmbedder, nil)
    _, err := indexer.IndexChunk(context.Background(), "test content")

    assert.NoError(t, err)
    assert.Equal(t, 3, attempts)
}

func TestIndexer_ReportsProgress(t *testing.T) {
    mockEmbedder := &mockEmbedder{
        embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
            return make([][]float32, len(texts)), nil
        },
    }

    var progressUpdates []float64
    progressCallback := func(p *IndexProgress) {
        progressUpdates = append(progressUpdates, p.Percentage())
    }

    indexer := NewIndexer(mockEmbedder, nil)
    indexer.OnProgress(progressCallback)

    chunks := make([]string, 10)
    for i := range chunks {
        chunks[i] = fmt.Sprintf("chunk %d", i)
    }

    indexer.IndexBatch(context.Background(), chunks)

    assert.NotEmpty(t, progressUpdates)
    assert.Equal(t, 100.0, progressUpdates[len(progressUpdates)-1])
}

func TestIndexer_ETADisplayFormat(t *testing.T) {
    p := NewIndexProgress(1000)

    // Add some batches
    for i := 0; i < 5; i++ {
        p.RecordBatch(10, time.Second)
    }

    display := FormatETA(p.ETA())

    // 950 remaining at 10/sec = 95 seconds
    assert.Contains(t, display, "1m") // About 1.5 minutes
}

func TestIndexer_ETADisplayCalculating(t *testing.T) {
    p := NewIndexProgress(100)

    display := FormatETA(p.ETA())

    assert.Equal(t, "Calculating...", display)
}
```

---

### Step 5: Documentation Updates

**Files:** `README.md`, `CLAUDE.md`

**Tests:** Manual review checklist (no automated tests)

**README.md additions:**

```markdown
## Embedding Providers

Pommel supports multiple embedding providers:

| Provider | Type | Cost | Best For |
|----------|------|------|----------|
| Local Ollama | Local | Free | Default, privacy-focused |
| Remote Ollama | Remote | Free | Offload to server/NAS |
| OpenAI | API | $0.02/1M tokens | Easy setup, existing key |
| Voyage AI | API | $0.06/1M tokens | Code-specialized |

### Configuration

Run during installation or anytime:

```bash
pm config provider
```

Or set directly:

```bash
pm config provider openai --api-key sk-your-key
pm config provider voyage --api-key pa-your-key
pm config provider ollama-remote --url http://192.168.1.100:11434
```

Configuration is stored in `~/.config/pommel/config.yaml`.

### Switching Providers

When you switch providers, Pommel will prompt to reindex:

```
⚠ Embedding provider changed (ollama → openai)
  Existing index has 847 chunks with incompatible dimensions.
  Reindex now? (Y/n)
```
```

**CLAUDE.md additions:**

```markdown
## Embedding Provider Configuration

Pommel may need provider configuration before use. Check status:

```bash
pm status
```

If embeddings are not configured:

```bash
pm config provider
```

### Provider-Specific Notes

- **Local Ollama**: Requires Ollama running (`ollama serve`)
- **Remote Ollama**: Requires accessible URL
- **OpenAI/Voyage**: Requires valid API key (set via config or environment variable)
```

---

## Acceptance Criteria

- [ ] ETA calculation uses rolling window of recent batches
- [ ] ETA displays "Calculating..." until enough data
- [ ] Rate limits trigger automatic retry with backoff
- [ ] Non-retryable errors fail immediately
- [ ] Context cancellation stops retries
- [ ] Migration prompt appears for users with legacy config
- [ ] Migration prompt doesn't appear if already configured
- [ ] README documents all provider options
- [ ] CLAUDE.md includes provider troubleshooting

## Dependencies

- Phase 1-4 (all prior phases)

## Estimated Test Count

- Progress/ETA calculation: ~12 tests
- Retry logic: ~10 tests
- Migration prompts: ~6 tests
- Indexer integration: ~5 tests
- Documentation: Manual review

**Total: ~33 tests**
