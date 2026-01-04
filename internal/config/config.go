package config

import (
	"fmt"
	"os"
	"time"
)

// Config represents the complete Pommel configuration
type Config struct {
	Version         int               `yaml:"version" json:"version" mapstructure:"version"`
	ChunkLevels     []string          `yaml:"chunk_levels" json:"chunk_levels" mapstructure:"chunk_levels"`
	IncludePatterns []string          `yaml:"include_patterns" json:"include_patterns" mapstructure:"include_patterns"`
	ExcludePatterns []string          `yaml:"exclude_patterns" json:"exclude_patterns" mapstructure:"exclude_patterns"`
	Watcher         WatcherConfig     `yaml:"watcher" json:"watcher" mapstructure:"watcher"`
	Daemon          DaemonConfig      `yaml:"daemon" json:"daemon" mapstructure:"daemon"`
	Embedding       EmbeddingConfig   `yaml:"embedding" json:"embedding" mapstructure:"embedding"`
	Search          SearchConfig      `yaml:"search" json:"search" mapstructure:"search"`
	Subprojects     SubprojectsConfig `yaml:"subprojects" json:"subprojects" mapstructure:"subprojects"`
}

// WatcherConfig contains file watcher settings
type WatcherConfig struct {
	DebounceMs  int   `yaml:"debounce_ms" json:"debounce_ms" mapstructure:"debounce_ms"`
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size" mapstructure:"max_file_size"`
}

// DaemonConfig contains daemon server settings
type DaemonConfig struct {
	Host     string `yaml:"host" json:"host" mapstructure:"host"`
	Port     *int   `yaml:"port" json:"port,omitempty" mapstructure:"port"` // nil = use hash-based port
	LogLevel string `yaml:"log_level" json:"log_level" mapstructure:"log_level"`
}

// EmbeddingConfig contains embedding model settings
type EmbeddingConfig struct {
	Provider  string `yaml:"provider" json:"provider" mapstructure:"provider"`
	Model     string `yaml:"model" json:"model" mapstructure:"model"`                   // Legacy: used with Ollama
	OllamaURL string `yaml:"ollama_url" json:"ollama_url" mapstructure:"ollama_url"`    // Legacy: use Ollama.URL instead
	BatchSize int    `yaml:"batch_size" json:"batch_size" mapstructure:"batch_size"`
	CacheSize int    `yaml:"cache_size" json:"cache_size" mapstructure:"cache_size"`

	// Provider-specific configurations
	Ollama OllamaProviderConfig `yaml:"ollama" json:"ollama" mapstructure:"ollama"`
	OpenAI OpenAIProviderConfig `yaml:"openai" json:"openai" mapstructure:"openai"`
	Voyage VoyageProviderConfig `yaml:"voyage" json:"voyage" mapstructure:"voyage"`
}

// OllamaProviderConfig contains Ollama-specific settings
type OllamaProviderConfig struct {
	URL   string `yaml:"url" json:"url" mapstructure:"url"`
	Model string `yaml:"model" json:"model" mapstructure:"model"`
}

// OpenAIProviderConfig contains OpenAI-specific settings
type OpenAIProviderConfig struct {
	APIKey string `yaml:"api_key" json:"api_key" mapstructure:"api_key"`
	Model  string `yaml:"model" json:"model" mapstructure:"model"`
}

// VoyageProviderConfig contains Voyage AI-specific settings
type VoyageProviderConfig struct {
	APIKey string `yaml:"api_key" json:"api_key" mapstructure:"api_key"`
	Model  string `yaml:"model" json:"model" mapstructure:"model"`
}

// GetOllamaURL returns the Ollama URL from config or environment variable.
// Supports legacy OllamaURL field for backwards compatibility.
func (e *EmbeddingConfig) GetOllamaURL() string {
	// First check new config structure
	if e.Ollama.URL != "" {
		return e.Ollama.URL
	}
	// Check legacy field
	if e.OllamaURL != "" {
		return e.OllamaURL
	}
	// Check environment variable
	if url := os.Getenv("OLLAMA_HOST"); url != "" {
		return url
	}
	return "http://localhost:11434"
}

// GetOpenAIAPIKey returns the OpenAI API key from config or environment variable.
func (e *EmbeddingConfig) GetOpenAIAPIKey() string {
	if e.OpenAI.APIKey != "" {
		return e.OpenAI.APIKey
	}
	return os.Getenv("OPENAI_API_KEY")
}

// GetVoyageAPIKey returns the Voyage API key from config or environment variable.
func (e *EmbeddingConfig) GetVoyageAPIKey() string {
	if e.Voyage.APIKey != "" {
		return e.Voyage.APIKey
	}
	return os.Getenv("VOYAGE_API_KEY")
}

// DefaultDimensions returns the default embedding dimensions for the configured provider.
func (e *EmbeddingConfig) DefaultDimensions() int {
	switch e.Provider {
	case "openai":
		return 1536 // text-embedding-3-small
	case "voyage":
		return 1024 // voyage-code-3
	default:
		return 768 // Jina Code embeddings via Ollama
	}
}

// SearchConfig contains search default settings
type SearchConfig struct {
	DefaultLimit  int                `yaml:"default_limit" json:"default_limit" mapstructure:"default_limit"`
	DefaultLevels []string           `yaml:"default_levels" json:"default_levels" mapstructure:"default_levels"`
	Hybrid        HybridSearchConfig `yaml:"hybrid" json:"hybrid" mapstructure:"hybrid"`
	Reranker      RerankerConfig     `yaml:"reranker" json:"reranker" mapstructure:"reranker"`
}

// HybridSearchConfig contains hybrid search settings
type HybridSearchConfig struct {
	Enabled       bool    `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	RRFK          int     `yaml:"rrf_k" json:"rrf_k" mapstructure:"rrf_k"`
	VectorWeight  float64 `yaml:"vector_weight" json:"vector_weight" mapstructure:"vector_weight"`
	KeywordWeight float64 `yaml:"keyword_weight" json:"keyword_weight" mapstructure:"keyword_weight"`
}

// DefaultHybridSearchConfig returns the default hybrid search configuration
func DefaultHybridSearchConfig() HybridSearchConfig {
	return HybridSearchConfig{
		Enabled:       true,
		RRFK:          60,
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
	}
}

// RerankerConfig contains re-ranker settings
type RerankerConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	Model      string `yaml:"model" json:"model,omitempty" mapstructure:"model"`
	TimeoutMs  int    `yaml:"timeout_ms" json:"timeout_ms" mapstructure:"timeout_ms"`
	Fallback   string `yaml:"fallback" json:"fallback" mapstructure:"fallback"`
	Candidates int    `yaml:"candidates" json:"candidates" mapstructure:"candidates"`
}

// DefaultRerankerConfig returns the default re-ranker configuration
func DefaultRerankerConfig() RerankerConfig {
	return RerankerConfig{
		Enabled:    true,
		Model:      "",   // Empty = use heuristic only
		TimeoutMs:  2000, // 2 seconds
		Fallback:   "heuristic",
		Candidates: 20,
	}
}

// SubprojectsConfig contains sub-project detection settings
type SubprojectsConfig struct {
	AutoDetect bool              `yaml:"auto_detect" json:"auto_detect" mapstructure:"auto_detect"`
	Markers    []string          `yaml:"markers" json:"markers" mapstructure:"markers"`
	Projects   []ProjectOverride `yaml:"projects" json:"projects,omitempty" mapstructure:"projects"`
	Exclude    []string          `yaml:"exclude" json:"exclude,omitempty" mapstructure:"exclude"`
}

// ProjectOverride defines a manual sub-project configuration
type ProjectOverride struct {
	ID   string `yaml:"id" json:"id,omitempty" mapstructure:"id"`
	Path string `yaml:"path" json:"path" mapstructure:"path"`
	Name string `yaml:"name" json:"name,omitempty" mapstructure:"name"`
}

// DebounceDuration returns the debounce duration as time.Duration
func (w WatcherConfig) DebounceDuration() time.Duration {
	return time.Duration(w.DebounceMs) * time.Millisecond
}

// Address returns the full host:port address for the daemon.
// If Port is nil, returns just the host (port will be determined elsewhere).
// If Port is 0, returns host:0 (system-assigned port).
func (d DaemonConfig) Address() string {
	if d.Port == nil {
		return d.Host
	}
	return fmt.Sprintf("%s:%d", d.Host, *d.Port)
}

// AddressWithPort returns the full host:port address using the provided port.
func (d DaemonConfig) AddressWithPort(port int) string {
	return fmt.Sprintf("%s:%d", d.Host, port)
}
