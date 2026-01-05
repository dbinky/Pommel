package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPIValidator is a mock implementation for testing
type mockAPIValidator struct {
	valid bool
}

func (m *mockAPIValidator) ValidateOpenAI(apiKey string) bool {
	return m.valid
}

func (m *mockAPIValidator) ValidateVoyage(apiKey string) bool {
	return m.valid
}

// === Interactive Mode Tests ===

func TestConfigProviderCmd_Interactive_Ollama(t *testing.T) {
	// Simulate user selecting option 1 (Local Ollama)
	input := strings.NewReader("1\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cmd := NewConfigProviderCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify config was saved
	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "ollama", cfg.Embedding.Provider)

	// Verify output
	assert.Contains(t, output.String(), "Local Ollama")
	assert.Contains(t, output.String(), "Ready!")
}

func TestConfigProviderCmd_Interactive_OpenAI(t *testing.T) {
	// Simulate: select OpenAI (3), enter API key
	input := strings.NewReader("3\nsk-test-key-12345\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Mock API validation to succeed
	mockValidator := &mockAPIValidator{valid: true}

	cmd := NewConfigProviderCmdWithValidator(mockValidator)
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "sk-test-key-12345", cfg.Embedding.OpenAI.APIKey)

	assert.Contains(t, output.String(), "validated")
}

func TestConfigProviderCmd_Interactive_OpenAI_SkipKey(t *testing.T) {
	// Simulate: select OpenAI, leave key blank
	input := strings.NewReader("3\n\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cmd := NewConfigProviderCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Empty(t, cfg.Embedding.OpenAI.APIKey)

	assert.Contains(t, output.String(), "configure later")
}

func TestConfigProviderCmd_Interactive_RemoteOllama(t *testing.T) {
	// Simulate: select Remote Ollama (2), enter URL
	input := strings.NewReader("2\nhttp://192.168.1.100:11434\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cmd := NewConfigProviderCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
	assert.Equal(t, "http://192.168.1.100:11434", cfg.Embedding.Ollama.URL)
}

func TestConfigProviderCmd_Interactive_InvalidAPIKey(t *testing.T) {
	input := strings.NewReader("3\ninvalid-key\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	mockValidator := &mockAPIValidator{valid: false}

	cmd := NewConfigProviderCmdWithValidator(mockValidator)
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err) // Command succeeds but warns

	assert.Contains(t, output.String(), "Invalid API key")
	assert.Contains(t, output.String(), "configure later")
}

func TestConfigProviderCmd_Interactive_InvalidChoice(t *testing.T) {
	// Invalid choice, then valid choice
	input := strings.NewReader("99\n1\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cmd := NewConfigProviderCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, output.String(), "Invalid choice")
}

func TestConfigProviderCmd_ShowsCurrent(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Pre-create config
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			Provider: "openai",
		},
	}
	err := config.SaveGlobalConfig(cfg)
	require.NoError(t, err)

	input := strings.NewReader("1\n") // Switch to Ollama
	output := &bytes.Buffer{}

	cmd := NewConfigProviderCmd()
	cmd.SetIn(input)
	cmd.SetOut(output)
	err = cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, output.String(), "Current provider: openai")
}

func TestConfigProviderCmd_ProviderChangeWarning(t *testing.T) {
	// When changing providers, warn about reindexing
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{Provider: "ollama"},
	}
	err := config.SaveGlobalConfig(cfg)
	require.NoError(t, err)

	input := strings.NewReader("3\nsk-test\n") // Switch to OpenAI
	output := &bytes.Buffer{}

	mockValidator := &mockAPIValidator{valid: true}
	cmd := NewConfigProviderCmdWithValidator(mockValidator)
	cmd.SetIn(input)
	cmd.SetOut(output)
	err = cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, output.String(), "reindex")
}

// === Direct Mode Tests ===

func TestConfigProviderCmd_Direct_Ollama(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"ollama"})

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "ollama", cfg.Embedding.Provider)
}

func TestConfigProviderCmd_Direct_OpenAI_WithKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("OPENAI_API_KEY", "") // Clear env

	output := &bytes.Buffer{}

	// Reset and set flags
	configProviderAPIKey = "sk-test"
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"openai", "--api-key", "sk-test"})
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "sk-test", cfg.Embedding.OpenAI.APIKey)
}

func TestConfigProviderCmd_Direct_OllamaRemote_WithURL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Reset and set flags
	configProviderAPIKey = ""
	configProviderURL = "http://192.168.1.100:11434"

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"ollama-remote", "--url", "http://192.168.1.100:11434"})

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "ollama-remote", cfg.Embedding.Provider)
	assert.Equal(t, "http://192.168.1.100:11434", cfg.Embedding.Ollama.URL)
}

func TestConfigProviderCmd_Direct_InvalidProvider(t *testing.T) {
	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"invalid-provider"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestConfigProviderCmd_Direct_OpenAI_MissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "") // Ensure no env key

	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"openai"}) // No --api-key flag

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key required")
}

func TestConfigProviderCmd_Direct_OpenAI_EnvKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"openai"}) // Key from env

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	// API key not stored in config (uses env)
	assert.Empty(t, cfg.Embedding.OpenAI.APIKey)
}

func TestConfigProviderCmd_Direct_Voyage_WithKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("VOYAGE_API_KEY", "") // Clear env

	// Reset and set flags
	configProviderAPIKey = "pa-test"
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"voyage", "--api-key", "pa-test"})

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "voyage", cfg.Embedding.Provider)
	assert.Equal(t, "pa-test", cfg.Embedding.Voyage.APIKey)
}

func TestConfigProviderCmd_Direct_Voyage_EnvKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("VOYAGE_API_KEY", "pa-from-env")

	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"voyage"}) // Key from env

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "voyage", cfg.Embedding.Provider)
	assert.Empty(t, cfg.Embedding.Voyage.APIKey)
}

func TestConfigProviderCmd_Direct_OllamaRemote_MissingURL(t *testing.T) {
	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"ollama-remote"}) // No --url flag

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--url is required")
}

func TestConfigProviderCmd_Direct_WithModel(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Reset and set flags
	configProviderAPIKey = ""
	configProviderURL = ""
	configProviderModel = "custom-model"

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"ollama", "--model", "custom-model"})

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "ollama", cfg.Embedding.Provider)
	assert.Equal(t, "custom-model", cfg.Embedding.Ollama.Model)
}

func TestConfigProviderCmd_Interactive_Voyage(t *testing.T) {
	// Simulate: select Voyage (4), enter API key
	input := strings.NewReader("4\npa-test-key\n")
	output := &bytes.Buffer{}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	mockValidator := &mockAPIValidator{valid: true}

	cmd := NewConfigProviderCmdWithValidator(mockValidator)
	cmd.SetIn(input)
	cmd.SetOut(output)

	err := cmd.Execute()
	require.NoError(t, err)

	cfg, err := config.LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "voyage", cfg.Embedding.Provider)
	assert.Equal(t, "pa-test-key", cfg.Embedding.Voyage.APIKey)
}

// === Config Persistence Tests ===

func TestConfigProviderCmd_SavesGlobalConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Reset flags
	configProviderAPIKey = ""
	configProviderURL = ""

	cmd := NewConfigProviderCmd()
	cmd.SetArgs([]string{"ollama"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was created
	configPath := filepath.Join(tempDir, "pommel", "config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}
