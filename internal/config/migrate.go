package config

import (
	"net/url"
	"strings"
)

// MigrateLegacyConfig converts legacy config format to the new provider-based format.
// If the config already has a provider set, it returns the config unchanged.
// Returns nil if the input is nil.
func MigrateLegacyConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	// If provider is already set, no migration needed
	if cfg.Embedding.Provider != "" {
		return cfg
	}

	// Check if we have legacy OllamaURL or Model fields
	if cfg.Embedding.OllamaURL == "" && cfg.Embedding.Model == "" {
		return cfg
	}

	// Determine if this is local or remote Ollama
	provider := determineOllamaProvider(cfg.Embedding.OllamaURL)

	// Set the provider
	cfg.Embedding.Provider = provider

	// Migrate settings to new structure
	if cfg.Embedding.OllamaURL != "" {
		cfg.Embedding.Ollama.URL = cfg.Embedding.OllamaURL
	}
	if cfg.Embedding.Model != "" {
		cfg.Embedding.Ollama.Model = cfg.Embedding.Model
	}

	return cfg
}

// determineOllamaProvider determines whether the Ollama URL is local or remote.
func determineOllamaProvider(ollamaURL string) string {
	if ollamaURL == "" {
		return "ollama"
	}

	parsed, err := url.Parse(ollamaURL)
	if err != nil {
		// If we can't parse, assume local
		return "ollama"
	}

	host := parsed.Hostname()

	// Check for localhost variants
	if isLocalhost(host) {
		return "ollama"
	}

	return "ollama-remote"
}

// isLocalhost checks if a hostname refers to the local machine.
func isLocalhost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "0.0.0.0" ||
		host == "::1" ||
		host == ""
}

// NeedsMigration checks if a config uses legacy fields that need migration.
func NeedsMigration(cfg *Config) bool {
	if cfg == nil {
		return false
	}

	// If provider is not set but legacy fields are present
	if cfg.Embedding.Provider == "" {
		return cfg.Embedding.OllamaURL != "" || cfg.Embedding.Model != ""
	}

	return false
}
