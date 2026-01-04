package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Config struct tests
// ============================================================================

func TestWatcherConfig_DebounceDuration(t *testing.T) {
	tests := []struct {
		name       string
		debounceMs int
		expected   time.Duration
	}{
		{
			name:       "500ms debounce",
			debounceMs: 500,
			expected:   500 * time.Millisecond,
		},
		{
			name:       "1000ms debounce",
			debounceMs: 1000,
			expected:   1 * time.Second,
		},
		{
			name:       "zero debounce",
			debounceMs: 0,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := WatcherConfig{DebounceMs: tt.debounceMs}
			assert.Equal(t, tt.expected, w.DebounceDuration())
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func TestDaemonConfig_Address(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     *int
		expected string
	}{
		{
			name:     "localhost with explicit port",
			host:     "127.0.0.1",
			port:     intPtr(7420),
			expected: "127.0.0.1:7420",
		},
		{
			name:     "custom host and port",
			host:     "0.0.0.0",
			port:     intPtr(8080),
			expected: "0.0.0.0:8080",
		},
		{
			name:     "nil port returns host only",
			host:     "127.0.0.1",
			port:     nil,
			expected: "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DaemonConfig{Host: tt.host, Port: tt.port}
			assert.Equal(t, tt.expected, d.Address())
		})
	}
}

// ============================================================================
// Default config tests
// ============================================================================

func TestDefault(t *testing.T) {
	cfg := Default()

	// Version
	assert.Equal(t, 1, cfg.Version)

	// Chunk levels
	assert.Equal(t, []string{"method", "class", "file"}, cfg.ChunkLevels)

	// Include patterns
	assert.Contains(t, cfg.IncludePatterns, "**/*.cs")
	assert.Contains(t, cfg.IncludePatterns, "**/*.py")
	assert.Contains(t, cfg.IncludePatterns, "**/*.js")
	assert.Contains(t, cfg.IncludePatterns, "**/*.ts")
	assert.Contains(t, cfg.IncludePatterns, "**/*.jsx")
	assert.Contains(t, cfg.IncludePatterns, "**/*.tsx")

	// Exclude patterns
	assert.Contains(t, cfg.ExcludePatterns, "**/node_modules/**")
	assert.Contains(t, cfg.ExcludePatterns, "**/.git/**")
	assert.Contains(t, cfg.ExcludePatterns, "**/.pommel/**")

	// Watcher
	assert.Equal(t, 500, cfg.Watcher.DebounceMs)
	assert.Equal(t, int64(1048576), cfg.Watcher.MaxFileSize)

	// Daemon
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)
	assert.Nil(t, cfg.Daemon.Port, "default port should be nil (hash-based)")
	assert.Equal(t, "info", cfg.Daemon.LogLevel)

	// Embedding
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", cfg.Embedding.Model)
	assert.Equal(t, 32, cfg.Embedding.BatchSize)
	assert.Equal(t, 1000, cfg.Embedding.CacheSize)

	// Search
	assert.Equal(t, 10, cfg.Search.DefaultLimit)
	assert.Equal(t, []string{"method", "class"}, cfg.Search.DefaultLevels)

	// Hybrid Search
	assert.True(t, cfg.Search.Hybrid.Enabled)
	assert.Equal(t, 60, cfg.Search.Hybrid.RRFK)
	assert.Equal(t, 0.7, cfg.Search.Hybrid.VectorWeight)
	assert.Equal(t, 0.3, cfg.Search.Hybrid.KeywordWeight)

	// Reranker
	assert.True(t, cfg.Search.Reranker.Enabled)
	assert.Equal(t, 2000, cfg.Search.Reranker.TimeoutMs)
	assert.Equal(t, 20, cfg.Search.Reranker.Candidates)
	assert.Equal(t, "heuristic", cfg.Search.Reranker.Fallback)
}

func TestDefault_Validation(t *testing.T) {
	cfg := Default()
	errors := Validate(cfg)
	assert.False(t, errors.HasErrors(), "Default config should pass validation")
}

// ============================================================================
// Loader tests
// ============================================================================

func TestNewLoader(t *testing.T) {
	loader := NewLoader("/tmp/test-project")
	assert.NotNil(t, loader)
	assert.Equal(t, "/tmp/test-project", loader.projectRoot)
}

func TestLoader_ConfigPath(t *testing.T) {
	loader := NewLoader("/tmp/test-project")
	expected := filepath.Join("/tmp/test-project", ".pommel", "config.yaml")
	assert.Equal(t, expected, loader.ConfigPath())
}

func TestLoader_PommelDirPath(t *testing.T) {
	loader := NewLoader("/tmp/test-project")
	expected := filepath.Join("/tmp/test-project", ".pommel")
	assert.Equal(t, expected, loader.PommelDirPath())
}

func TestLoader_Exists_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)
	assert.False(t, loader.Exists())
}

func TestLoader_Exists_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("version: 1\n"), 0644))

	loader := NewLoader(tmpDir)
	assert.True(t, loader.Exists())
}

func TestLoader_Init(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	cfg, err := loader.Init()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, cfg.Version)

	// Verify files were created
	assert.True(t, loader.Exists())
	assert.DirExists(t, loader.PommelDirPath())
	assert.FileExists(t, loader.ConfigPath())
}

func TestLoader_Init_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Init first time
	_, err := loader.Init()
	require.NoError(t, err)

	// Init second time should fail
	_, err = loader.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config already exists")
}

func TestLoader_Load_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	_, err := loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestLoader_Load_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Create config via Init
	_, err := loader.Init()
	require.NoError(t, err)

	// Load it back
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)
	assert.Nil(t, cfg.Daemon.Port, "default port should be nil")
}

func TestLoader_LoadOrDefault_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	cfg, err := loader.LoadOrDefault()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, Default().Version, cfg.Version)
}

func TestLoader_LoadOrDefault_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Create config
	_, err := loader.Init()
	require.NoError(t, err)

	// LoadOrDefault should return the saved config
	cfg, err := loader.LoadOrDefault()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoader_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	cfg := Default()
	port := 9999
	cfg.Daemon.Port = &port

	err := loader.Save(cfg)
	require.NoError(t, err)

	// Verify directory and file exist
	assert.DirExists(t, loader.PommelDirPath())
	assert.FileExists(t, loader.ConfigPath())

	// Load and verify
	loaded, err := loader.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded.Daemon.Port)
	assert.Equal(t, 9999, *loaded.Daemon.Port)
}

func TestLoader_Save_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Save initial config
	cfg1 := Default()
	port1 := 1111
	cfg1.Daemon.Port = &port1
	require.NoError(t, loader.Save(cfg1))

	// Save updated config
	cfg2 := Default()
	port2 := 2222
	cfg2.Daemon.Port = &port2
	require.NoError(t, loader.Save(cfg2))

	// Load and verify latest
	loaded, err := loader.Load()
	require.NoError(t, err)
	require.NotNil(t, loaded.Daemon.Port)
	assert.Equal(t, 2222, *loaded.Daemon.Port)
}

func TestLoader_Load_CustomConfig(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 2
chunk_levels:
  - file
  - method
include_patterns:
  - "**/*.go"
exclude_patterns:
  - "**/vendor/**"
watcher:
  debounce_ms: 1000
  max_file_size: 2097152
daemon:
  host: "0.0.0.0"
  port: 8080
  log_level: debug
embedding:
  model: "custom-model"
  batch_size: 64
  cache_size: 2000
search:
  default_limit: 20
  default_levels:
    - file
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, 2, cfg.Version)
	assert.Equal(t, []string{"file", "method"}, cfg.ChunkLevels)
	assert.Equal(t, []string{"**/*.go"}, cfg.IncludePatterns)
	assert.Equal(t, []string{"**/vendor/**"}, cfg.ExcludePatterns)
	assert.Equal(t, 1000, cfg.Watcher.DebounceMs)
	assert.Equal(t, int64(2097152), cfg.Watcher.MaxFileSize)
	assert.Equal(t, "0.0.0.0", cfg.Daemon.Host)
	require.NotNil(t, cfg.Daemon.Port)
	assert.Equal(t, 8080, *cfg.Daemon.Port)
	assert.Equal(t, "debug", cfg.Daemon.LogLevel)
	assert.Equal(t, "custom-model", cfg.Embedding.Model)
	assert.Equal(t, 64, cfg.Embedding.BatchSize)
	assert.Equal(t, 2000, cfg.Embedding.CacheSize)
	assert.Equal(t, 20, cfg.Search.DefaultLimit)
	assert.Equal(t, []string{"file"}, cfg.Search.DefaultLevels)
}

// ============================================================================
// Validation tests
// ============================================================================

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "daemon.port",
		Message: "must be between 1 and 65535",
	}
	assert.Equal(t, "daemon.port: must be between 1 and 65535", err.Error())
}

func TestValidationErrors_Error(t *testing.T) {
	errors := ValidationErrors{
		{Field: "version", Message: "must be at least 1"},
		{Field: "daemon.port", Message: "must be between 1 and 65535"},
	}
	result := errors.Error()
	assert.Contains(t, result, "configuration validation failed")
	assert.Contains(t, result, "version: must be at least 1")
	assert.Contains(t, result, "daemon.port: must be between 1 and 65535")
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		var errors ValidationErrors
		assert.False(t, errors.HasErrors())
	})

	t.Run("with errors", func(t *testing.T) {
		errors := ValidationErrors{
			{Field: "test", Message: "error"},
		}
		assert.True(t, errors.HasErrors())
	})
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := Default()
	errors := Validate(cfg)
	assert.False(t, errors.HasErrors())
}

func TestValidate_InvalidVersion(t *testing.T) {
	cfg := Default()
	cfg.Version = 0

	errors := Validate(cfg)
	assert.True(t, errors.HasErrors())
	assert.Equal(t, "version", errors[0].Field)
}

func TestValidate_InvalidChunkLevels(t *testing.T) {
	t.Run("empty chunk levels", func(t *testing.T) {
		cfg := Default()
		cfg.ChunkLevels = []string{}

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "chunk_levels" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("invalid chunk level value", func(t *testing.T) {
		cfg := Default()
		cfg.ChunkLevels = []string{"method", "invalid", "file"}

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "chunk_levels[1]" {
				found = true
				assert.Contains(t, e.Message, "invalid")
				break
			}
		}
		assert.True(t, found)
	})
}

func TestValidate_InvalidIncludePatterns(t *testing.T) {
	cfg := Default()
	cfg.IncludePatterns = []string{}

	errors := Validate(cfg)
	assert.True(t, errors.HasErrors())
	found := false
	for _, e := range errors {
		if e.Field == "include_patterns" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestValidate_InvalidWatcher(t *testing.T) {
	t.Run("negative debounce", func(t *testing.T) {
		cfg := Default()
		cfg.Watcher.DebounceMs = -1

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "watcher.debounce_ms" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("zero max file size", func(t *testing.T) {
		cfg := Default()
		cfg.Watcher.MaxFileSize = 0

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "watcher.max_file_size" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestValidate_InvalidDaemon(t *testing.T) {
	t.Run("empty host", func(t *testing.T) {
		cfg := Default()
		cfg.Daemon.Host = ""

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "daemon.host" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("port negative", func(t *testing.T) {
		cfg := Default()
		negativePort := -1
		cfg.Daemon.Port = &negativePort

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "daemon.port" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("port too high", func(t *testing.T) {
		cfg := Default()
		highPort := 70000
		cfg.Daemon.Port = &highPort

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "daemon.port" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := Default()
		cfg.Daemon.LogLevel = "invalid"

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "daemon.log_level" {
				found = true
				assert.Contains(t, e.Message, "invalid")
				break
			}
		}
		assert.True(t, found)
	})
}

func TestValidate_InvalidEmbedding(t *testing.T) {
	t.Run("empty model", func(t *testing.T) {
		cfg := Default()
		cfg.Embedding.Model = ""

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "embedding.model" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("zero batch size", func(t *testing.T) {
		cfg := Default()
		cfg.Embedding.BatchSize = 0

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "embedding.batch_size" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("negative cache size", func(t *testing.T) {
		cfg := Default()
		cfg.Embedding.CacheSize = -1

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "embedding.cache_size" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestValidate_InvalidSearch(t *testing.T) {
	t.Run("zero default limit", func(t *testing.T) {
		cfg := Default()
		cfg.Search.DefaultLimit = 0

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "search.default_limit" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("invalid default level", func(t *testing.T) {
		cfg := Default()
		cfg.Search.DefaultLevels = []string{"method", "invalid"}

		errors := Validate(cfg)
		assert.True(t, errors.HasErrors())
		found := false
		for _, e := range errors {
			if e.Field == "search.default_levels[1]" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := Default()
	cfg.Version = 0
	cfg.ChunkLevels = []string{}
	negativePort := -1
	cfg.Daemon.Port = &negativePort

	errors := Validate(cfg)
	assert.True(t, errors.HasErrors())
	assert.GreaterOrEqual(t, len(errors), 3)
}

func TestValidateOrError_Valid(t *testing.T) {
	cfg := Default()
	err := ValidateOrError(cfg)
	assert.NoError(t, err)
}

func TestValidateOrError_Invalid(t *testing.T) {
	cfg := Default()
	cfg.Version = 0

	err := ValidateOrError(cfg)
	assert.Error(t, err)
}

// ============================================================================
// Valid chunk levels tests
// ============================================================================

func TestValidChunkLevels(t *testing.T) {
	validLevels := []string{"file", "class", "section", "method", "block", "line"}
	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			cfg := Default()
			cfg.ChunkLevels = []string{level}
			errors := Validate(cfg)

			// Check no chunk_levels errors
			for _, e := range errors {
				if e.Field == "chunk_levels[0]" {
					t.Errorf("unexpected error for valid chunk level '%s': %s", level, e.Message)
				}
			}
		})
	}
}

func TestValidLogLevels(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			cfg := Default()
			cfg.Daemon.LogLevel = level
			errors := Validate(cfg)

			// Check no log_level errors
			for _, e := range errors {
				if e.Field == "daemon.log_level" {
					t.Errorf("unexpected error for valid log level '%s': %s", level, e.Message)
				}
			}
		})
	}
}

// ============================================================================
// Hybrid search config tests
// ============================================================================

func TestDefaultHybridSearchConfig(t *testing.T) {
	cfg := DefaultHybridSearchConfig()

	assert.True(t, cfg.Enabled, "hybrid search should be enabled by default")
	assert.Equal(t, 60, cfg.RRFK, "default RRFK should be 60")
	assert.Equal(t, 0.7, cfg.VectorWeight, "default vector weight should be 0.7")
	assert.Equal(t, 0.3, cfg.KeywordWeight, "default keyword weight should be 0.3")
}

func TestHybridConfig_DefaultsEnabled(t *testing.T) {
	cfg := Default()
	assert.True(t, cfg.Search.Hybrid.Enabled)
}

func TestHybridConfig_LoadFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
search:
  default_limit: 10
  hybrid:
    enabled: true
    rrf_k: 30
    vector_weight: 0.8
    keyword_weight: 0.2
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.True(t, cfg.Search.Hybrid.Enabled)
	assert.Equal(t, 30, cfg.Search.Hybrid.RRFK)
	assert.Equal(t, 0.8, cfg.Search.Hybrid.VectorWeight)
	assert.Equal(t, 0.2, cfg.Search.Hybrid.KeywordWeight)
}

func TestHybridConfig_DisabledInConfig(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
search:
  default_limit: 10
  hybrid:
    enabled: false
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.False(t, cfg.Search.Hybrid.Enabled)
}

func TestHybridConfig_MissingSection(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	// Config without hybrid section - should use defaults
	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
search:
  default_limit: 10
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	// When hybrid section is missing, values should be zero/false (YAML default)
	// The application should handle this by using defaults when values are zero
	assert.False(t, cfg.Search.Hybrid.Enabled) // YAML default when not specified
}

// ============================================================================
// Reranker config tests
// ============================================================================

func TestDefaultRerankerConfig(t *testing.T) {
	cfg := DefaultRerankerConfig()

	assert.True(t, cfg.Enabled, "reranker should be enabled by default")
	assert.Equal(t, 2000, cfg.TimeoutMs, "default timeout should be 2000ms")
	assert.Equal(t, 20, cfg.Candidates, "default candidates should be 20")
	assert.Equal(t, "heuristic", cfg.Fallback, "default fallback should be heuristic")
}

func TestRerankerConfig_DefaultsEnabled(t *testing.T) {
	cfg := Default()
	assert.True(t, cfg.Search.Reranker.Enabled)
}

func TestRerankerConfig_LoadFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
search:
  default_limit: 10
  reranker:
    enabled: true
    model: "test-model"
    timeout_ms: 3000
    fallback: "none"
    candidates: 30
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.True(t, cfg.Search.Reranker.Enabled)
	assert.Equal(t, "test-model", cfg.Search.Reranker.Model)
	assert.Equal(t, 3000, cfg.Search.Reranker.TimeoutMs)
	assert.Equal(t, "none", cfg.Search.Reranker.Fallback)
	assert.Equal(t, 30, cfg.Search.Reranker.Candidates)
}

func TestRerankerConfig_DisabledInConfig(t *testing.T) {
	tmpDir := t.TempDir()
	pommelDir := filepath.Join(tmpDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `
version: 1
chunk_levels:
  - method
include_patterns:
  - "**/*.go"
search:
  default_limit: 10
  reranker:
    enabled: false
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.False(t, cfg.Search.Reranker.Enabled)
}

// ============================================================================
// Embedding provider config tests
// ============================================================================

func TestEmbeddingConfig_Provider_Defaults(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "ollama", cfg.Embedding.Provider)
	assert.Equal(t, "http://localhost:11434", cfg.Embedding.Ollama.URL)
}

func TestEmbeddingConfig_OpenAIProvider(t *testing.T) {
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
  provider: openai
  batch_size: 32
  openai:
    api_key: sk-test-key
    model: text-embedding-3-small
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "sk-test-key", cfg.Embedding.OpenAI.APIKey)
	assert.Equal(t, "text-embedding-3-small", cfg.Embedding.OpenAI.Model)
}

func TestEmbeddingConfig_VoyageProvider(t *testing.T) {
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
  provider: voyage
  batch_size: 32
  voyage:
    api_key: pa-test-key
    model: voyage-code-3
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "voyage", cfg.Embedding.Provider)
	assert.Equal(t, "pa-test-key", cfg.Embedding.Voyage.APIKey)
	assert.Equal(t, "voyage-code-3", cfg.Embedding.Voyage.Model)
}

func TestEmbeddingConfig_OllamaRemoteProvider(t *testing.T) {
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
  batch_size: 32
  ollama:
    url: http://remote-ollama:11434
    model: custom-model
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
	assert.Equal(t, "http://remote-ollama:11434", cfg.Embedding.Ollama.URL)
	assert.Equal(t, "custom-model", cfg.Embedding.Ollama.Model)
}

func TestEmbeddingConfig_OpenAI_APIKeyFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	cfg := &EmbeddingConfig{
		Provider: "openai",
		OpenAI:   OpenAIProviderConfig{}, // No API key in config
	}

	apiKey := cfg.GetOpenAIAPIKey()
	assert.Equal(t, "sk-from-env", apiKey)
}

func TestEmbeddingConfig_Voyage_APIKeyFromEnv(t *testing.T) {
	t.Setenv("VOYAGE_API_KEY", "pa-from-env")

	cfg := &EmbeddingConfig{
		Provider: "voyage",
		Voyage:   VoyageProviderConfig{}, // No API key in config
	}

	apiKey := cfg.GetVoyageAPIKey()
	assert.Equal(t, "pa-from-env", apiKey)
}

func TestEmbeddingConfig_Ollama_URLFromEnv(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://env-ollama:11434")

	cfg := &EmbeddingConfig{
		Provider: "ollama",
		Ollama:   OllamaProviderConfig{}, // No URL in config
	}

	url := cfg.GetOllamaURL()
	assert.Equal(t, "http://env-ollama:11434", url)
}

func TestEmbeddingConfig_ConfigOverridesEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	cfg := &EmbeddingConfig{
		Provider: "openai",
		OpenAI:   OpenAIProviderConfig{APIKey: "sk-from-config"},
	}

	apiKey := cfg.GetOpenAIAPIKey()
	assert.Equal(t, "sk-from-config", apiKey)
}

func TestEmbeddingConfig_LegacyOllamaURL(t *testing.T) {
	// Support legacy ollama_url field for backwards compatibility
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
  model: jina-embeddings-v2-base-code
  ollama_url: http://legacy-ollama:11434
  batch_size: 32
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	loader := NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	// Legacy config should still work - GetOllamaURL should return legacy URL
	url := cfg.Embedding.GetOllamaURL()
	assert.Equal(t, "http://legacy-ollama:11434", url)
}

func TestEmbeddingConfig_ProviderDimensions(t *testing.T) {
	tests := []struct {
		provider   string
		dimensions int
	}{
		{"ollama", 768},
		{"ollama-remote", 768},
		{"openai", 1536},
		{"voyage", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := &EmbeddingConfig{Provider: tt.provider}
			assert.Equal(t, tt.dimensions, cfg.DefaultDimensions())
		})
	}
}
