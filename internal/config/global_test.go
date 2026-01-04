package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === Global Config Path Tests ===

func TestGlobalConfigPath_Default(t *testing.T) {
	// Clear env vars to test default behavior
	t.Setenv("XDG_CONFIG_HOME", "")

	path := GlobalConfigPath()
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			assert.Equal(t, filepath.Join(appData, "pommel", "config.yaml"), path)
		}
	} else {
		expected := filepath.Join(homeDir, ".config", "pommel", "config.yaml")
		assert.Equal(t, expected, path)
	}
}

func TestGlobalConfigPath_XDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	path := GlobalConfigPath()
	assert.Equal(t, "/custom/config/pommel/config.yaml", path)
}

func TestGlobalConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := GlobalConfigDir()
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			assert.Equal(t, filepath.Join(appData, "pommel"), dir)
		}
	} else {
		expected := filepath.Join(homeDir, ".config", "pommel")
		assert.Equal(t, expected, dir)
	}
}

func TestGlobalConfigPath_WindowsStyle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	path := GlobalConfigPath()
	assert.Contains(t, path, "pommel")
	assert.Contains(t, path, "config.yaml")
}

// === Global Config Loading Tests ===

func TestLoadGlobalConfig_Success(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configContent := `
embedding:
  provider: openai
  openai:
    api_key: sk-test
    model: text-embedding-3-small
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644))

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "sk-test", cfg.Embedding.OpenAI.APIKey)
}

func TestLoadGlobalConfig_NotExists(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadGlobalConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("invalid: yaml: : content:"), 0644))

	_, err := LoadGlobalConfig()
	assert.Error(t, err)
}

func TestLoadGlobalConfig_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test unreliable on Windows")
	}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("embedding:\n  provider: openai"), 0644))
	require.NoError(t, os.Chmod(configPath, 0000))
	defer os.Chmod(configPath, 0644)

	_, err := LoadGlobalConfig()
	assert.Error(t, err)
}

func TestLoadGlobalConfig_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(""), 0644))

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

// === Save Global Config Tests ===

func TestSaveGlobalConfig_Success(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg := &Config{
		Embedding: EmbeddingConfig{
			Provider: "openai",
			OpenAI: OpenAIProviderConfig{
				APIKey: "sk-test",
			},
		},
	}

	err := SaveGlobalConfig(cfg)
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "pommel", "config.yaml")
	assert.FileExists(t, configPath)

	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "openai", loaded.Embedding.Provider)
}

func TestSaveGlobalConfig_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	assert.NoDirExists(t, configDir)

	cfg := &Config{Embedding: EmbeddingConfig{Provider: "openai"}}
	err := SaveGlobalConfig(cfg)

	require.NoError(t, err)
	assert.DirExists(t, configDir)
}

func TestSaveGlobalConfig_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test unreliable on Windows")
	}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0555))
	defer os.Chmod(configDir, 0755)

	cfg := &Config{Embedding: EmbeddingConfig{Provider: "openai"}}
	err := SaveGlobalConfig(cfg)

	assert.Error(t, err)
}

// === Global Config Exists Tests ===

func TestGlobalConfigExists_True(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir := filepath.Join(tempDir, "pommel")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("test: value"), 0644))

	assert.True(t, GlobalConfigExists())
}

func TestGlobalConfigExists_False(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	assert.False(t, GlobalConfigExists())
}

// === Config Merge Tests ===

func TestMergeConfigs_ProjectOverridesGlobal(t *testing.T) {
	global := &Config{
		Embedding: EmbeddingConfig{
			Provider: "openai",
			OpenAI: OpenAIProviderConfig{
				APIKey: "sk-global",
				Model:  "text-embedding-3-small",
			},
		},
	}

	project := &Config{
		Embedding: EmbeddingConfig{
			Provider: "voyage",
			Voyage: VoyageProviderConfig{
				APIKey: "pa-project",
			},
		},
	}

	merged := MergeConfigs(global, project)
	assert.Equal(t, "voyage", merged.Embedding.Provider)
	assert.Equal(t, "pa-project", merged.Embedding.Voyage.APIKey)
}

func TestMergeConfigs_GlobalOnly(t *testing.T) {
	global := &Config{
		Embedding: EmbeddingConfig{
			Provider: "openai",
			OpenAI: OpenAIProviderConfig{
				APIKey: "sk-global",
			},
		},
	}

	merged := MergeConfigs(global, nil)
	assert.Equal(t, "openai", merged.Embedding.Provider)
	assert.Equal(t, "sk-global", merged.Embedding.OpenAI.APIKey)
}

func TestMergeConfigs_ProjectOnly(t *testing.T) {
	project := &Config{
		Embedding: EmbeddingConfig{
			Provider: "ollama",
			Ollama: OllamaProviderConfig{
				URL: "http://localhost:11434",
			},
		},
	}

	merged := MergeConfigs(nil, project)
	assert.Equal(t, "ollama", merged.Embedding.Provider)
}

func TestMergeConfigs_NeitherExists(t *testing.T) {
	merged := MergeConfigs(nil, nil)
	assert.NotNil(t, merged)
}

func TestMergeConfigs_GlobalAPIKeyWithProjectProvider(t *testing.T) {
	// Global has API keys, project just specifies provider
	global := &Config{
		Embedding: EmbeddingConfig{
			OpenAI: OpenAIProviderConfig{
				APIKey: "sk-global",
				Model:  "text-embedding-3-small",
			},
		},
	}

	project := &Config{
		Embedding: EmbeddingConfig{
			Provider: "openai", // Just want to use OpenAI
		},
	}

	merged := MergeConfigs(global, project)
	assert.Equal(t, "openai", merged.Embedding.Provider)
	assert.Equal(t, "sk-global", merged.Embedding.OpenAI.APIKey)
	assert.Equal(t, "text-embedding-3-small", merged.Embedding.OpenAI.Model)
}

func TestMergeConfigs_ProjectOverridesSearchLimit(t *testing.T) {
	global := &Config{
		Embedding: EmbeddingConfig{Provider: "openai"},
		Search:    SearchConfig{DefaultLimit: 10},
	}

	project := &Config{
		Search: SearchConfig{DefaultLimit: 20},
	}

	merged := MergeConfigs(global, project)
	assert.Equal(t, "openai", merged.Embedding.Provider) // From global
	assert.Equal(t, 20, merged.Search.DefaultLimit)      // From project
}
