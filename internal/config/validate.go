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
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("configuration validation failed:\n")
	for _, err := range e {
		sb.WriteString("  - ")
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

// HasErrors returns true if there are any validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// validChunkLevels defines the allowed chunk level values
var validChunkLevels = map[string]bool{
	"file":   true,
	"class":  true,
	"method": true,
	"block":  true,
	"line":   true,
}

// validLogLevels defines the allowed log level values
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Validate checks the configuration for errors and returns all validation errors found
func Validate(cfg *Config) ValidationErrors {
	var errors ValidationErrors

	// Version validation
	if cfg.Version < 1 {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "must be at least 1",
		})
	}

	// Chunk levels validation
	if len(cfg.ChunkLevels) == 0 {
		errors = append(errors, ValidationError{
			Field:   "chunk_levels",
			Message: "must specify at least one chunk level",
		})
	} else {
		for i, level := range cfg.ChunkLevels {
			if !validChunkLevels[level] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("chunk_levels[%d]", i),
					Message: fmt.Sprintf("invalid chunk level '%s'; valid values are: file, class, method, block, line", level),
				})
			}
		}
	}

	// Include patterns validation
	if len(cfg.IncludePatterns) == 0 {
		errors = append(errors, ValidationError{
			Field:   "include_patterns",
			Message: "must specify at least one include pattern",
		})
	}

	// Watcher validation
	if cfg.Watcher.DebounceMs < 0 {
		errors = append(errors, ValidationError{
			Field:   "watcher.debounce_ms",
			Message: "must be non-negative",
		})
	}
	if cfg.Watcher.MaxFileSize <= 0 {
		errors = append(errors, ValidationError{
			Field:   "watcher.max_file_size",
			Message: "must be positive",
		})
	}

	// Daemon validation
	if cfg.Daemon.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "daemon.host",
			Message: "must not be empty",
		})
	}
	if cfg.Daemon.Port < 1 || cfg.Daemon.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "daemon.port",
			Message: "must be between 1 and 65535",
		})
	}
	if !validLogLevels[cfg.Daemon.LogLevel] {
		errors = append(errors, ValidationError{
			Field:   "daemon.log_level",
			Message: fmt.Sprintf("invalid log level '%s'; valid values are: debug, info, warn, error", cfg.Daemon.LogLevel),
		})
	}

	// Embedding validation
	if cfg.Embedding.Model == "" {
		errors = append(errors, ValidationError{
			Field:   "embedding.model",
			Message: "must not be empty",
		})
	}
	if cfg.Embedding.BatchSize < 1 {
		errors = append(errors, ValidationError{
			Field:   "embedding.batch_size",
			Message: "must be at least 1",
		})
	}
	if cfg.Embedding.CacheSize < 0 {
		errors = append(errors, ValidationError{
			Field:   "embedding.cache_size",
			Message: "must be non-negative",
		})
	}

	// Search validation
	if cfg.Search.DefaultLimit < 1 {
		errors = append(errors, ValidationError{
			Field:   "search.default_limit",
			Message: "must be at least 1",
		})
	}
	for i, level := range cfg.Search.DefaultLevels {
		if !validChunkLevels[level] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("search.default_levels[%d]", i),
				Message: fmt.Sprintf("invalid chunk level '%s'; valid values are: file, class, method, block, line", level),
			})
		}
	}

	return errors
}

// ValidateOrError is a convenience function that returns an error if validation fails
func ValidateOrError(cfg *Config) error {
	errors := Validate(cfg)
	if errors.HasErrors() {
		return errors
	}
	return nil
}
