package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateLegacyConfig_OllamaURL(t *testing.T) {
	oldCfg := &Config{
		Embedding: EmbeddingConfig{
			Model:     "unclemusclez/jina-embeddings-v2-base-code",
			OllamaURL: "http://localhost:11434", // Legacy field
		},
	}

	migrated := MigrateLegacyConfig(oldCfg)

	assert.Equal(t, "ollama", migrated.Embedding.Provider)
	assert.Equal(t, "http://localhost:11434", migrated.Embedding.Ollama.URL)
	assert.Equal(t, "unclemusclez/jina-embeddings-v2-base-code", migrated.Embedding.Ollama.Model)
}

func TestMigrateLegacyConfig_RemoteOllama(t *testing.T) {
	oldCfg := &Config{
		Embedding: EmbeddingConfig{
			OllamaURL: "http://192.168.1.100:11434",
			Model:     "jina-code",
		},
	}

	migrated := MigrateLegacyConfig(oldCfg)

	assert.Equal(t, "ollama-remote", migrated.Embedding.Provider)
	assert.Equal(t, "http://192.168.1.100:11434", migrated.Embedding.Ollama.URL)
}

func TestMigrateLegacyConfig_AlreadyMigrated(t *testing.T) {
	cfg := &Config{
		Embedding: EmbeddingConfig{
			Provider:  "openai",
			OllamaURL: "http://localhost:11434", // Leftover, ignored
		},
	}

	migrated := MigrateLegacyConfig(cfg)

	assert.Equal(t, "openai", migrated.Embedding.Provider)
}

func TestMigrateLegacyConfig_NoLegacyFields(t *testing.T) {
	cfg := &Config{
		Embedding: EmbeddingConfig{
			Provider: "voyage",
			Voyage: VoyageProviderConfig{
				APIKey: "pa-test",
			},
		},
	}

	migrated := MigrateLegacyConfig(cfg)
	assert.Equal(t, "voyage", migrated.Embedding.Provider)
}

func TestMigrateLegacyConfig_NilConfig(t *testing.T) {
	assert.Nil(t, MigrateLegacyConfig(nil))
}

func TestMigrateLegacyConfig_PreservesOtherSettings(t *testing.T) {
	oldCfg := &Config{
		Version:     1,
		ChunkLevels: []string{"method", "class"},
		Embedding: EmbeddingConfig{
			Model:     "jina-code",
			OllamaURL: "http://localhost:11434",
			BatchSize: 32,
		},
		Search: SearchConfig{
			DefaultLimit: 15,
		},
	}

	migrated := MigrateLegacyConfig(oldCfg)

	assert.Equal(t, 1, migrated.Version)
	assert.Equal(t, []string{"method", "class"}, migrated.ChunkLevels)
	assert.Equal(t, 32, migrated.Embedding.BatchSize)
	assert.Equal(t, 15, migrated.Search.DefaultLimit)
}

func TestMigrateLegacyConfig_HostnameVariants(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		expectedProvider string
	}{
		{"localhost", "http://localhost:11434", "ollama"},
		{"127.0.0.1", "http://127.0.0.1:11434", "ollama"},
		{"0.0.0.0", "http://0.0.0.0:11434", "ollama"},
		{"remote IP", "http://10.0.0.1:11434", "ollama-remote"},
		{"remote hostname", "http://ollama.example.com:11434", "ollama-remote"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Embedding: EmbeddingConfig{
					OllamaURL: tt.url,
					Model:     "test",
				},
			}
			migrated := MigrateLegacyConfig(cfg)
			assert.Equal(t, tt.expectedProvider, migrated.Embedding.Provider)
		})
	}
}
