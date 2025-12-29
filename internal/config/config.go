package config

import (
	"fmt"
	"time"
)

// Config represents the complete Pommel configuration
type Config struct {
	Version         int             `yaml:"version" json:"version" mapstructure:"version"`
	ChunkLevels     []string        `yaml:"chunk_levels" json:"chunk_levels" mapstructure:"chunk_levels"`
	IncludePatterns []string        `yaml:"include_patterns" json:"include_patterns" mapstructure:"include_patterns"`
	ExcludePatterns []string        `yaml:"exclude_patterns" json:"exclude_patterns" mapstructure:"exclude_patterns"`
	Watcher         WatcherConfig   `yaml:"watcher" json:"watcher" mapstructure:"watcher"`
	Daemon          DaemonConfig    `yaml:"daemon" json:"daemon" mapstructure:"daemon"`
	Embedding       EmbeddingConfig `yaml:"embedding" json:"embedding" mapstructure:"embedding"`
	Search          SearchConfig    `yaml:"search" json:"search" mapstructure:"search"`
}

// WatcherConfig contains file watcher settings
type WatcherConfig struct {
	DebounceMs  int   `yaml:"debounce_ms" json:"debounce_ms" mapstructure:"debounce_ms"`
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size" mapstructure:"max_file_size"`
}

// DaemonConfig contains daemon server settings
type DaemonConfig struct {
	Host     string `yaml:"host" json:"host" mapstructure:"host"`
	Port     int    `yaml:"port" json:"port" mapstructure:"port"`
	LogLevel string `yaml:"log_level" json:"log_level" mapstructure:"log_level"`
}

// EmbeddingConfig contains embedding model settings
type EmbeddingConfig struct {
	Model     string `yaml:"model" json:"model" mapstructure:"model"`
	OllamaURL string `yaml:"ollama_url" json:"ollama_url" mapstructure:"ollama_url"`
	BatchSize int    `yaml:"batch_size" json:"batch_size" mapstructure:"batch_size"`
	CacheSize int    `yaml:"cache_size" json:"cache_size" mapstructure:"cache_size"`
}

// SearchConfig contains search default settings
type SearchConfig struct {
	DefaultLimit  int      `yaml:"default_limit" json:"default_limit" mapstructure:"default_limit"`
	DefaultLevels []string `yaml:"default_levels" json:"default_levels" mapstructure:"default_levels"`
}

// DebounceDuration returns the debounce duration as time.Duration
func (w WatcherConfig) DebounceDuration() time.Duration {
	return time.Duration(w.DebounceMs) * time.Millisecond
}

// Address returns the full host:port address for the daemon
func (d DaemonConfig) Address() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}
