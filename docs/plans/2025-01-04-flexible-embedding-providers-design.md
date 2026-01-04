# Flexible Embedding Providers

**Date:** 2025-01-04
**Status:** Approved

## Overview

This feature adds support for multiple embedding providers beyond local Ollama, enabling users to choose the setup that fits their environment:

**Supported Providers:**
- **Local Ollama** (current default) - Free, runs on the user's machine
- **Remote Ollama** - Free, connects to Ollama running elsewhere (NAS, server, etc.)
- **OpenAI API** - Paid, `text-embedding-3-small` ($0.02/1M tokens), easiest setup
- **Voyage AI** - Paid, `voyage-code-3` ($0.06/1M tokens), optimized for code search

**Key Design Decisions:**
1. **Global config with per-project override** - Provider settings live in `~/.config/pommel/config.yaml` by default, but projects can override in `.pommel/config.yaml`
2. **API keys in config file** - With environment variable fallback for CI/automation
3. **Install scripts ask interactively** - No assumptions about local Ollama; user chooses their provider during install
4. **`pm config provider` command** - Dedicated command for provider setup/changes, separate from `pm init`
5. **Automatic reindex on provider change** - Detect dimension mismatches and prompt to reindex

**User Experience Goals:**
- "It just works" regardless of which provider they choose
- Beginners can paste an API key during install and never think about it again
- Power users can configure per-project overrides
- Clear error messages when provider isn't configured

**Upgrade Experience:**
- Install scripts detect existing Pommel installation and display "Previous install detected - upgrading to x.x.x"
- Both bash and PowerShell scripts show the version being installed
- Existing users with local Ollama continue working unchanged (backwards compatible)
- On first run after upgrade, `pm start` detects missing global config and prompts: "Run 'pm config provider' to configure embeddings (your current Ollama setup will keep working)"

---

## Configuration Schema

**Global config: `~/.config/pommel/config.yaml`**

```yaml
# Embedding provider configuration
embedding:
  provider: openai  # ollama | ollama-remote | openai | voyage

  # Provider-specific settings
  ollama:
    url: "http://localhost:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"

  ollama-remote:
    url: "http://192.168.1.100:11434"
    model: "unclemusclez/jina-embeddings-v2-base-code"

  openai:
    api_key: "sk-..."  # or use OPENAI_API_KEY env var
    model: "text-embedding-3-small"

  voyage:
    api_key: "pa-..."  # or use VOYAGE_API_KEY env var
    model: "voyage-code-3"
```

**Per-project override: `.pommel/config.yaml`**

```yaml
# Only need to specify what you're overriding
embedding:
  provider: voyage  # This project uses Voyage instead of global default
```

**Environment variable fallback:**
- `OPENAI_API_KEY` - Used if `openai.api_key` is empty
- `VOYAGE_API_KEY` - Used if `voyage.api_key` is empty
- `OLLAMA_HOST` - Used if `ollama.url` or `ollama-remote.url` is empty

---

## Provider Abstraction (Go Code)

The existing `Embedder` interface already works well. We add a factory function to create the right provider based on config:

**`internal/embedder/embedder.go`** (existing interface, unchanged):
```go
type Embedder interface {
    EmbedSingle(ctx context.Context, text string) ([]float32, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Health(ctx context.Context) error
    ModelName() string
    Dimensions() int
}
```

**`internal/embedder/provider.go`** (new):
```go
type ProviderType string

const (
    ProviderOllama       ProviderType = "ollama"
    ProviderOllamaRemote ProviderType = "ollama-remote"
    ProviderOpenAI       ProviderType = "openai"
    ProviderVoyage       ProviderType = "voyage"
)

// NewFromConfig creates an Embedder based on configuration
func NewFromConfig(cfg *config.EmbeddingConfig) (Embedder, error) {
    switch cfg.Provider {
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

**New files:**
- `internal/embedder/openai.go` - OpenAI API client
- `internal/embedder/voyage.go` - Voyage AI API client

Both are simple HTTP clients (~100-150 lines each) hitting their respective `/embeddings` endpoints.

---

## Install Script Flow

**Bash script (`scripts/install.sh`):**

```bash
# Early in the script - detect existing installation
detect_existing_install() {
    if command -v pm &> /dev/null; then
        CURRENT_VERSION=$(pm version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
        if [[ -n "$CURRENT_VERSION" ]]; then
            info "Previous install detected (v${CURRENT_VERSION}) - upgrading to v${VERSION}"
            IS_UPGRADE=true
            return 0
        fi
    fi
    IS_UPGRADE=false
}

# Provider selection prompt
select_provider() {
    echo ""
    echo "[2/4] How would you like to generate embeddings?"
    echo ""
    echo "  1) Local Ollama    - Free, runs on this machine (~300MB model)"
    echo "  2) Remote Ollama   - Free, connect to Ollama on another machine"
    echo "  3) OpenAI API      - Paid, no local setup required"
    echo "  4) Voyage AI       - Paid, optimized for code search"
    echo ""
    read -p "  Choice [1]: " choice
    choice=${choice:-1}

    case $choice in
        1) setup_local_ollama ;;
        2) setup_remote_ollama ;;
        3) setup_openai ;;
        4) setup_voyage ;;
        *) error "Invalid choice" ;;
    esac
}

# Example: OpenAI setup
setup_openai() {
    echo ""
    read -p "  Enter your OpenAI API key (leave blank to configure later): " api_key

    if [[ -n "$api_key" ]]; then
        if validate_openai_key "$api_key"; then
            write_global_config "openai" "$api_key"
            success "API key validated and saved"
        else
            warn "Invalid API key. Run 'pm config provider' later to configure."
        fi
    else
        write_global_config "openai" ""
        info "Skipped. Run 'pm config provider' to add your API key later."
    fi
}
```

**PowerShell script (`scripts/install.ps1`):**

Same interactive flow, with equivalent prompts using `Read-Host` and `Write-Host`.

**Upgrade behavior:**
- If upgrading and global config already exists -> skip provider selection
- If upgrading from pre-provider version -> prompt once, existing Ollama users auto-migrate

**Automation flags:**
```powershell
# Automated install with OpenAI
.\install.ps1 -Provider OpenAI -ApiKey $env:OPENAI_API_KEY
```

---

## CLI Commands

**`pm config provider`** - Interactive provider setup:

```
$ pm config provider

Current provider: (not configured)

Select an embedding provider:

  1) Local Ollama    - Free, runs on this machine
  2) Remote Ollama   - Free, connect to Ollama elsewhere
  3) OpenAI API      - Paid, no local setup
  4) Voyage AI       - Paid, optimized for code

Choice: 3

Enter your OpenAI API key (leave blank to set later): sk-...

Validating API key... ✓

Configuration saved to ~/.config/pommel/config.yaml

  Provider: openai
  Model:    text-embedding-3-small

✓ Ready! Run 'pm start' to begin indexing.
```

**`pm config provider <name>`** - Direct selection (for scripts/automation):

```
$ pm config provider openai
Enter your OpenAI API key: sk-...
✓ Configured OpenAI provider
```

**`pm init`** - Now warns if provider missing:

```
$ pm init

Initializing Pommel in /Users/ryan/myproject...

  ✓ Created .pommel/
  ✓ Created .pommel/config.yaml
  ✓ Added .pommel/ to .gitignore

⚠ No embedding provider configured

  Run 'pm config provider' to set up embeddings.
  Until configured, 'pm start' and 'pm search' will not work.
```

**`pm start` / `pm search`** - Hard fail if no provider:

```
$ pm start

✖ No embedding provider configured

  Run 'pm config provider' to set up embeddings.
```

---

## Dimension Mismatch & Reindexing

**Database metadata tracking:**

The `.pommel/pommel.db` SQLite database stores provider info:

```sql
-- New metadata table (or add to existing)
CREATE TABLE IF NOT EXISTS index_metadata (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Stored values:
-- provider = "openai"
-- model = "text-embedding-3-small"
-- dimensions = 1536
-- indexed_at = "2025-01-04T12:00:00Z"
```

**Detection on startup (`pm start`):**

```go
func (d *Daemon) checkProviderMismatch() error {
    stored := d.db.GetMetadata("provider", "dimensions")
    current := d.embedder.ProviderName(), d.embedder.Dimensions()

    if stored.Provider != "" && stored.Provider != current.Provider {
        return &ProviderMismatchError{
            Old: stored,
            New: current,
        }
    }
    return nil
}
```

**User-facing prompt:**

```
$ pm start

⚠ Embedding provider changed (ollama → openai)
  Existing index has 847 chunks with incompatible dimensions (768 → 1536).

  Reindex now? This will take ~2 minutes. (Y/n): y

  Reindexing... ████████████████████ 100%

✓ Daemon started (PID 12345)
```

**Non-interactive mode (for CI):**

```
$ pm start --reindex-if-needed
# Automatically reindexes without prompting
```

---

## Adaptive ETA Calculation

ETA must account for varying latency across providers (local Ollama vs remote APIs):

```go
type IndexProgress struct {
    TotalChunks     int
    CompletedChunks int
    StartTime       time.Time

    // Rolling window of recent batch timings
    recentBatches   []BatchTiming  // last 10 batches
}

type BatchTiming struct {
    Chunks   int
    Duration time.Duration
}

func (p *IndexProgress) ETA() time.Duration {
    if len(p.recentBatches) < 2 {
        return 0  // "Calculating..." until we have data
    }

    // Calculate chunks/sec from recent batches (not overall average)
    var totalChunks int
    var totalDuration time.Duration
    for _, b := range p.recentBatches {
        totalChunks += b.Chunks
        totalDuration += b.Duration
    }

    chunksPerSec := float64(totalChunks) / totalDuration.Seconds()
    remaining := p.TotalChunks - p.CompletedChunks

    return time.Duration(float64(remaining) / chunksPerSec) * time.Second
}
```

**Display behavior:**

```
$ pm status

Indexing in progress...
  Completed: 234 / 847 chunks
  Rate:      12.3 chunks/sec        # Measured, not assumed
  ETA:       ~50 seconds
```

First few seconds show "Calculating ETA..." until we have enough samples.

---

## Error Handling

**API-specific error handling:**

```go
// internal/embedder/errors.go

type EmbeddingError struct {
    Code       string  // Machine-readable: RATE_LIMITED, AUTH_FAILED, etc.
    Message    string  // Human-readable description
    Suggestion string  // What the user should do
    Retryable  bool    // Can we retry automatically?
    RetryAfter time.Duration  // How long to wait (for rate limits)
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
)
```

**Automatic retry with backoff (for rate limits):**

```go
func (c *OpenAIClient) embedWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
    maxRetries := 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        result, err := c.embed(ctx, texts)
        if err == nil {
            return result, nil
        }

        var embErr *EmbeddingError
        if errors.As(err, &embErr) && embErr.Retryable {
            wait := embErr.RetryAfter
            if wait == 0 {
                wait = time.Duration(1<<attempt) * time.Second  // 1s, 2s, 4s
            }
            time.Sleep(wait)
            continue
        }

        return nil, err  // Non-retryable error
    }

    return nil, fmt.Errorf("max retries exceeded")
}
```

**User-facing during indexing:**

```
Indexing... ████████░░░░░░░░░░░░ 42%
  ⚠ Rate limited, waiting 2s...

Indexing... █████████░░░░░░░░░░░ 45%
```

---

## Backwards Compatibility & Migration

**Existing users upgrading:**

When a user with an existing Pommel installation upgrades:

1. **Install script detects upgrade** -> skips provider selection if global config exists
2. **No global config exists (pre-provider version)** -> one-time migration prompt:

```
╔════════════════════════════════════════════════════════════════════╗
║  Pommel now supports multiple embedding providers!                  ║
╚════════════════════════════════════════════════════════════════════╝

Your existing setup uses local Ollama. This will continue to work.

Would you like to:
  1) Keep using local Ollama (no changes needed)
  2) Switch to a different provider

Choice [1]: 1

✓ Migrated config to ~/.config/pommel/config.yaml
  Provider: ollama (local)

Your existing project indexes are unchanged.
```

**Existing project indexes:**

- If provider hasn't changed -> indexes work as-is
- If user switches provider -> `pm start` detects mismatch and prompts to reindex

**Config file precedence:**

1. Per-project `.pommel/config.yaml` (if `embedding.provider` set)
2. Global `~/.config/pommel/config.yaml`
3. Legacy: check for `embedding.ollama_url` in old per-project config -> auto-migrate to new format

**Old config format (still works):**
```yaml
embedding:
  model: "unclemusclez/jina-embeddings-v2-base-code"
  ollama_url: "http://localhost:11434"
```

Automatically interpreted as `provider: ollama` with those settings.

---

## Implementation Phases

### Phase 1: Provider Abstraction (foundation)
- Add `internal/embedder/provider.go` with factory function
- Add `internal/embedder/openai.go` client
- Add `internal/embedder/voyage.go` client
- Update `EmbeddingConfig` struct with new provider fields
- Add provider/dimensions metadata to database
- Unit tests for each provider client

### Phase 2: Global Config Support
- Add global config loading from `~/.config/pommel/config.yaml`
- Implement config precedence (project -> global -> defaults)
- Environment variable fallback for API keys
- Config validation (check required fields per provider)

### Phase 3: CLI Commands
- Add `pm config provider` interactive command
- Add `pm config provider <name>` direct mode
- Update `pm init` to warn when provider not configured
- Update `pm start` / `pm search` to fail clearly without provider
- Add `--reindex-if-needed` flag to `pm start`

### Phase 4: Install Scripts
- Update `install.sh` with provider selection prompt
- Update `install.ps1` with same flow
- Add upgrade detection and version display to both
- Add API key validation during setup
- Test on fresh install and upgrade scenarios

### Phase 5: Polish
- Adaptive ETA calculation with rolling window
- Rate limit handling with automatic retry
- Migration prompts for existing users
- Documentation updates (README, CLAUDE.md)

---

## Testing Requirements

**All implementation must follow strict Test-Driven Development (TDD):**

1. Write tests first, before any implementation code
2. Tests must fail initially (red phase)
3. Write minimal code to make tests pass (green phase)
4. Refactor while keeping tests passing (refactor phase)
5. Repeat for each piece of functionality

**Test Coverage Categories:**

Each component must include tests covering:

- **Happy Path Scenarios** - Standard successful operations with valid inputs
- **Success Scenarios** - Variations of successful operations (different valid inputs, configurations)
- **Failure Scenarios** - Expected failures that are handled gracefully (invalid API keys, network timeouts)
- **Error Scenarios** - Unexpected errors and edge cases in error handling itself
- **Edge Case Scenarios** - Boundary conditions, empty inputs, nil values, concurrent access, large payloads

**Test Organization:**

- Unit tests for each provider client (`openai_test.go`, `voyage_test.go`)
- Integration tests for provider factory and config loading
- Mock-based tests for API interactions (no real API calls in unit tests)
- End-to-end tests for CLI commands where feasible

See detailed phase plans for specific test cases per component.
