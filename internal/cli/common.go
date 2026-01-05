package cli

import (
	"fmt"
	"os"

	"github.com/pommel-dev/pommel/internal/config"
)

// ProviderNotConfiguredError indicates no embedding provider is configured.
type ProviderNotConfiguredError struct{}

func (e *ProviderNotConfiguredError) Error() string {
	return `No embedding provider configured

Run 'pm config provider' to set up embeddings.`
}

// CheckProviderConfigured validates that an embedding provider is properly configured.
// Returns nil if a valid provider is configured, or an error describing what's missing.
func CheckProviderConfigured(cfg *config.Config) error {
	if cfg == nil {
		return &ProviderNotConfiguredError{}
	}

	if cfg.Embedding.Provider == "" {
		return &ProviderNotConfiguredError{}
	}

	// Check provider-specific requirements
	switch cfg.Embedding.Provider {
	case "openai":
		if cfg.Embedding.GetOpenAIAPIKey() == "" {
			return fmt.Errorf("openai provider requires API key: set embedding.openai.api_key in config or OPENAI_API_KEY environment variable")
		}
	case "voyage":
		if cfg.Embedding.GetVoyageAPIKey() == "" {
			return fmt.Errorf("voyage provider requires API key: set embedding.voyage.api_key in config or VOYAGE_API_KEY environment variable")
		}
	case "ollama-remote":
		url := cfg.Embedding.GetOllamaURL()
		if url == "" || url == "http://localhost:11434" {
			return fmt.Errorf("ollama-remote provider requires URL: set embedding.ollama.url in config")
		}
	case "ollama":
		// Local ollama can use defaults, no required fields
	default:
		return fmt.Errorf("unknown provider '%s'; valid providers are: ollama, ollama-remote, openai, voyage", cfg.Embedding.Provider)
	}

	return nil
}

// LoadMergedConfig loads and merges global and project configurations.
// Returns the merged config with project values taking precedence over global values.
func LoadMergedConfig(projectRoot string) (*config.Config, error) {
	// Load global config (may be nil if not exists)
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Load project config
	loader := config.NewLoader(projectRoot)
	projectCfg, err := loader.Load()
	if err != nil {
		// If project config doesn't exist but global does, use global
		if os.IsNotExist(err) && globalCfg != nil {
			return globalCfg, nil
		}
		return nil, err
	}

	// Merge configs (project takes precedence)
	merged := config.MergeConfigs(globalCfg, projectCfg)

	// Apply legacy migration if needed
	merged = config.MigrateLegacyConfig(merged)

	return merged, nil
}
