package config

import (
	"fmt"
	"time"
)

// Config represents the complete Pommel configuration
type Config struct {
	Version         int             `yaml:"version" mapstructure:"version"`
	ChunkLevels     []string        `yaml:"chunk_levels" mapstructure:"chunk_levels"`
	IncludePatterns []string        `yaml:"include_patterns" mapstructure:"include_patterns"`
	ExcludePatterns []string        `yaml:"exclude_patterns" mapstructure:"exclude_patterns"`
	Watcher         WatcherConfig   `yaml:"watcher" mapstructure:"watcher"`
	Daemon          DaemonConfig    `yaml:"daemon" mapstructure:"daemon"`
	Embedding       EmbeddingConfig `yaml:"embedding" mapstructure:"embedding"`
	Search          SearchConfig    `yaml:"search" mapstructure:"search"`
}

// WatcherConfig contains file watcher settings
type WatcherConfig struct {
	DebounceMs  int   `yaml:"debounce_ms" mapstructure:"debounce_ms"`
	MaxFileSize int64 `yaml:"max_file_size" mapstructure:"max_file_size"`
}

// DaemonConfig contains daemon server settings
type DaemonConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`
	Port     int    `yaml:"port" mapstructure:"port"`
	LogLevel string `yaml:"log_level" mapstructure:"log_level"`
}

// EmbeddingConfig contains embedding model settings
type EmbeddingConfig struct {
	Model     string `yaml:"model" mapstructure:"model"`
	BatchSize int    `yaml:"batch_size" mapstructure:"batch_size"`
	CacheSize int    `yaml:"cache_size" mapstructure:"cache_size"`
}

// SearchConfig contains search default settings
type SearchConfig struct {
	DefaultLimit  int      `yaml:"default_limit" mapstructure:"default_limit"`
	DefaultLevels []string `yaml:"default_levels" mapstructure:"default_levels"`
}

// DebounceDuration returns the debounce duration as time.Duration
func (w WatcherConfig) DebounceDuration() time.Duration {
	return time.Duration(w.DebounceMs) * time.Millisecond
}

// Address returns the full host:port address for the daemon
func (d DaemonConfig) Address() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}
