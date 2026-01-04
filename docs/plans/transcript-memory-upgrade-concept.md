# Pommel Transcript Memory Upgrade

## Overview

This document describes an enhancement to Pommel that enables semantic search over Claude Code conversation transcripts. This gives AI coding agents a form of "long-term memory" that persists across context compactions and session restarts—addressing the gap where valuable context gets lost when conversations exceed token limits.

### Problem Statement

When working with Claude Code on complex projects:
- Context compaction discards detailed reasoning and decisions
- Previous session context is inaccessible
- Important architectural decisions get lost
- Users end up re-explaining past work
- Sub-agent work disappears after task completion

### Solution

Extend Pommel to:
1. Index Claude Code transcripts via lifecycle hooks
2. Store conversation chunks with semantic embeddings
3. Enable search across past coding sessions
4. Capture user-annotated "milestones" and decisions
5. Generate session summaries for high-level retrieval

---

## Architecture

### Existing Pommel Architecture

Pommel already uses a daemon architecture where:
- Each project runs a separate `pommeld` instance
- The CLI communicates with the daemon via HTTP
- Port assignment is deterministic (hash-based) per project
- LanceDB stores embeddings with FTS support

### Extended Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Claude Code                               │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │ PreCompact   │ │UserPromptSub │ │ SessionStart             │ │
│  │ Hook         │ │ Hook         │ │ Hook                     │ │
│  └──────┬───────┘ └──────┬───────┘ └────────────┬─────────────┘ │
└─────────┼────────────────┼──────────────────────┼───────────────┘
          │                │                      │
          ▼                ▼                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Pommel Hook Scripts                          │
│  (Lightweight shims that POST to daemon and exit immediately)   │
│                                                                  │
│  • pm hook precompact   → POST /index/transcript                │
│  • pm hook user-prompt  → POST /index/transcript/annotate       │
│  • pm hook session-start → POST /session/start                  │
└─────────────────────────────────────────────────────────────────┘
          │                │                      │
          └────────────────┼──────────────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                         pommeld                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Index Queue                               ││
│  │  • Accepts work items from hooks                            ││
│  │  • Processes in background                                  ││
│  │  • Tracks progress, rate, ETA                               ││
│  │  • Deduplicates by session_id                               ││
│  │  • Resilient: re-queues incomplete jobs on restart          ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                 Processing Pipeline                          ││
│  │  1. Read transcript JSONL (retry with backoff if locked)    ││
│  │  2. Chunk by message/tool-use boundaries                    ││
│  │  3. Scrub sensitive content (regex + optional LLM)          ││
│  │  4. Generate embeddings (nomic-embed-text)                  ││
│  │  5. Generate summary (llama3.1:8b)                          ││
│  │  6. Store in LanceDB                                        ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    LanceDB Storage                           ││
│  │  • code_chunks table (existing)                             ││
│  │  • transcript_chunks table (new)                            ││
│  │  • Full-text search indexes                                 ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Daemon Endpoints

**Existing endpoints:**
```
POST /index/code          — trigger code indexing
GET  /search/code         — search code index
GET  /status              — current status
```

**New endpoints:**
```
POST /index/transcript              — index a transcript (compaction event)
POST /index/transcript/annotate     — index with user annotation
GET  /search/transcripts            — search transcript index
GET  /search/all                    — search both indexes, merged results
GET  /annotations                   — list all annotations
GET  /queue                         — detailed queue status with ETA
POST /session/start                 — register session start (future-proofing)
```

---

## Claude Code Hooks Integration

### Hook Events Used

| Hook Event | Purpose | Pommel Action |
|------------|---------|---------------|
| `PreCompact` | Fires before context compaction | Index full transcript + generate summary |
| `UserPromptSubmit` | Fires when user submits prompt | Detect "Note to Pommel:" annotations |
| `SessionStart` | Fires on session start/resume | Register session (future-proofing for context injection) |

### Hook Configuration

Pommel automatically installs these hooks via `pm init`. The hooks are appended to `.claude/settings.json` if not already present (idempotent — skips if already installed):

```json
{
  "hooks": {
    "PreCompact": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "pm hook precompact"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "pm hook user-prompt"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "pm hook session-start"
          }
        ]
      }
    ]
  }
}
```

### Hook Input Data

**PreCompact hook receives:**
```json
{
  "session_id": "abc123",
  "transcript_path": "~/.claude/projects/.../session.jsonl",
  "hook_event_name": "PreCompact",
  "trigger": "manual | auto",
  "custom_instructions": "",
  "cwd": "/path/to/project",
  "permission_mode": "default"
}
```

**UserPromptSubmit hook receives:**
```json
{
  "session_id": "abc123",
  "transcript_path": "~/.claude/projects/.../session.jsonl",
  "hook_event_name": "UserPromptSubmit",
  "prompt": "Note to Pommel: Decided to use Redis for caching",
  "cwd": "/path/to/project"
}
```

**SessionStart hook receives:**
```json
{
  "session_id": "abc123",
  "transcript_path": "~/.claude/projects/.../session.jsonl",
  "hook_event_name": "SessionStart",
  "source": "startup | resume | clear | compact",
  "cwd": "/path/to/project"
}
```

### Hook Implementation (Go)

**PreCompact Hook Command:**
```go
// cmd/hook_precompact.go

package cmd

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

type PreCompactInput struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    Trigger        string `json:"trigger"`
    Cwd            string `json:"cwd"`
}

type IndexTranscriptRequest struct {
    TranscriptPath  string `json:"transcript_path"`
    SessionID       string `json:"session_id"`
    Trigger         string `json:"trigger"`
    GenerateSummary bool   `json:"generate_summary"`
}

var hookPrecompactCmd = &cobra.Command{
    Use:   "precompact",
    Short: "Handle PreCompact hook from Claude Code",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Read hook input from stdin
        var input PreCompactInput
        if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
            // Don't block Claude on parse errors
            fmt.Fprintf(os.Stderr, "Warning: Could not parse hook input: %v\n", err)
            return nil
        }

        // Build request for daemon
        request := IndexTranscriptRequest{
            TranscriptPath:  input.TranscriptPath,
            SessionID:       input.SessionID,
            Trigger:         input.Trigger,
            GenerateSummary: true,
        }

        // Non-blocking POST to daemon
        client, err := GetDaemonClient()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Warning: Pommel daemon not responding. Conversation not archived.\n")
            return nil // Don't block Claude
        }

        resp, err := client.Post("/index/transcript", request)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Warning: Could not queue transcript for indexing: %v\n", err)
            return nil // Don't block Claude
        }

        if !resp.Queued {
            fmt.Fprintf(os.Stderr, "Warning: Transcript not queued: %s\n", resp.Reason)
        }

        return nil
    },
}
```

**UserPromptSubmit Hook Command:**
```go
// cmd/hook_user_prompt.go

package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "github.com/spf13/cobra"
)

type UserPromptInput struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    Prompt         string `json:"prompt"`
    Cwd            string `json:"cwd"`
}

type AnnotateRequest struct {
    TranscriptPath string `json:"transcript_path"`
    SessionID      string `json:"session_id"`
    Note           string `json:"note"`
}

const annotationPrefix = "Note to Pommel:"

var hookUserPromptCmd = &cobra.Command{
    Use:   "user-prompt",
    Short: "Handle UserPromptSubmit hook from Claude Code",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Read hook input from stdin
        var input UserPromptInput
        if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
            // Don't block Claude on parse errors
            return nil
        }

        prompt := strings.TrimSpace(input.Prompt)

        // Check for annotation prefix anywhere in the message
        if idx := strings.Index(prompt, annotationPrefix); idx != -1 {
            // Extract just the note portion after the prefix
            noteStart := idx + len(annotationPrefix)
            note := strings.TrimSpace(prompt[noteStart:])
            
            // Handle case where note continues until end of line or end of message
            if newlineIdx := strings.Index(note, "\n"); newlineIdx != -1 {
                note = strings.TrimSpace(note[:newlineIdx])
            }

            request := AnnotateRequest{
                TranscriptPath: input.TranscriptPath,
                SessionID:      input.SessionID,
                Note:           note,
            }

            // Non-blocking POST to daemon
            client, err := GetDaemonClient()
            if err != nil {
                fmt.Fprintf(os.Stderr, "Warning: Could not record annotation (daemon not responding)\n")
                return nil
            }

            if _, err := client.Post("/index/transcript/annotate", request); err != nil {
                fmt.Fprintf(os.Stderr, "Warning: Could not record annotation: %v\n", err)
            }
        }

        // Always exit 0 - let prompt through to Claude
        return nil
    },
}
```

**SessionStart Hook Command:**
```go
// cmd/hook_session_start.go

package cmd

import (
    "encoding/json"
    "os"

    "github.com/spf13/cobra"
)

type SessionStartInput struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    Source         string `json:"source"`
    Cwd            string `json:"cwd"`
}

type SessionStartRequest struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    Source         string `json:"source"`
}

var hookSessionStartCmd = &cobra.Command{
    Use:   "session-start",
    Short: "Handle SessionStart hook from Claude Code",
    RunE: func(cmd *cobra.Command, args []string) error {
        var input SessionStartInput
        if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
            return nil
        }

        request := SessionStartRequest{
            SessionID:      input.SessionID,
            TranscriptPath: input.TranscriptPath,
            Source:         input.Source,
        }

        // Fire and forget - just register the session (future-proofing)
        client, _ := GetDaemonClient()
        if client != nil {
            client.Post("/session/start", request)
        }

        return nil
    },
}
```

### Transcript File Access

When the PreCompact hook fires, the transcript file may still be in use. Implementation must retry with exponential backoff:

```go
// internal/indexer/transcript.go

func ReadTranscriptWithRetry(path string, maxRetries int) ([]TranscriptMessage, error) {
    var lastErr error
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        messages, err := ReadTranscript(path)
        if err == nil {
            return messages, nil
        }
        
        lastErr = err
        
        // Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
        backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
        if backoff > 2*time.Second {
            backoff = 2 * time.Second
        }
        
        time.Sleep(backoff)
    }
    
    return nil, fmt.Errorf("failed to read transcript after %d attempts: %w", maxRetries, lastErr)
}
```

---

## Model Stack

### Three-Model Architecture

| Model | Purpose | Size | Context |
|-------|---------|------|---------|
| jina/jina-embeddings-v2-base-code | Code embeddings | ~270MB | 8K |
| nomic-embed-text | Transcript/text embeddings | ~274MB | 8K |
| llama3.1:8b | Summarization, classification, LLM scrubbing | 4.7GB | 128K |

**Rationale:**
- **Jina for code**: Trained on code pairs, optimized for code semantics
- **nomic-embed-text for transcripts**: General-purpose, handles mixed content (code + natural language), 8K context for long chunks
- **llama3.1:8b for LLM tasks**: Single model for all generative tasks (summaries, scrubbing), 128K context handles long transcripts, 4.7GB is acceptable for the capability

**Total download:** ~5.2GB

### Embedding Model Details

**nomic-embed-text for transcripts:**
- 8K token context allows chunking at "task" level (prompt → all steps → response)
- General-purpose embeddings handle mixed content well
- 1024 dimensions

**Ollama configuration note:**
```go
// May need to explicitly set num_ctx for full context window
resp, err := ollamaClient.Embed(ctx, &ollama.EmbedRequest{
    Model:  "nomic-embed-text",
    Input:  []string{chunk},
    Options: map[string]interface{}{
        "num_ctx": 8192,
    },
})
```

### Summarization

```go
const summaryPrompt = `Summarize this Claude Code coding session concisely. Include:
- What the user was trying to accomplish
- Key decisions made
- Problems encountered and how they were resolved
- Files/modules that were modified
- Any important conclusions or next steps

Keep the summary focused and actionable for future reference.

Transcript:
%s

Summary:`
```

```go
// internal/indexer/summarize.go

package indexer

import (
    "context"
    "fmt"
    "strings"

    "github.com/ollama/ollama/api"
)

type Summarizer struct {
    client *api.Client
    model  string
}

func NewSummarizer(model string) (*Summarizer, error) {
    client, err := api.ClientFromEnvironment()
    if err != nil {
        return nil, err
    }
    return &Summarizer{client: client, model: model}, nil
}

func (s *Summarizer) Summarize(ctx context.Context, transcript string) (string, error) {
    prompt := fmt.Sprintf(summaryPrompt, transcript)

    var response strings.Builder

    err := s.client.Generate(ctx, &api.GenerateRequest{
        Model:  s.model,
        Prompt: prompt,
        Options: map[string]interface{}{
            "num_ctx":     65536, // Adjust based on transcript size
            "temperature": 0.3,   // Lower temperature for factual summary
        },
    }, func(resp api.GenerateResponse) error {
        response.WriteString(resp.Response)
        return nil
    })

    if err != nil {
        return "", fmt.Errorf("summary generation failed: %w", err)
    }

    return response.String(), nil
}
```

**Summary quality note:** Summaries are generated by llama3.1:8b and may occasionally miss nuance or mischaracterize details. They're intended as a quick reference for high-level retrieval, not as authoritative records. For precise details, search raw transcript chunks.

---

## Transcript Chunking Strategy

### Chunk Boundaries

Unlike code (which chunks by AST: file/class/method), transcripts have natural semantic boundaries:

1. **Per message** - User turn, assistant turn
2. **Per tool use** - PreToolUse + PostToolUse as a unit
3. **Per task** - Prompt → all steps → Stop
4. **Per sub-agent** - Task tool invocation start to finish

### Recommended Approach: Tool-Use-Centric

Group content into semantic units:
- "Claude searched for X, found Y, then edited Z"
- Maps to what users want to retrieve: "when did we touch the auth module?"

```go
// internal/indexer/chunker.go

package indexer

import (
    "encoding/json"
    "fmt"
    "strings"
    "time"
)

type TranscriptMessage struct {
    UUID      string          `json:"uuid"`
    Type      string          `json:"type"` // "user", "assistant", "tool_use", "tool_result"
    Timestamp time.Time       `json:"timestamp"`
    Message   json.RawMessage `json:"message"`
}

type TranscriptChunk struct {
    SessionID   string
    ChunkIndex  int
    ChunkType   string // "raw", "summary", "annotated"
    EventType   string // "user_prompt", "tool_use", "assistant", "mixed"
    Content     string
    Timestamp   time.Time
    Annotation  string // Only for annotated chunks
}

type ChunkConfig struct {
    MaxTokens             int  // Target max tokens per chunk (default: 6000)
    PreserveToolUseUnits  bool // Keep tool_use + tool_result together
}

func ChunkTranscript(messages []TranscriptMessage, config ChunkConfig) ([]TranscriptChunk, error) {
    var chunks []TranscriptChunk
    var currentChunk strings.Builder
    var currentTokens int
    var chunkIndex int
    var chunkEventType string
    var chunkTimestamp time.Time

    flushChunk := func() {
        if currentChunk.Len() > 0 {
            chunks = append(chunks, TranscriptChunk{
                ChunkIndex: chunkIndex,
                ChunkType:  "raw",
                EventType:  chunkEventType,
                Content:    currentChunk.String(),
                Timestamp:  chunkTimestamp,
            })
            chunkIndex++
            currentChunk.Reset()
            currentTokens = 0
            chunkEventType = ""
        }
    }

    for i, msg := range messages {
        content := formatMessage(msg)
        tokens := estimateTokens(content)

        // Check if this is a tool_use that should stay with its result
        if config.PreserveToolUseUnits && msg.Type == "tool_use" {
            // Look ahead for the tool_result
            if i+1 < len(messages) && messages[i+1].Type == "tool_result" {
                combinedContent := content + "\n" + formatMessage(messages[i+1])
                combinedTokens := estimateTokens(combinedContent)

                // If combined unit fits, add it
                if currentTokens+combinedTokens <= config.MaxTokens {
                    currentChunk.WriteString(combinedContent)
                    currentChunk.WriteString("\n\n")
                    currentTokens += combinedTokens
                    updateEventType(&chunkEventType, "tool_use")
                    if chunkTimestamp.IsZero() {
                        chunkTimestamp = msg.Timestamp
                    }
                    continue
                } else {
                    // Flush current and start new chunk with the unit
                    flushChunk()
                    currentChunk.WriteString(combinedContent)
                    currentChunk.WriteString("\n\n")
                    currentTokens = combinedTokens
                    chunkEventType = "tool_use"
                    chunkTimestamp = msg.Timestamp
                    continue
                }
            }
        }

        // Would this message exceed the limit?
        if currentTokens+tokens > config.MaxTokens {
            flushChunk()
        }

        currentChunk.WriteString(content)
        currentChunk.WriteString("\n\n")
        currentTokens += tokens
        updateEventType(&chunkEventType, msg.Type)
        if chunkTimestamp.IsZero() {
            chunkTimestamp = msg.Timestamp
        }
    }

    // Flush any remaining content
    flushChunk()

    return chunks, nil
}

func estimateTokens(text string) int {
    // Rough estimate: 4 chars per token
    return len(text) / 4
}

func formatMessage(msg TranscriptMessage) string {
    // Format message for embedding based on type
    switch msg.Type {
    case "user":
        return fmt.Sprintf("[USER] %s", extractContent(msg.Message))
    case "assistant":
        return fmt.Sprintf("[ASSISTANT] %s", extractContent(msg.Message))
    case "tool_use":
        return fmt.Sprintf("[TOOL_USE] %s", extractToolUse(msg.Message))
    case "tool_result":
        return fmt.Sprintf("[TOOL_RESULT] %s", extractToolResult(msg.Message))
    default:
        return string(msg.Message)
    }
}

func updateEventType(current *string, msgType string) {
    if *current == "" {
        *current = msgType
    } else if *current != msgType {
        *current = "mixed"
    }
}
```

---

## Scrubbing System

### Design Philosophy

Transcripts may contain sensitive information:
- API keys and tokens
- Passwords and credentials
- Connection strings
- PII (emails, phone numbers)
- Internal URLs

### Two-Layer Approach

1. **Regex patterns** (fast, default) - Catches common secret formats
2. **LLM scrubbing** (opt-in, thorough) - Catches unusual patterns

### Default Scrubbing Patterns

```toml
# ~/.config/pommel/scrubbing-patterns.toml
# Shipped with Pommel, user can edit

[[patterns]]
name = "aws-access-key"
pattern = "AKIA[0-9A-Z]{16}"
replacement = "[REDACTED_AWS_KEY]"

[[patterns]]
name = "aws-secret-key"
pattern = "(?i)aws_secret_access_key\\s*=\\s*[A-Za-z0-9/+=]{40}"
replacement = "[REDACTED_AWS_SECRET]"

[[patterns]]
name = "github-token"
pattern = "ghp_[A-Za-z0-9]{36}"
replacement = "[REDACTED_GITHUB_TOKEN]"

[[patterns]]
name = "github-fine-grained-token"
pattern = "github_pat_[A-Za-z0-9]{22}_[A-Za-z0-9]{59}"
replacement = "[REDACTED_GITHUB_PAT]"

[[patterns]]
name = "generic-api-key"
pattern = "(?i)(api[_-]?key|apikey|secret[_-]?key)\\s*[:=]\\s*['\"]?[A-Za-z0-9_\\-]{20,}['\"]?"
replacement = "[REDACTED_API_KEY]"

[[patterns]]
name = "jwt-token"
pattern = "eyJ[A-Za-z0-9_-]*\\.eyJ[A-Za-z0-9_-]*\\.[A-Za-z0-9_-]*"
replacement = "[REDACTED_JWT]"

[[patterns]]
name = "private-key-header"
pattern = "-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----"
replacement = "[REDACTED_PRIVATE_KEY]"

[[patterns]]
name = "password-assignment"
pattern = "(?i)(password|passwd|pwd)\\s*[:=]\\s*['\"][^'\"]{4,}['\"]"
replacement = "[REDACTED_PASSWORD]"

[[patterns]]
name = "connection-string-mongo"
pattern = "mongodb(\\+srv)?://[^\\s]+"
replacement = "[REDACTED_MONGODB_URI]"

[[patterns]]
name = "connection-string-postgres"
pattern = "postgres(ql)?://[^\\s]+"
replacement = "[REDACTED_POSTGRES_URI]"

[[patterns]]
name = "connection-string-mysql"
pattern = "mysql://[^\\s]+"
replacement = "[REDACTED_MYSQL_URI]"

[[patterns]]
name = "connection-string-redis"
pattern = "redis://[^\\s]+"
replacement = "[REDACTED_REDIS_URI]"

[[patterns]]
name = "slack-token"
pattern = "xox[baprs]-[A-Za-z0-9-]+"
replacement = "[REDACTED_SLACK_TOKEN]"

[[patterns]]
name = "stripe-key"
pattern = "sk_live_[A-Za-z0-9]{24,}"
replacement = "[REDACTED_STRIPE_KEY]"

# PII patterns (disabled by default - opt-in)
[[patterns]]
name = "email"
pattern = "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
replacement = "[REDACTED_EMAIL]"
enabled = false

[[patterns]]
name = "phone-us"
pattern = "\\b\\d{3}[-.]?\\d{3}[-.]?\\d{4}\\b"
replacement = "[REDACTED_PHONE]"
enabled = false

[[patterns]]
name = "ssn"
pattern = "\\b\\d{3}-\\d{2}-\\d{4}\\b"
replacement = "[REDACTED_SSN]"
enabled = false
```

### Scrubber Implementation (Go)

```go
// internal/scrubber/scrubber.go

package scrubber

import (
    "context"
    "fmt"
    "os"
    "regexp"
    "strings"

    "github.com/ollama/ollama/api"
)

type Pattern struct {
    Name        string `toml:"name"`
    Pattern     string `toml:"pattern"`
    Replacement string `toml:"replacement"`
    Enabled     bool   `toml:"enabled"`
}

type Config struct {
    Enabled  bool      `toml:"enabled"`
    LLMScrub bool      `toml:"llm_scrub"`
    LLMModel string    `toml:"llm_model"`
    Patterns []Pattern `toml:"patterns"`
}

type Scrubber struct {
    config   Config
    patterns []*compiledPattern
    llm      *api.Client
}

type compiledPattern struct {
    name        string
    regex       *regexp.Regexp
    replacement string
}

func New(config Config) (*Scrubber, error) {
    s := &Scrubber{config: config}

    // Compile regex patterns
    for _, p := range config.Patterns {
        if !p.Enabled {
            continue
        }
        re, err := regexp.Compile(p.Pattern)
        if err != nil {
            return nil, fmt.Errorf("invalid pattern %s: %w", p.Name, err)
        }
        s.patterns = append(s.patterns, &compiledPattern{
            name:        p.Name,
            regex:       re,
            replacement: p.Replacement,
        })
    }

    // Initialize LLM client if needed
    if config.LLMScrub {
        client, err := api.ClientFromEnvironment()
        if err != nil {
            return nil, fmt.Errorf("LLM scrubbing enabled but Ollama not available: %w", err)
        }
        s.llm = client
    }

    return s, nil
}

func (s *Scrubber) Scrub(ctx context.Context, text string) (string, error) {
    if !s.config.Enabled {
        return text, nil
    }

    // First pass: regex patterns
    result := text
    for _, p := range s.patterns {
        result = p.regex.ReplaceAllString(result, p.replacement)
    }

    // Second pass: LLM scrubbing (if enabled)
    if s.config.LLMScrub && s.llm != nil {
        scrubbed, err := s.llmScrub(ctx, result)
        if err != nil {
            // Log warning but don't fail - regex scrubbing is still applied
            fmt.Fprintf(os.Stderr, "Warning: LLM scrubbing failed: %v\n", err)
            return result, nil
        }
        result = scrubbed
    }

    return result, nil
}

const llmScrubPrompt = `You are a security-focused text processor. Your task is to identify and redact any sensitive information in the following text that may have been missed by pattern matching.

Look for:
- API keys, tokens, or secrets that don't match common patterns
- Passwords or credentials
- Internal URLs or endpoints that look sensitive
- Personal information (names associated with accounts, addresses)
- Any other information that could be a security risk if exposed

Replace each sensitive item with an appropriate [REDACTED_TYPE] placeholder.

If no additional sensitive information is found, return the text unchanged.

Text to process:
%s

Processed text:`

func (s *Scrubber) llmScrub(ctx context.Context, text string) (string, error) {
    prompt := fmt.Sprintf(llmScrubPrompt, text)

    var response strings.Builder
    err := s.llm.Generate(ctx, &api.GenerateRequest{
        Model:  s.config.LLMModel,
        Prompt: prompt,
        Options: map[string]interface{}{
            "temperature": 0.1, // Very low for consistent output
        },
    }, func(resp api.GenerateResponse) error {
        response.WriteString(resp.Response)
        return nil
    })

    if err != nil {
        return "", err
    }

    return strings.TrimSpace(response.String()), nil
}
```

---

## Database Schema

### LanceDB Tables

**Existing: code_chunks**
```go
type CodeChunk struct {
    ID           string    `lance:"id"`
    FilePath     string    `lance:"file_path"`
    ChunkType    string    `lance:"chunk_type"`    // "file", "class", "method"
    SymbolName   string    `lance:"symbol_name"`
    Content      string    `lance:"content"`
    Embedding    []float32 `lance:"embedding"`     // jina code embeddings (768 dim)
    Language     string    `lance:"language"`
    LastModified time.Time `lance:"last_modified"`
}
```

**New: transcript_chunks**
```go
type TranscriptChunk struct {
    ID          string    `lance:"id"`
    SessionID   string    `lance:"session_id"`
    ChunkIndex  int       `lance:"chunk_index"`     // Order within session
    ChunkType   string    `lance:"chunk_type"`      // "raw", "summary", "annotated"
    EventType   string    `lance:"event_type"`      // "user_prompt", "tool_use", "assistant", "mixed"
    Content     string    `lance:"content"`
    ContentFTS  string    `lance:"content_fts"`     // For full-text search
    Embedding   []float32 `lance:"embedding"`       // nomic-embed-text (1024 dim)
    Timestamp   time.Time `lance:"timestamp"`
    Trigger     string    `lance:"trigger"`         // "manual", "auto" (compaction type)
    Annotation  string    `lance:"annotation"`      // User's note (for annotated chunks)
}
```

### Index Structure

```
project/
├── .pommel/                    # ADD TO .gitignore
│   ├── pommel.db/              # LanceDB directory
│   │   ├── code_chunks.lance   # Existing code index
│   │   └── transcript_chunks.lance  # New transcript index
│   ├── queue.db                # SQLite for persistent queue
│   └── config.toml             # Project configuration
└── .claude/
    ├── settings.json           # Claude Code hooks
    └── CLAUDE.md               # Instructions
```

**Important:** `.pommel/` should be added to `.gitignore` to keep conversation history private to each developer's local repo.

---

## Configuration Files

### Global Configuration: `~/.config/pommel/config.toml`

```toml
[models]
# Embedding models
embedding_code = "jina/jina-embeddings-v2-base-code"
embedding_text = "nomic-embed-text"

# LLM model (used for scrubbing and summarization)
llm = "llama3.1:8b"

[scrubbing]
enabled = true
llm_scrub = false  # Opt-in for deeper scrubbing

[summarization]
enabled = true

[indexing]
# Performance tuning
batch_size = 10
max_concurrent = 4
```

### Project Configuration: `<project>/.pommel/config.toml`

```toml
# Project-specific overrides

[scrubbing]
enabled = true
llm_scrub = true  # Enable for this sensitive project

# Additional patterns for this project
[[scrubbing.patterns]]
name = "internal-api-keys"
pattern = "MYCOMPANY_[A-Z0-9]{32}"
replacement = "[REDACTED_INTERNAL_KEY]"

[[scrubbing.patterns]]
name = "staging-urls"
pattern = "https://staging\\.mycompany\\.internal[^\\s]*"
replacement = "[REDACTED_STAGING_URL]"

[transcripts]
# Claude Code project hash (auto-detected or explicit)
claude_project_hash = "auto"
```

---

## CLI Commands

### Existing Commands (Updated)

```bash
pm init              # Setup - now includes hook installation
pm start             # Start daemon - shows initial index time estimate
pm stop              # Stop daemon
pm status            # Enhanced with transcript stats and queue info
pm search <query>    # Now an alias for 'pm search all'
```

### New Commands

```bash
# Explicit search scopes
pm search code <query>          # Search code index only
pm search transcripts <query>   # Search transcript index only
pm search all <query>           # Search both, merged results (grouped by source)

# Search options
pm search <scope> <query> --limit N      # Limit results (default: 10)
pm search <scope> <query> --json         # JSON output for Claude Code
pm search <scope> <query> --full         # Show full content (not 200-char snippets)

# Transcript-specific options
pm search transcripts <query> --since "2024-12-01"   # After date (ISO format)
pm search transcripts <query> --since "last week"    # Natural language
pm search transcripts <query> --until "2024-12-31"   # Before date
pm search transcripts <query> --type annotated       # Only annotations
pm search transcripts <query> --type summary         # Only summaries

# Annotations
pm annotations                      # List all annotations
pm annotations --search "caching"   # Filter by keyword
pm annotations --since "last week"  # Filter by date
pm annotations --until "2024-12-01" # Filter by date
pm annotations --limit 20           # Limit results (default: 10)
pm annotations --json               # JSON output

# Hook handlers (called by Claude Code hooks)
pm hook precompact      # Handle PreCompact hook
pm hook user-prompt     # Handle UserPromptSubmit hook
pm hook session-start   # Handle SessionStart hook

# Status options
pm status               # Human-readable status
pm status --json        # Machine-readable for Claude Code
pm status --watch       # Live updating display (2 second refresh)
```

### Search Result Display

**Default (200-char snippet mode):**
```
$ pm search transcripts "caching strategy"

Transcripts (3 results)
═══════════════════════

[1] Score: 0.847 | 2024-12-28 14:23 | session: 8f3c...
    Type: annotated
    Note: "Decided to use Redis for session caching instead of in-memory"
    ───
    [USER] I think we should consider caching options for the session
    data. What do you think about Redis vs in-memory?
    [ASSISTANT] For session caching, Redis offers several advant...

[2] Score: 0.792 | 2024-12-15 09:41 | session: 2a1b...
    Type: raw
    ───
    [TOOL_USE] Read src/cache/config.go
    [TOOL_RESULT] package cache...
    [ASSISTANT] I see the current caching implementation uses a s...

[3] Score: 0.756 | 2024-12-10 16:18 | session: f9e2...
    Type: summary
    ───
    Session focused on implementing a caching layer for API respo...
```

**With --full flag:**
Shows complete chunk content instead of 200-character snippets.

**With --json flag:**
```json
{
  "scope": "transcripts",
  "query": "caching strategy",
  "total_results": 3,
  "results": [
    {
      "id": "chunk_abc123",
      "score": 0.847,
      "source": "transcript",
      "session_id": "8f3c...",
      "chunk_type": "annotated",
      "event_type": "mixed",
      "timestamp": "2024-12-28T14:23:00Z",
      "annotation": "Decided to use Redis for session caching instead of in-memory",
      "snippet": "[USER] I think we should consider caching options...",
      "content": "[USER] I think we should consider caching options for the session data. What do you think about Redis vs in-memory?\n[ASSISTANT] For session caching, Redis offers several advantages..."
    }
  ]
}
```

### Search All Results (Grouped by Source)

```
$ pm search all "authentication"

Code (2 results)
════════════════

[1] Score: 0.891 | src/auth/handler.go | function: ValidateToken
    func ValidateToken(token string) (*Claims, error) {
        claims := &Claims{}
        ...

[2] Score: 0.834 | src/auth/middleware.go | function: AuthMiddleware
    func AuthMiddleware(next http.Handler) http.Handler {
        ...

Transcripts (2 results)
═══════════════════════

[1] Score: 0.823 | 2024-12-20 11:15 | session: 4d2e...
    Type: annotated
    Note: "Authentication flow finalized - using JWT with 24h expiry"
    ───
    [USER] Let's finalize the authentication approach...

[2] Score: 0.798 | 2024-12-18 09:30 | session: 7b3f...
    Type: raw
    ───
    [ASSISTANT] I'll implement the JWT validation...
```

### Daemon Not Running - Fallback Behavior

When the daemon is not running during a search:

```
$ pm search transcripts "caching"

⚠️  WARNING: Pommel daemon not responding. Falling back to FTS-only search.
    (Semantic search unavailable)

Transcripts (2 results) [FTS-only]
══════════════════════════════════

[1] src/cache/redis.go mentioned in session 8f3c... (2024-12-28)
[2] "caching" found in session 2a1b... (2024-12-15)

Would you like to restart the daemon and rerun the query? [Y/n] 
```

---

## `pm init` Flow

```
$ pm init

Pommel Setup
============

[1/6] Checking Ollama...
      ✓ Ollama running at localhost:11434

[2/6] Checking required models...
      
      Downloading models (total: ~5.2 GB):
      
      jina/jina-embeddings-v2-base-code (code embeddings)
      [████████████████████████████████████████] 100% | 274 MB | ETA: done
      
      nomic-embed-text (text embeddings)
      [████████████████████████████████████████] 100% | 274 MB | ETA: done
      
      llama3.1:8b (summarization & classification)
      [██████████████░░░░░░░░░░░░░░░░░░░░░░░░░░]  35% | 1.6/4.7 GB | ETA: 2m 34s

      ✓ All models ready

[3/6] Initializing project structure...
      ✓ Created .pommel/
      ✓ Created .pommel/config.toml
      ✓ Added .pommel/ to .gitignore

[4/6] Initializing database...
      ✓ Created LanceDB at .pommel/pommel.db/
      ✓ Created code_chunks table
      ✓ Created transcript_chunks table
      ✓ Created queue database

[5/6] Installing Claude Code hooks...
      Updating .claude/settings.json...
      ✓ Added PreCompact hook
      ✓ Added UserPromptSubmit hook
      ✓ Added SessionStart hook

[6/6] Updating CLAUDE.md...
      ✓ Appended Pommel instructions to .claude/CLAUDE.md

Setup complete!

Next steps:
  1. Run 'pm start' to start the daemon and begin indexing
  2. Use Claude Code normally — transcripts will be indexed automatically
  3. Search with 'pm search all <query>'

Tip: Use 'pm status' to monitor indexing progress.
```

### Model Pull Failure Handling

If a model fails to download, continue with warning:

```
[2/6] Checking required models...
      
      Downloading models (total: ~5.2 GB):
      
      jina/jina-embeddings-v2-base-code (code embeddings)
      [████████████████████████████████████████] 100% | 274 MB | ETA: done
      
      nomic-embed-text (text embeddings)
      [████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░]  32% | ERROR: connection reset
      
      ⚠️  Warning: Failed to download nomic-embed-text
          Transcript search will not be available.
          Run 'ollama pull nomic-embed-text' later to enable.
      
      llama3.1:8b (summarization & classification)
      [████████████████████████████████████████] 100% | 4.7 GB | ETA: done
      
      ⚠️  Setup completed with warnings. Some features unavailable.
```

### `pm start` with Time Estimate

```
$ pm start

Starting Pommel daemon...
  ✓ Daemon started (PID 48291, port 17423)

Scanning project for initial indexing...

  Code files:     847 files (~12,400 chunks)

  Estimated index time: ~4 minutes
    • Code indexing: ~4 minutes (52 chunks/sec typical)

  Note: Transcripts are indexed on-demand via Claude Code hooks.
        No existing transcripts will be imported.

Indexing started in background.

  Monitor progress:   pm status
  Watch live:         pm status --watch
  JSON for scripts:   pm status --json

You can start using Claude Code now. Searches will return
partial results until indexing completes.
```

### `pm start` When Already Running

```
$ pm start

Pommel daemon is already running (PID 48291, port 17423).
Use 'pm status' to check current state.
```

---

## Enhanced Status Output

```
$ pm status

Pommel v0.6.0 — Running (PID 48291, port 17423)
===============================================

Indexes
───────
Code:
  Files indexed:    847
  Chunks:           12,453
  Last updated:     2 minutes ago

Transcripts:
  Sessions:         156
  Chunks:           8,721
  Annotations:      23
  Summaries:        89
  Last updated:     14 minutes ago

Indexing Queue
──────────────
  Pending:          3 items
  Processing:       session a8f3c... (chunk 4/127)

  Performance:
    Rate:           42.3 chunks/sec (avg last 5m)
    ETA:            ~8 seconds

  Recent Activity:
    ✓ session b2d1... compaction indexed (127 chunks, 3.1s)
    ✓ src/db/connection.go reindexed (8 chunks, 0.2s)
    ⧖ session a8f3c... indexing...

Storage
───────
  Code embeddings:      245 MB
  Transcript embeddings: 312 MB
  FTS index:            47 MB
  Total:                604 MB
  Growth (30d):         +89 MB

Models (via Ollama)
───────────────────
  Embedding (code):     jina/jina-embeddings-v2-base-code ✓
  Embedding (text):     nomic-embed-text ✓
  LLM:                  llama3.1:8b ✓
```

### JSON Status Output

```bash
$ pm status --json
```

```json
{
  "version": "0.6.0",
  "status": "running",
  "pid": 48291,
  "port": 17423,
  "indexes": {
    "code": {
      "files": 847,
      "chunks": 12453,
      "last_updated": "2025-01-01T14:32:15Z"
    },
    "transcripts": {
      "sessions": 156,
      "chunks": 8721,
      "annotations": 23,
      "summaries": 89,
      "last_updated": "2025-01-01T14:18:42Z"
    }
  },
  "queue": {
    "pending": 3,
    "processing": {
      "item": "session:a8f3c",
      "type": "transcript",
      "progress": {
        "current": 4,
        "total": 127
      }
    },
    "rate_chunks_per_sec": 42.3,
    "eta_seconds": 8,
    "is_idle": false
  },
  "storage": {
    "code_mb": 245,
    "transcripts_mb": 312,
    "fts_mb": 47,
    "total_mb": 604,
    "growth_30d_mb": 89
  },
  "models": {
    "embedding_code": {
      "name": "jina/jina-embeddings-v2-base-code",
      "available": true
    },
    "embedding_text": {
      "name": "nomic-embed-text",
      "available": true
    },
    "llm": {
      "name": "llama3.1:8b",
      "available": true
    }
  }
}
```

### Watch Mode

```bash
$ pm status --watch
```

Refreshes every 2 seconds with live updates to queue progress, rate, and ETA.

---

## CLAUDE.md Instructions

The following section is automatically appended to `.claude/CLAUDE.md` by `pm init`:

```markdown
## Code & Conversation Search (Pommel)

This project uses Pommel for semantic search across code and past coding sessions.

### Searching

```bash
# Search code
pm search code "auth handler"

# Search past sessions
pm search transcripts "caching discussion"

# Search both (results grouped by source)
pm search all "database connection"

# Temporal search
pm search transcripts "refactoring" --since "last week"
pm search transcripts "auth" --since "2024-12-01" --until "2024-12-31"

# Limit results (default: 10)
pm search all "auth" --limit 20

# Full content instead of snippets
pm search transcripts "caching" --full

# JSON output for parsing
pm search all "auth" --json
```

### Checking Index Status

After compaction or major file changes, check if indexing is complete:

```bash
pm status --json | jq '.queue.is_idle'
```

Wait for `true` if you need fresh results. The queue drains automatically.

### Annotations & Milestones

Important decisions and milestones are captured when the user writes:

```
Note to Pommel: <description of decision or milestone>
```

This can appear anywhere in a message. Pommel extracts the note and indexes the conversation up to that point with the annotation as a searchable marker. Annotations are permanent once created.

**To find past decisions:**

```bash
pm annotations                      # List all annotations
pm annotations --search "caching"   # Filter by keyword
pm annotations --since "2024-12-01" # Filter by date
pm annotations --json               # JSON output
```

**How to use annotations:**

- When user references "that decision we made about X" → search annotations first
- When starting work on a module → check for relevant architectural decisions
- Annotations represent deliberate milestones; weight them higher than raw transcript noise
- When you see "Note to Pommel:" in the conversation, acknowledge that you understand this is being recorded as a milestone

### When to Search

**Do search when:**

- User references past work ("like we did before", "as we discussed")
- Starting complex tasks that might have prior context
- You encounter a familiar-seeming problem
- User asks about past decisions or approaches

**Don't search when:**

- Context is already in current conversation
- Query is general knowledge, not project-specific
- User is clearly starting fresh

### Waiting for Indexing

If you've just triggered a large indexing operation (e.g., after compaction), you can wait for it to complete:

```bash
# Check if queue is empty
pm status --json | jq '.queue.is_idle'

# Or watch until idle
while [ "$(pm status --json | jq '.queue.is_idle')" != "true" ]; do
  sleep 2
done
echo "Indexing complete"
```
```

---

## Search & Reranking

### Hybrid Search

Like code search, transcript search uses hybrid semantic + FTS:

1. **Semantic search**: Query embedded with nomic-embed-text, vector similarity in LanceDB
2. **Full-text search**: Keyword matching in content_fts field
3. **Score fusion**: Reciprocal Rank Fusion (RRF) to combine results
4. **Reranking**: Apply recency and type boosting

### Recency-Aware Reranking

**Adaptive recency boosting:**

```go
// Apply recency decay only when results span a wide time range
func applyRecencyBoost(results []SearchResult) []SearchResult {
    if len(results) < 2 {
        return results
    }

    // Calculate temporal spread
    timestamps := make([]time.Time, len(results))
    for i, r := range results {
        timestamps[i] = r.Timestamp
    }
    stdDev := temporalStdDev(timestamps)

    // If results are clustered in time, skip recency boost
    // (the cluster IS the relevant context)
    if stdDev < 7*24*time.Hour { // Less than 7 days spread
        return results
    }

    // Apply exponential decay based on age
    now := time.Now()
    for i := range results {
        age := now.Sub(results[i].Timestamp)
        // Half-life of 30 days
        decay := math.Exp(-age.Hours() / (30 * 24))
        results[i].Score *= decay
    }

    // Re-sort by adjusted score
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    return results
}
```

### Chunk Type Boosting

```go
func applyTypeBoost(results []SearchResult, queryIntent string) []SearchResult {
    for i := range results {
        switch results[i].ChunkType {
        case "annotated":
            // Annotations are deliberate milestones - boost significantly
            results[i].Score *= 1.5
        case "summary":
            // Summaries are good for high-level queries
            // Slight penalty for detailed implementation queries
            if isHighLevelQuery(queryIntent) {
                results[i].Score *= 1.2
            } else {
                results[i].Score *= 0.8
            }
        case "raw":
            // No adjustment for raw chunks
        }
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    return results
}
```

---

## Queue Management

### Deduplication

When multiple compaction events fire for the same session (e.g., user triggers `/compact` while auto-compact is already queued), deduplicate by `session_id`:

```go
func (q *Queue) Enqueue(item QueueItem) error {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    // Check for existing item with same session_id
    for i, existing := range q.items {
        if existing.SessionID == item.SessionID {
            // Replace with newer item (may have updated transcript)
            q.items[i] = item
            return nil
        }
    }
    
    q.items = append(q.items, item)
    return nil
}
```

### Resilience

On daemon restart, incomplete jobs are re-queued from scratch:

```go
func (q *Queue) RecoverFromCrash() error {
    // Load persisted queue state
    state, err := q.loadState()
    if err != nil {
        return err
    }
    
    // Any item marked "processing" was interrupted - re-queue it
    for _, item := range state.Items {
        if item.Status == "processing" {
            item.Status = "pending"
            item.Progress = 0
        }
        q.items = append(q.items, item)
    }
    
    return q.saveState()
}
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (MVP)

- [ ] Add nomic-embed-text to model management
- [ ] Create `transcript_chunks` table in LanceDB
- [ ] Implement transcript chunking logic (tool-use-centric)
- [ ] Basic regex scrubbing with default patterns
- [ ] Transcript file reading with retry/backoff
- [ ] `pm hook precompact` command
- [ ] Daemon endpoint: `POST /index/transcript`
- [ ] Daemon endpoint: `GET /search/transcripts`
- [ ] Queue deduplication by session_id
- [ ] Update `pm init` to install hooks and add .pommel to .gitignore
- [ ] Update `pm status` with transcript stats
- [ ] Basic `pm search transcripts <query>` command
- [ ] `--limit N` option for search (default: 10)
- [ ] 200-char snippet display with `--full` option

### Phase 2: Annotations & Polish

- [ ] `pm hook user-prompt` with "Note to Pommel:" extraction (anywhere in message)
- [ ] Daemon endpoint: `POST /index/transcript/annotate`
- [ ] Daemon endpoint: `GET /annotations`
- [ ] `pm annotations` command with filtering
- [ ] Temporal filtering (`--since`, `--until`) with natural language support
- [ ] Recency-aware reranking
- [ ] `pm search all` merged results (grouped by source)
- [ ] JSON output for all search commands
- [ ] FTS fallback when daemon not running (with restart prompt)

### Phase 3: Summaries & Intelligence

- [ ] LLM-based summary generation at compaction time
- [ ] LLM scrubbing (opt-in configuration)
- [ ] Queue ETA calculation
- [ ] Performance metrics in status
- [ ] `pm status --watch` (2 second refresh)
- [ ] Model pull progress bars with ETA in `pm init`

### Phase 4: Refinement

- [ ] Queue resilience (re-queue incomplete jobs on restart)
- [ ] Graceful handling of model pull failures (continue with warning)
- [ ] `pm start` no-op when already running
- [ ] Diff-aware staleness detection (flag chunks referencing changed files)
- [ ] Cross-index queries (find transcripts discussing files I'm editing)
- [ ] Session start context injection (optional)
- [ ] MCP server for external agent access

---

## Migration & Compatibility

### Existing Pommel Users

When upgrading to a version with transcript support:

1. `pm init` detects existing installation
2. Adds new hooks to `.claude/settings.json` (idempotent — appends if not present)
3. Creates `transcript_chunks` table in existing LanceDB
4. Adds `.pommel/` to `.gitignore` if not present
5. **Does not** import existing transcripts — waits for new compaction events going forward

Re-indexing of code is handled seamlessly — the "it just works" experience is preserved.

### Claude Code Transcript Format Changes

If Anthropic changes the transcript JSONL format:

1. Pommel release notes will document the change
2. Update includes migration logic to convert existing indexed data
3. Re-indexing happens automatically on daemon start if format version mismatch detected

### Embedding Model Updates

If nomic-embed-text releases a v2 with different dimensions:

1. Detect dimension mismatch on startup
2. Prompt user: "Embedding model updated. Re-index required. Proceed? [Y/n]"
3. Re-index all transcript chunks with new model
4. This is a breaking change — document in release notes

---

## Security Considerations

### Data at Rest

- LanceDB files are unencrypted by default
- Users with sensitive data should use disk encryption (FileVault, LUKS, BitLocker)
- Pommel does not implement its own encryption layer
- `.pommel/` directory contains conversation history — added to `.gitignore` automatically

### Scrubbing Limitations

- Regex patterns may miss novel secret formats
- LLM scrubbing is probabilistic, not guaranteed
- Users should review scrubbing patterns for their specific needs

### License Disclaimer

Pommel is MIT licensed. Users are responsible for:
- Securing their own systems
- Ensuring compliance with their organization's data policies
- Reviewing and customizing scrubbing patterns as needed

---

## Appendix: Claude Code Transcript Format

Example transcript JSONL structure (for reference during implementation):

```jsonl
{"uuid":"abc123","type":"user","timestamp":"2025-01-01T10:00:00Z","message":{"role":"user","content":"Fix the authentication bug in UserService"}}
{"uuid":"def456","type":"assistant","timestamp":"2025-01-01T10:00:05Z","message":{"role":"assistant","content":"I'll look into the authentication issue..."}}
{"uuid":"ghi789","type":"tool_use","timestamp":"2025-01-01T10:00:10Z","tool_name":"Read","tool_input":{"file_path":"src/auth/user_service.go"}}
{"uuid":"jkl012","type":"tool_result","timestamp":"2025-01-01T10:00:11Z","tool_use_id":"ghi789","content":"[file contents]"}
{"uuid":"mno345","type":"assistant","timestamp":"2025-01-01T10:00:15Z","message":{"role":"assistant","content":"I found the issue. The token validation..."}}
{"uuid":"pqr678","type":"tool_use","timestamp":"2025-01-01T10:00:20Z","tool_name":"Edit","tool_input":{"file_path":"src/auth/user_service.go","old_str":"...","new_str":"..."}}
{"uuid":"stu901","type":"tool_result","timestamp":"2025-01-01T10:00:21Z","tool_use_id":"pqr678","content":"File edited successfully"}
```

---

*Document Version: 1.1*
*Created: 2025-01-01*
*Last Updated: 2025-01-01*
