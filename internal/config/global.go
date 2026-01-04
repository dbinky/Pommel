package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// GlobalConfigDir returns the global Pommel configuration directory.
// On Unix: ~/.config/pommel (or XDG_CONFIG_HOME/pommel)
// On Windows: %APPDATA%\pommel
func GlobalConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "pommel")
	}

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "pommel")
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, ".config", "pommel")
}

// GlobalConfigPath returns the full path to the global config file.
func GlobalConfigPath() string {
	dir := GlobalConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

// GlobalConfigExists returns true if a global config file exists.
func GlobalConfigExists() bool {
	path := GlobalConfigPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// LoadGlobalConfig loads the global configuration from disk.
// Returns nil, nil if no global config exists (not an error).
// Returns nil, error if the config exists but cannot be read or parsed.
func LoadGlobalConfig() (*Config, error) {
	path := GlobalConfigPath()
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}

	// Empty file is treated as no config
	if len(data) == 0 {
		return nil, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global config: %w", err)
	}

	return &cfg, nil
}

// SaveGlobalConfig saves the configuration to the global config file.
// Creates the directory if it doesn't exist.
func SaveGlobalConfig(cfg *Config) error {
	dir := GlobalConfigDir()
	if dir == "" {
		return fmt.Errorf("cannot determine global config directory")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create global config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := GlobalConfigPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write global config: %w", err)
	}

	return nil
}

// MergeConfigs merges global and project configs, with project taking precedence.
// Returns a merged config where project values override global values.
// If both are nil, returns an empty Config (not nil).
func MergeConfigs(global, project *Config) *Config {
	result := &Config{}

	// Start with global config values
	if global != nil {
		*result = *global
	}

	// Override with project config values
	if project != nil {
		// Only override non-zero values from project
		if project.Version != 0 {
			result.Version = project.Version
		}
		if len(project.ChunkLevels) > 0 {
			result.ChunkLevels = project.ChunkLevels
		}
		if len(project.IncludePatterns) > 0 {
			result.IncludePatterns = project.IncludePatterns
		}
		if len(project.ExcludePatterns) > 0 {
			result.ExcludePatterns = project.ExcludePatterns
		}

		// Merge watcher settings
		if project.Watcher.DebounceMs != 0 {
			result.Watcher.DebounceMs = project.Watcher.DebounceMs
		}
		if project.Watcher.MaxFileSize != 0 {
			result.Watcher.MaxFileSize = project.Watcher.MaxFileSize
		}

		// Merge daemon settings
		if project.Daemon.Host != "" {
			result.Daemon.Host = project.Daemon.Host
		}
		if project.Daemon.Port != nil {
			result.Daemon.Port = project.Daemon.Port
		}
		if project.Daemon.LogLevel != "" {
			result.Daemon.LogLevel = project.Daemon.LogLevel
		}

		// Merge embedding settings
		if project.Embedding.Provider != "" {
			result.Embedding.Provider = project.Embedding.Provider
		}
		if project.Embedding.Model != "" {
			result.Embedding.Model = project.Embedding.Model
		}
		if project.Embedding.OllamaURL != "" {
			result.Embedding.OllamaURL = project.Embedding.OllamaURL
		}
		if project.Embedding.BatchSize != 0 {
			result.Embedding.BatchSize = project.Embedding.BatchSize
		}
		if project.Embedding.CacheSize != 0 {
			result.Embedding.CacheSize = project.Embedding.CacheSize
		}

		// Merge Ollama provider settings
		if project.Embedding.Ollama.URL != "" {
			result.Embedding.Ollama.URL = project.Embedding.Ollama.URL
		}
		if project.Embedding.Ollama.Model != "" {
			result.Embedding.Ollama.Model = project.Embedding.Ollama.Model
		}

		// Merge OpenAI provider settings
		if project.Embedding.OpenAI.APIKey != "" {
			result.Embedding.OpenAI.APIKey = project.Embedding.OpenAI.APIKey
		}
		if project.Embedding.OpenAI.Model != "" {
			result.Embedding.OpenAI.Model = project.Embedding.OpenAI.Model
		}

		// Merge Voyage provider settings
		if project.Embedding.Voyage.APIKey != "" {
			result.Embedding.Voyage.APIKey = project.Embedding.Voyage.APIKey
		}
		if project.Embedding.Voyage.Model != "" {
			result.Embedding.Voyage.Model = project.Embedding.Voyage.Model
		}

		// Merge search settings
		if project.Search.DefaultLimit != 0 {
			result.Search.DefaultLimit = project.Search.DefaultLimit
		}
		if len(project.Search.DefaultLevels) > 0 {
			result.Search.DefaultLevels = project.Search.DefaultLevels
		}
	}

	return result
}
