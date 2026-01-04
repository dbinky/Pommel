package cli

import (
	"errors"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
)

// === Happy Path Tests ===

func TestCheckProviderConfigured_Ollama(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "ollama",
			Ollama: config.OllamaProviderConfig{
				URL: "http://localhost:11434",
			},
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_OllamaDefaults(t *testing.T) {
	// Ollama works even without explicit URL (uses defaults)
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "ollama",
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_OllamaRemote(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "ollama-remote",
			Ollama: config.OllamaProviderConfig{
				URL: "http://192.168.1.100:11434",
			},
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_OpenAI(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "openai",
			OpenAI: config.OpenAIProviderConfig{
				APIKey: "sk-test",
			},
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_OpenAI_EnvKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "openai",
			OpenAI:   config.OpenAIProviderConfig{}, // Key from env
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_Voyage(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "voyage",
			Voyage: config.VoyageProviderConfig{
				APIKey: "pa-test",
			},
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

func TestCheckProviderConfigured_Voyage_EnvKey(t *testing.T) {
	t.Setenv("VOYAGE_API_KEY", "pa-from-env")

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "voyage",
			Voyage:   config.VoyageProviderConfig{}, // Key from env
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.NoError(t, err)
}

// === Failure Scenario Tests ===

func TestCheckProviderConfigured_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "", // Empty
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)

	var provErr *ProviderNotConfiguredError
	assert.True(t, errors.As(err, &provErr))
}

func TestCheckProviderConfigured_OpenAI_NoKey(t *testing.T) {
	// Clear env var if set
	t.Setenv("OPENAI_API_KEY", "")

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "openai",
			OpenAI:   config.OpenAIProviderConfig{}, // No key
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestCheckProviderConfigured_Voyage_NoKey(t *testing.T) {
	t.Setenv("VOYAGE_API_KEY", "")

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "voyage",
			Voyage:   config.VoyageProviderConfig{}, // No key
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestCheckProviderConfigured_OllamaRemote_NoURL(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "ollama-remote",
			Ollama:   config.OllamaProviderConfig{}, // No URL
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL")
}

func TestCheckProviderConfigured_OllamaRemote_LocalhostURL(t *testing.T) {
	// ollama-remote with localhost is invalid (should use "ollama" provider instead)
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "ollama-remote",
			Ollama: config.OllamaProviderConfig{
				URL: "http://localhost:11434",
			},
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "URL")
}

func TestCheckProviderConfigured_UnknownProvider(t *testing.T) {
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "unknown-provider",
		},
	}
	err := CheckProviderConfigured(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

// === Edge Case Tests ===

func TestCheckProviderConfigured_NilConfig(t *testing.T) {
	err := CheckProviderConfigured(nil)
	assert.Error(t, err)

	var provErr *ProviderNotConfiguredError
	assert.True(t, errors.As(err, &provErr))
}

func TestProviderNotConfiguredError_Message(t *testing.T) {
	err := &ProviderNotConfiguredError{}
	msg := err.Error()
	assert.Contains(t, msg, "No embedding provider configured")
	assert.Contains(t, msg, "pm config provider")
}
