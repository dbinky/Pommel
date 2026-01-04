package embedder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === Provider Type Tests ===

func TestProviderType_String(t *testing.T) {
	tests := []struct {
		provider ProviderType
		expected string
	}{
		{ProviderOllama, "ollama"},
		{ProviderOllamaRemote, "ollama-remote"},
		{ProviderOpenAI, "openai"},
		{ProviderVoyage, "voyage"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.provider))
		})
	}
}

func TestProviderType_IsValid(t *testing.T) {
	t.Run("valid providers", func(t *testing.T) {
		assert.True(t, ProviderOllama.IsValid())
		assert.True(t, ProviderOllamaRemote.IsValid())
		assert.True(t, ProviderOpenAI.IsValid())
		assert.True(t, ProviderVoyage.IsValid())
	})

	t.Run("invalid provider", func(t *testing.T) {
		assert.False(t, ProviderType("invalid").IsValid())
	})

	t.Run("empty provider", func(t *testing.T) {
		assert.False(t, ProviderType("").IsValid())
	})
}

func TestProviderType_DisplayName(t *testing.T) {
	tests := []struct {
		provider ProviderType
		expected string
	}{
		{ProviderOllama, "Ollama (local)"},
		{ProviderOllamaRemote, "Ollama (remote)"},
		{ProviderOpenAI, "OpenAI"},
		{ProviderVoyage, "Voyage AI"},
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.DisplayName())
		})
	}

	t.Run("unknown provider", func(t *testing.T) {
		assert.Equal(t, "Unknown", ProviderType("unknown").DisplayName())
	})
}

func TestProviderType_RequiresAPIKey(t *testing.T) {
	t.Run("ollama does not require API key", func(t *testing.T) {
		assert.False(t, ProviderOllama.RequiresAPIKey())
		assert.False(t, ProviderOllamaRemote.RequiresAPIKey())
	})

	t.Run("API providers require key", func(t *testing.T) {
		assert.True(t, ProviderOpenAI.RequiresAPIKey())
		assert.True(t, ProviderVoyage.RequiresAPIKey())
	})
}

func TestProviderType_DefaultDimensions(t *testing.T) {
	tests := []struct {
		provider   ProviderType
		dimensions int
	}{
		{ProviderOllama, 768},       // Jina Code
		{ProviderOllamaRemote, 768}, // Jina Code
		{ProviderOpenAI, 1536},      // text-embedding-3-small
		{ProviderVoyage, 1024},      // voyage-code-3
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			assert.Equal(t, tt.dimensions, tt.provider.DefaultDimensions())
		})
	}
}

func TestAllProviders(t *testing.T) {
	providers := AllProviders()
	assert.Len(t, providers, 4)
	assert.Contains(t, providers, ProviderOllama)
	assert.Contains(t, providers, ProviderOllamaRemote)
	assert.Contains(t, providers, ProviderOpenAI)
	assert.Contains(t, providers, ProviderVoyage)
}

func TestAPIProviders(t *testing.T) {
	providers := APIProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, ProviderOpenAI)
	assert.Contains(t, providers, ProviderVoyage)
}

// === NewFromConfig Tests ===

func TestNewFromConfig_OllamaLocal(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "ollama",
		Ollama: OllamaProviderSettings{
			URL:   "http://localhost:11434",
			Model: "jina-code",
		},
	}

	embedder, err := NewFromConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, embedder)
	assert.IsType(t, &OllamaClient{}, embedder)
	assert.Equal(t, "jina-code", embedder.ModelName())
}

func TestNewFromConfig_OllamaRemote(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "ollama-remote",
		Ollama: OllamaProviderSettings{
			URL:   "http://remote:11434",
			Model: "jina-code",
		},
	}

	embedder, err := NewFromConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, embedder)
	assert.IsType(t, &OllamaClient{}, embedder)
}

func TestNewFromConfig_OpenAI(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "openai",
		OpenAI: OpenAIProviderSettings{
			APIKey: "sk-test",
			Model:  "text-embedding-3-small",
		},
	}

	embedder, err := NewFromConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, embedder)
	assert.IsType(t, &OpenAIClient{}, embedder)
	assert.Equal(t, "text-embedding-3-small", embedder.ModelName())
}

func TestNewFromConfig_Voyage(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "voyage",
		Voyage: VoyageProviderSettings{
			APIKey: "pa-test",
			Model:  "voyage-code-3",
		},
	}

	embedder, err := NewFromConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, embedder)
	assert.IsType(t, &VoyageClient{}, embedder)
	assert.Equal(t, "voyage-code-3", embedder.ModelName())
}

func TestNewFromConfig_UnknownProvider(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "unknown",
	}

	_, err := NewFromConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestNewFromConfig_EmptyProvider(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "",
	}

	_, err := NewFromConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestNewFromConfig_OpenAI_NoAPIKey(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "openai",
		OpenAI: OpenAIProviderSettings{
			Model: "text-embedding-3-small",
		},
	}

	_, err := NewFromConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestNewFromConfig_Voyage_NoAPIKey(t *testing.T) {
	cfg := &ProviderConfig{
		Provider: "voyage",
		Voyage: VoyageProviderSettings{
			Model: "voyage-code-3",
		},
	}

	_, err := NewFromConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}
