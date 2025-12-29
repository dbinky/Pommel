# Phase 1: Foundation

**Phase Goal:** Establish the project structure, build system, and basic components so that subsequent phases have a solid foundation to build upon.

**Prerequisites:** None (this is the first phase)

**Estimated Tasks:** 15 tasks across 4 areas

---

## Table of Contents

1. [Overview](#overview)
2. [Success Criteria](#success-criteria)
3. [Task 1.1: Project Setup](#task-11-project-setup)
4. [Task 1.2: Build System](#task-12-build-system)
5. [Task 1.3: Configuration System](#task-13-configuration-system)
6. [Task 1.4: Database Foundation](#task-14-database-foundation)
7. [Task 1.5: CLI Skeleton](#task-15-cli-skeleton)
8. [Dependencies](#dependencies)
9. [Testing Strategy](#testing-strategy)
10. [Risks and Mitigations](#risks-and-mitigations)

---

## Overview

Phase 1 creates the skeleton of the Pommel project. By the end of this phase:

- The project compiles with `go build`
- Tests run with `go test`
- Configuration loads from YAML files
- SQLite database initializes with sqlite-vec extension
- CLI has a working `pm version` command

This phase intentionally avoids any "real" functionality (embedding, chunking, watching). It's purely about infrastructure.

---

## Success Criteria

| Criterion | Validation |
|-----------|------------|
| Project compiles | `make build` succeeds |
| Tests pass | `make test` shows 100% pass rate |
| Config loads | Unit test reads sample config.yaml |
| Database initializes | Unit test creates tables with sqlite-vec |
| CLI runs | `./pm version` prints version string |

---

## Task 1.1: Project Setup

### 1.1.1 Initialize Go Module

**Description:** Create the Go module and establish the project identity.

**Steps:**
1. Run `go mod init github.com/pommel-dev/pommel`
2. Create initial `go.mod` with Go 1.21+ requirement

**Files Created:**
- `go.mod`

**Acceptance Criteria:**
- `go mod tidy` runs without errors
- Module path follows Go conventions

---

### 1.1.2 Create Directory Structure

**Description:** Create all directories as specified in the design document.

**Steps:**
1. Create the following directory tree:

```
pommel/
├── cmd/
│   ├── pm/
│   └── pommeld/
├── internal/
│   ├── api/
│   ├── chunker/
│   ├── cli/
│   ├── config/
│   ├── daemon/
│   ├── db/
│   ├── embedder/
│   ├── models/
│   └── setup/
├── pkg/
│   └── client/
├── testdata/
│   ├── csharp/
│   ├── python/
│   ├── javascript/
│   └── typescript/
└── docs/
    └── plans/
```

2. Add `.gitkeep` files to empty directories to ensure they're tracked

**Files Created:**
- All directories listed above
- `.gitkeep` in empty directories

**Acceptance Criteria:**
- All directories exist
- `tree` command shows expected structure

---

### 1.1.3 Create .gitignore

**Description:** Set up Git ignore patterns for Go projects.

**Steps:**
1. Create `.gitignore` with standard Go patterns

**File Content (.gitignore):**
```gitignore
# Binaries
/pm
/pommeld
*.exe
*.exe~
*.dll
*.so
*.dylib

# Build output
/bin/
/dist/

# Test binary
*.test

# Coverage
*.out
coverage.html

# Go workspace
go.work
go.work.sum

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Project specific
.pommel/
*.log
```

**Acceptance Criteria:**
- `.gitignore` exists and contains all patterns
- Build artifacts are not tracked by Git

---

### 1.1.4 Create Entry Points

**Description:** Create the main.go files for both binaries.

**Steps:**
1. Create `cmd/pm/main.go` with minimal CLI bootstrap
2. Create `cmd/pommeld/main.go` with minimal daemon bootstrap

**File Content (cmd/pm/main.go):**
```go
package main

import (
	"fmt"
	"os"

	"github.com/pommel-dev/pommel/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**File Content (cmd/pommeld/main.go):**
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("pommeld: not yet implemented")
	os.Exit(0)
}
```

**Acceptance Criteria:**
- Both files compile
- `go run cmd/pm/main.go` executes without panic
- `go run cmd/pommeld/main.go` prints placeholder message

---

## Task 1.2: Build System

### 1.2.1 Create Makefile

**Description:** Create a Makefile with standard build targets.

**Steps:**
1. Create `Makefile` with the following targets

**File Content (Makefile):**
```makefile
# Pommel Makefile

# Variables
BINARY_CLI = pm
BINARY_DAEMON = pommeld
BUILD_DIR = bin
GO = go
GOFLAGS = -trimpath
LDFLAGS = -s -w -X main.version=$(VERSION)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
.PHONY: all
all: build

# Build both binaries
.PHONY: build
build: build-cli build-daemon

.PHONY: build-cli
build-cli:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/pm

.PHONY: build-daemon
build-daemon:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_DAEMON) ./cmd/pommeld

# Run tests
.PHONY: test
test:
	$(GO) test -v -race -cover ./...

# Run tests with coverage report
.PHONY: coverage
coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format code
.PHONY: fmt
fmt:
	$(GO) fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean -cache -testcache

# Install binaries to GOPATH/bin
.PHONY: install
install:
	$(GO) install ./cmd/pm
	$(GO) install ./cmd/pommeld

# Run the CLI (for development)
.PHONY: run-cli
run-cli:
	$(GO) run ./cmd/pm $(ARGS)

# Run the daemon (for development)
.PHONY: run-daemon
run-daemon:
	$(GO) run ./cmd/pommeld $(ARGS)

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Download dependencies
.PHONY: deps
deps:
	$(GO) mod download

# Show help
.PHONY: help
help:
	@echo "Pommel Build Targets:"
	@echo "  make build      - Build both binaries"
	@echo "  make build-cli  - Build CLI only"
	@echo "  make build-daemon - Build daemon only"
	@echo "  make test       - Run tests with race detection"
	@echo "  make coverage   - Generate coverage report"
	@echo "  make lint       - Run golangci-lint"
	@echo "  make fmt        - Format code"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make install    - Install binaries to GOPATH/bin"
	@echo "  make tidy       - Tidy go.mod"
	@echo "  make deps       - Download dependencies"
```

**Acceptance Criteria:**
- `make build` creates binaries in `bin/`
- `make test` runs tests
- `make clean` removes artifacts
- `make help` shows available targets

---

### 1.2.2 Add golangci-lint Configuration

**Description:** Configure the Go linter for consistent code quality.

**Steps:**
1. Create `.golangci.yml` with project settings

**File Content (.golangci.yml):**
```yaml
run:
  timeout: 5m
  go: "1.21"

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam
    - gosec
    - prealloc

linters-settings:
  errcheck:
    check-type-assertions: true
  govet:
    check-shadowing: true
  goimports:
    local-prefixes: github.com/pommel-dev/pommel
  misspell:
    locale: US

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
        - gosec
```

**Acceptance Criteria:**
- `make lint` runs without configuration errors
- Linter catches common issues

---

## Task 1.3: Configuration System

### 1.3.1 Define Configuration Structs

**Description:** Create Go structs that represent the configuration schema.

**Steps:**
1. Create `internal/config/config.go` with struct definitions
2. Add struct tags for YAML mapping and validation

**File Content (internal/config/config.go):**
```go
package config

import "time"

// Config represents the complete Pommel configuration
type Config struct {
	Version         int              `yaml:"version"`
	ChunkLevels     []string         `yaml:"chunk_levels"`
	IncludePatterns []string         `yaml:"include_patterns"`
	ExcludePatterns []string         `yaml:"exclude_patterns"`
	Watcher         WatcherConfig    `yaml:"watcher"`
	Daemon          DaemonConfig     `yaml:"daemon"`
	Embedding       EmbeddingConfig  `yaml:"embedding"`
	Search          SearchConfig     `yaml:"search"`
}

// WatcherConfig contains file watcher settings
type WatcherConfig struct {
	DebounceMs  int   `yaml:"debounce_ms"`
	MaxFileSize int64 `yaml:"max_file_size"`
}

// DaemonConfig contains daemon server settings
type DaemonConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

// EmbeddingConfig contains embedding model settings
type EmbeddingConfig struct {
	Model     string `yaml:"model"`
	BatchSize int    `yaml:"batch_size"`
	CacheSize int    `yaml:"cache_size"`
}

// SearchConfig contains search default settings
type SearchConfig struct {
	DefaultLimit  int      `yaml:"default_limit"`
	DefaultLevels []string `yaml:"default_levels"`
}

// DebounceDuration returns the debounce duration as time.Duration
func (w WatcherConfig) DebounceDuration() time.Duration {
	return time.Duration(w.DebounceMs) * time.Millisecond
}

// Address returns the full host:port address for the daemon
func (d DaemonConfig) Address() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}
```

**Acceptance Criteria:**
- Structs compile without errors
- All configuration options from design doc are represented
- Helper methods work correctly

---

### 1.3.2 Define Default Configuration

**Description:** Create default values that are used when no config file exists.

**Steps:**
1. Create `internal/config/defaults.go` with default values

**File Content (internal/config/defaults.go):**
```go
package config

// Default returns a Config with sensible default values
func Default() *Config {
	return &Config{
		Version: 1,
		ChunkLevels: []string{
			"method",
			"class",
			"file",
		},
		IncludePatterns: []string{
			"**/*.cs",
			"**/*.py",
			"**/*.js",
			"**/*.ts",
			"**/*.jsx",
			"**/*.tsx",
		},
		ExcludePatterns: []string{
			"**/node_modules/**",
			"**/bin/**",
			"**/obj/**",
			"**/__pycache__/**",
			"**/.git/**",
			"**/.pommel/**",
		},
		Watcher: WatcherConfig{
			DebounceMs:  500,
			MaxFileSize: 1048576, // 1MB
		},
		Daemon: DaemonConfig{
			Host:     "127.0.0.1",
			Port:     7420,
			LogLevel: "info",
		},
		Embedding: EmbeddingConfig{
			Model:     "unclemusclez/jina-embeddings-v2-base-code",
			BatchSize: 32,
			CacheSize: 1000,
		},
		Search: SearchConfig{
			DefaultLimit: 10,
			DefaultLevels: []string{
				"method",
				"class",
			},
		},
	}
}
```

**Acceptance Criteria:**
- `Default()` returns a valid config
- Default values match design document specifications

---

### 1.3.3 Implement Configuration Loading

**Description:** Use Viper to load configuration from YAML files.

**Steps:**
1. Add Viper to the dependencies
2. Create `internal/config/loader.go` with loading logic
3. Support loading from `.pommel/config.yaml`
4. Merge defaults with file config

**File Content (internal/config/loader.go):**
```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	// ConfigDir is the directory name for Pommel configuration
	ConfigDir = ".pommel"
	// ConfigFile is the configuration file name
	ConfigFile = "config.yaml"
)

// Loader handles configuration loading and management
type Loader struct {
	projectRoot string
	viper       *viper.Viper
}

// NewLoader creates a new configuration loader for the given project root
func NewLoader(projectRoot string) *Loader {
	return &Loader{
		projectRoot: projectRoot,
		viper:       viper.New(),
	}
}

// Load reads the configuration from disk, merging with defaults
func (l *Loader) Load() (*Config, error) {
	cfg := Default()

	configPath := l.ConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No config file exists, use defaults
		return cfg, nil
	}

	l.viper.SetConfigFile(configPath)
	l.viper.SetConfigType("yaml")

	if err := l.viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := l.viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to disk
func (l *Loader) Save(cfg *Config) error {
	configDir := l.ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	l.viper.Set("version", cfg.Version)
	l.viper.Set("chunk_levels", cfg.ChunkLevels)
	l.viper.Set("include_patterns", cfg.IncludePatterns)
	l.viper.Set("exclude_patterns", cfg.ExcludePatterns)
	l.viper.Set("watcher", cfg.Watcher)
	l.viper.Set("daemon", cfg.Daemon)
	l.viper.Set("embedding", cfg.Embedding)
	l.viper.Set("search", cfg.Search)

	return l.viper.WriteConfigAs(l.ConfigPath())
}

// ConfigDir returns the path to the .pommel directory
func (l *Loader) ConfigDir() string {
	return filepath.Join(l.projectRoot, ConfigDir)
}

// ConfigPath returns the path to config.yaml
func (l *Loader) ConfigPath() string {
	return filepath.Join(l.ConfigDir(), ConfigFile)
}

// Exists checks if the project has been initialized
func (l *Loader) Exists() bool {
	_, err := os.Stat(l.ConfigDir())
	return err == nil
}
```

**Acceptance Criteria:**
- `Load()` returns defaults when no file exists
- `Load()` merges file config with defaults
- `Save()` creates config file
- `Exists()` correctly detects initialization state

---

### 1.3.4 Add Configuration Validation

**Description:** Validate configuration values to catch errors early.

**Steps:**
1. Create `internal/config/validate.go` with validation logic

**File Content (internal/config/validate.go):**
```go
package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config.%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	var errs ValidationErrors

	// Version
	if c.Version < 1 {
		errs = append(errs, ValidationError{
			Field:   "version",
			Message: "must be at least 1",
		})
	}

	// Chunk levels
	validLevels := map[string]bool{"file": true, "class": true, "method": true}
	for _, level := range c.ChunkLevels {
		if !validLevels[level] {
			errs = append(errs, ValidationError{
				Field:   "chunk_levels",
				Message: fmt.Sprintf("invalid level: %s (valid: file, class, method)", level),
			})
		}
	}

	// Include patterns
	if len(c.IncludePatterns) == 0 {
		errs = append(errs, ValidationError{
			Field:   "include_patterns",
			Message: "must have at least one pattern",
		})
	}

	// Watcher
	if c.Watcher.DebounceMs < 0 {
		errs = append(errs, ValidationError{
			Field:   "watcher.debounce_ms",
			Message: "must be non-negative",
		})
	}
	if c.Watcher.MaxFileSize < 1024 {
		errs = append(errs, ValidationError{
			Field:   "watcher.max_file_size",
			Message: "must be at least 1024 bytes",
		})
	}

	// Daemon
	if c.Daemon.Port < 1 || c.Daemon.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "daemon.port",
			Message: "must be between 1 and 65535",
		})
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Daemon.LogLevel] {
		errs = append(errs, ValidationError{
			Field:   "daemon.log_level",
			Message: fmt.Sprintf("invalid level: %s (valid: debug, info, warn, error)", c.Daemon.LogLevel),
		})
	}

	// Embedding
	if c.Embedding.Model == "" {
		errs = append(errs, ValidationError{
			Field:   "embedding.model",
			Message: "must not be empty",
		})
	}
	if c.Embedding.BatchSize < 1 {
		errs = append(errs, ValidationError{
			Field:   "embedding.batch_size",
			Message: "must be at least 1",
		})
	}

	// Search
	if c.Search.DefaultLimit < 1 {
		errs = append(errs, ValidationError{
			Field:   "search.default_limit",
			Message: "must be at least 1",
		})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
```

**Acceptance Criteria:**
- Invalid configs return descriptive errors
- Valid configs pass validation
- All fields are validated

---

### 1.3.5 Write Configuration Tests

**Description:** Create comprehensive tests for the configuration system.

**Steps:**
1. Create `internal/config/config_test.go`
2. Test default values, loading, saving, and validation

**File Content (internal/config/config_test.go):**
```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	assert.Equal(t, 1, cfg.Version)
	assert.Contains(t, cfg.ChunkLevels, "method")
	assert.Contains(t, cfg.ChunkLevels, "class")
	assert.Contains(t, cfg.ChunkLevels, "file")
	assert.Equal(t, 7420, cfg.Daemon.Port)
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", cfg.Embedding.Model)
}

func TestDefaultValidation(t *testing.T) {
	cfg := Default()
	err := cfg.Validate()
	assert.NoError(t, err, "default config should be valid")
}

func TestLoader_LoadWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	cfg, err := loader.Load()
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, Default().Daemon.Port, cfg.Daemon.Port)
}

func TestLoader_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Modify config
	cfg := Default()
	cfg.Daemon.Port = 9999
	cfg.Search.DefaultLimit = 50

	// Save
	err := loader.Save(cfg)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, filepath.Join(tmpDir, ConfigDir, ConfigFile))

	// Load
	loaded, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, 9999, loaded.Daemon.Port)
	assert.Equal(t, 50, loaded.Search.DefaultLimit)
}

func TestLoader_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	assert.False(t, loader.Exists())

	// Create config dir
	err := os.MkdirAll(loader.ConfigDir(), 0755)
	require.NoError(t, err)

	assert.True(t, loader.Exists())
}

func TestValidation_InvalidChunkLevel(t *testing.T) {
	cfg := Default()
	cfg.ChunkLevels = []string{"invalid"}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid level")
}

func TestValidation_InvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Daemon.Port = 0

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidation_EmptyModel(t *testing.T) {
	cfg := Default()
	cfg.Embedding.Model = ""

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestDaemonConfig_Address(t *testing.T) {
	cfg := DaemonConfig{Host: "127.0.0.1", Port: 7420}
	assert.Equal(t, "127.0.0.1:7420", cfg.Address())
}

func TestWatcherConfig_DebounceDuration(t *testing.T) {
	cfg := WatcherConfig{DebounceMs: 500}
	assert.Equal(t, 500*time.Millisecond, cfg.DebounceDuration())
}
```

**Acceptance Criteria:**
- All tests pass
- Tests cover happy path and error cases
- At least 80% code coverage for config package

---

## Task 1.4: Database Foundation

### 1.4.1 Create Database Connection Manager

**Description:** Set up SQLite connection with sqlite-vec extension.

**Steps:**
1. Add sqlite-vec and ncruces/go-sqlite3 dependencies
2. Create `internal/db/sqlite.go` with connection management

**File Content (internal/db/sqlite.go):**
```go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

const (
	// DatabaseFile is the name of the sqlite database file
	DatabaseFile = "index.db"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
	path string
}

// Open creates a new database connection
func Open(projectRoot string) (*DB, error) {
	// Register sqlite-vec extension
	sqlite_vec.Auto()

	dbDir := filepath.Join(projectRoot, ".pommel")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, DatabaseFile)

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection
	if _, err := conn.Exec("PRAGMA journal_mode = WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn, path: dbPath}

	// Verify sqlite-vec is available
	if err := db.verifySqliteVec(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}

	return db, nil
}

// verifySqliteVec checks that the sqlite-vec extension is loaded
func (db *DB) verifySqliteVec() error {
	var version string
	err := db.conn.QueryRow("SELECT vec_version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("vec_version() failed: %w", err)
	}
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// Conn returns the underlying sql.DB connection
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Exec executes a query without returning rows
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}
```

**Acceptance Criteria:**
- Database file is created in `.pommel/index.db`
- WAL mode is enabled
- sqlite-vec extension is verified

---

### 1.4.2 Implement Schema Migrations

**Description:** Create the database schema with version tracking.

**Steps:**
1. Create `internal/db/schema.go` with migration logic
2. Implement initial schema (version 1)

**File Content (internal/db/schema.go):**
```go
package db

import (
	"context"
	"fmt"
)

const currentSchemaVersion = 1

// migrations maps version numbers to migration SQL
var migrations = map[int]string{
	1: `
		-- Main chunks table
		CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			file_path TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			level TEXT NOT NULL CHECK (level IN ('file', 'class', 'method')),
			language TEXT NOT NULL,
			content TEXT NOT NULL,
			parent_id TEXT REFERENCES chunks(id) ON DELETE SET NULL,
			name TEXT,
			content_hash TEXT NOT NULL,
			last_modified TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		-- Indexes for common queries
		CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
		CREATE INDEX IF NOT EXISTS idx_chunks_level ON chunks(level);
		CREATE INDEX IF NOT EXISTS idx_chunks_parent_id ON chunks(parent_id);
		CREATE INDEX IF NOT EXISTS idx_chunks_content_hash ON chunks(content_hash);

		-- Virtual table for vector search (sqlite-vec)
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_embeddings USING vec0(
			chunk_id TEXT PRIMARY KEY,
			embedding FLOAT[768]
		);

		-- Files table for tracking indexed files
		CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY,
			content_hash TEXT NOT NULL,
			last_modified TEXT NOT NULL,
			last_indexed TEXT NOT NULL,
			chunk_count INTEGER NOT NULL DEFAULT 0
		);

		-- Metadata table for schema version, etc.
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		-- Initialize schema version
		INSERT OR IGNORE INTO metadata (key, value) VALUES ('schema_version', '1');
	`,
}

// Migrate runs all pending migrations
func (db *DB) Migrate(ctx context.Context) error {
	currentVersion, err := db.getSchemaVersion(ctx)
	if err != nil {
		// Table doesn't exist yet, start from 0
		currentVersion = 0
	}

	for version := currentVersion + 1; version <= currentSchemaVersion; version++ {
		migration, ok := migrations[version]
		if !ok {
			return fmt.Errorf("missing migration for version %d", version)
		}

		if _, err := db.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", version, err)
		}

		if err := db.setSchemaVersion(ctx, version); err != nil {
			return fmt.Errorf("failed to update schema version: %w", err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version
func (db *DB) getSchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := db.QueryRow(ctx, "SELECT CAST(value AS INTEGER) FROM metadata WHERE key = 'schema_version'").Scan(&version)
	return version, err
}

// setSchemaVersion updates the schema version
func (db *DB) setSchemaVersion(ctx context.Context, version int) error {
	_, err := db.Exec(ctx, "UPDATE metadata SET value = ? WHERE key = 'schema_version'", fmt.Sprintf("%d", version))
	return err
}

// SchemaVersion returns the current schema version
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	return db.getSchemaVersion(ctx)
}
```

**Acceptance Criteria:**
- Schema creates all required tables
- Migrations are idempotent
- Version tracking works correctly

---

### 1.4.3 Write Database Tests

**Description:** Test database connection and schema creation.

**Steps:**
1. Create `internal/db/db_test.go`

**File Content (internal/db/db_test.go):**
```go
package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	assert.FileExists(t, db.Path())
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Verify tables exist
	tables := []string{"chunks", "chunk_embeddings", "files", "metadata"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
	}

	// Verify schema version
	version, err := db.SchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestMigrate_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	// Run migration twice
	err = db.Migrate(ctx)
	require.NoError(t, err)

	err = db.Migrate(ctx)
	require.NoError(t, err)

	version, err := db.SchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestSqliteVec_Available(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	db, err := Open(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Test vec_version()
	var version string
	err = db.QueryRow(ctx, "SELECT vec_version()").Scan(&version)
	require.NoError(t, err)
	assert.NotEmpty(t, version)
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := Open(tmpDir)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	// Verify connection is closed
	_, err = db.Exec(context.Background(), "SELECT 1")
	assert.Error(t, err)
}
```

**Acceptance Criteria:**
- All tests pass
- Database creates successfully
- sqlite-vec is available and working

---

## Task 1.5: CLI Skeleton

### 1.5.1 Create Root Command

**Description:** Set up Cobra root command with global flags.

**Steps:**
1. Add Cobra dependency
2. Create `internal/cli/root.go`

**File Content (internal/cli/root.go):**
```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	jsonOutput  bool
	verbose     bool
	projectRoot string
)

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "pm",
	Short: "Pommel - Semantic code search for AI agents",
	Long: `Pommel is a local-first semantic code search system designed to reduce
context window consumption for AI coding agents.

It maintains an always-current vector database of code embeddings,
enabling targeted semantic searches rather than reading numerous
files into context.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&projectRoot, "project", "p", "", "Project root directory (default: current directory)")

	// Set default project root
	cobra.OnInitialize(initProjectRoot)
}

func initProjectRoot() {
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
			os.Exit(1)
		}
	}
}

// GetProjectRoot returns the project root directory
func GetProjectRoot() string {
	return projectRoot
}

// IsJSONOutput returns true if JSON output is enabled
func IsJSONOutput() bool {
	return jsonOutput
}

// IsVerbose returns true if verbose output is enabled
func IsVerbose() bool {
	return verbose
}
```

**Acceptance Criteria:**
- `pm` runs without error
- `pm --help` shows help text
- Global flags are registered

---

### 1.5.2 Create Version Command

**Description:** Add a version command to display build information.

**Steps:**
1. Create `internal/cli/version.go`
2. Wire version from build flags

**File Content (internal/cli/version.go):**
```go
package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build information (set at compile time via ldflags)
var (
	BuildCommit = "unknown"
	BuildDate   = "unknown"
)

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Display the version, build commit, and other build information.",
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := versionInfo{
		Version:   Version,
		Commit:    BuildCommit,
		Date:      BuildDate,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if jsonOutput {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal version info: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Pommel %s\n", info.Version)
		fmt.Printf("  Commit:     %s\n", info.Commit)
		fmt.Printf("  Built:      %s\n", info.Date)
		fmt.Printf("  Go version: %s\n", info.GoVersion)
		fmt.Printf("  OS/Arch:    %s/%s\n", info.OS, info.Arch)
	}

	return nil
}
```

**Acceptance Criteria:**
- `pm version` outputs version info
- `pm version --json` outputs JSON format
- Build info is populated via ldflags

---

### 1.5.3 Create Output Formatter

**Description:** Create a utility for consistent CLI output formatting.

**Steps:**
1. Create `internal/cli/output.go`

**File Content (internal/cli/output.go):**
```go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// Output handles consistent CLI output formatting
type Output struct {
	json bool
}

// NewOutput creates a new output handler
func NewOutput() *Output {
	return &Output{json: jsonOutput}
}

// Success prints a success message
func (o *Output) Success(format string, args ...any) {
	if o.json {
		return // Success messages are not included in JSON output
	}
	fmt.Printf("✓ "+format+"\n", args...)
}

// Info prints an informational message
func (o *Output) Info(format string, args ...any) {
	if o.json {
		return
	}
	fmt.Printf(format+"\n", args...)
}

// Warn prints a warning message
func (o *Output) Warn(format string, args ...any) {
	if o.json {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

// Error prints an error message
func (o *Output) Error(format string, args ...any) {
	if o.json {
		o.JSON(map[string]any{
			"error": fmt.Sprintf(format, args...),
		})
		return
	}
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// JSON outputs data as JSON
func (o *Output) JSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Table prints data in a tabular format
func (o *Output) Table(headers []string, rows [][]string) {
	if o.json {
		// Convert to list of maps for JSON
		var data []map[string]string
		for _, row := range rows {
			rowMap := make(map[string]string)
			for i, header := range headers {
				if i < len(row) {
					rowMap[header] = row[i]
				}
			}
			data = append(data, rowMap)
		}
		o.JSON(data)
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Printf("%-*s  ", widths[i], h)
	}
	fmt.Println()

	// Print separator
	for i := range headers {
		for j := 0; j < widths[i]; j++ {
			fmt.Print("-")
		}
		fmt.Print("  ")
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Println()
	}
}
```

**Acceptance Criteria:**
- Output formatter handles JSON and text modes
- Table formatting works correctly
- Error output goes to stderr

---

### 1.5.4 Update Main Entry Point

**Description:** Update cmd/pm/main.go to use proper version injection.

**Steps:**
1. Update `cmd/pm/main.go` with version variables
2. Update Makefile ldflags

**File Content (cmd/pm/main.go) - Updated:**
```go
package main

import (
	"fmt"
	"os"

	"github.com/pommel-dev/pommel/internal/cli"
)

// Set at build time via ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set version info
	cli.Version = version
	cli.BuildCommit = commit
	cli.BuildDate = date

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Updated Makefile ldflags:**
```makefile
LDFLAGS = -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
	-X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
```

**Acceptance Criteria:**
- Version command shows commit and date when built with make
- Dev builds show "dev" version

---

### 1.5.5 Write CLI Tests

**Description:** Test CLI commands work correctly.

**Steps:**
1. Create `internal/cli/cli_test.go`

**File Content (internal/cli/cli_test.go):**
```go
package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	// Reset flags for test
	rootCmd.SetArgs([]string{})

	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestVersionCommand(t *testing.T) {
	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Pommel")
}

func TestVersionCommand_JSON(t *testing.T) {
	// Enable JSON output
	jsonOutput = true
	defer func() { jsonOutput = false }()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	// Note: This test is tricky because output goes to stdout
	// In real tests, we'd need to capture stdout
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestOutput_JSON(t *testing.T) {
	// Save and restore stdout
	jsonOutput = true
	defer func() { jsonOutput = false }()

	out := NewOutput()

	data := map[string]string{"key": "value"}
	err := out.JSON(data)
	assert.NoError(t, err)
}

func TestOutput_Table(t *testing.T) {
	jsonOutput = false
	out := NewOutput()

	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"foo", "bar"},
		{"baz", "qux"},
	}

	// Should not panic
	out.Table(headers, rows)
}
```

**Acceptance Criteria:**
- All tests pass
- CLI commands execute without error
- JSON output is valid JSON

---

## Dependencies

### Go Modules Required

```
github.com/spf13/cobra v1.8.0
github.com/spf13/viper v1.18.0
github.com/ncruces/go-sqlite3 v0.18.0
github.com/asg017/sqlite-vec-go-bindings v0.1.0
github.com/stretchr/testify v1.8.4
```

### Installation Command

```bash
go get github.com/spf13/cobra@v1.8.0
go get github.com/spf13/viper@v1.18.0
go get github.com/ncruces/go-sqlite3@latest
go get github.com/asg017/sqlite-vec-go-bindings/ncruces@latest
go get github.com/stretchr/testify@v1.8.4
```

---

## Testing Strategy

### Unit Tests

Each package should have `*_test.go` files with:
- Table-driven tests where appropriate
- Mocking of external dependencies
- Coverage of happy path and error cases

### Integration Tests

- Test config load/save with real files
- Test database creation and migration
- Test CLI command execution

### Coverage Target

- Minimum 80% coverage for all packages
- 100% coverage for critical paths (config validation, schema migration)

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| sqlite-vec WASM has issues on macOS | Low | High | Test early on both macOS and Linux |
| Viper config merge has edge cases | Medium | Medium | Write thorough tests for config merging |
| Version ldflags not set correctly | Low | Low | Test both `go run` and `make build` |

---

## Checklist

Before marking Phase 1 complete:

- [ ] `make build` succeeds
- [ ] `make test` shows all tests passing
- [ ] `make lint` shows no errors
- [ ] `./bin/pm version` prints version info
- [ ] Config loading works (test with sample file)
- [ ] Database creates with all tables
- [ ] sqlite-vec is verified working
- [ ] All code is committed and documented
