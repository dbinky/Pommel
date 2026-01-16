package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigModel_ShowCurrent_Default(t *testing.T) {
	// Setup: create temp project with default config
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Save and restore original projectRoot
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "v2")
	assert.Contains(t, stdout.String(), "unclemusclez/jina-embeddings-v2-base-code")
}

func TestConfigModel_ShowCurrent_V4(t *testing.T) {
	// Setup: create temp project with v4 config
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "sellerscrisp/jina-embeddings-v4-text-code-q4"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Save and restore original projectRoot
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "v4")
	assert.Contains(t, stdout.String(), "sellerscrisp/jina-embeddings-v4-text-code-q4")
}

func TestConfigModel_ShowCurrent_CustomModel(t *testing.T) {
	// Setup: create temp project with custom model
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "my-custom-embedding-model"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Save and restore original projectRoot
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "custom")
	assert.Contains(t, stdout.String(), "my-custom-embedding-model")
}

func TestConfigModel_ShowCurrent_EmptyModel(t *testing.T) {
	// Setup: create temp project with no model configured (should use default)
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Save and restore original projectRoot
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	// Should show default v2 model
	assert.Contains(t, stdout.String(), "v2")
	assert.Contains(t, stdout.String(), "unclemusclez/jina-embeddings-v2-base-code")
}

func TestConfigModel_NoConfigFile(t *testing.T) {
	// Setup: temp directory without config
	projectDir := t.TempDir()

	// Save and restore original projectRoot
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Assert - should fail because no config exists
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestConfigModel_SetV4_NoExistingDB(t *testing.T) {
	// Setup: create temp project with v2 config, no database
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	configPath := filepath.Join(pommelDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Execute
	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"v4"})
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "v4")
	assert.Contains(t, stdout.String(), "pm start")

	// Verify config was updated
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "sellerscrisp/jina-embeddings-v4-text-code-q4")
}

func TestConfigModel_SameModel(t *testing.T) {
	// Setup: create temp project with v2 config
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"v2"})
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Already using")
}

func TestConfigModel_InvalidModel(t *testing.T) {
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	cmd := newConfigModelCmd()
	cmd.SetArgs([]string{"v5"})
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model 'v5'")
}

func TestConfigModel_DeletesExistingDB(t *testing.T) {
	// Setup: create temp project with v2 config AND existing database
	projectDir := t.TempDir()
	pommelDir := filepath.Join(projectDir, ".pommel")
	require.NoError(t, os.MkdirAll(pommelDir, 0755))

	configContent := `version: "1"
embedding:
  provider: ollama
  ollama:
    model: "unclemusclez/jina-embeddings-v2-base-code"
`
	require.NoError(t, os.WriteFile(filepath.Join(pommelDir, "config.yaml"), []byte(configContent), 0644))

	// Create a dummy database file
	dbPath := filepath.Join(pommelDir, "pommel.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("dummy db content"), 0644))

	var stdout bytes.Buffer
	cmd := newConfigModelCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"v4"})
	origProjectRoot := projectRoot
	defer func() { projectRoot = origProjectRoot }()
	projectRoot = projectDir

	err := cmd.Execute()

	require.NoError(t, err)
	// Verify database was deleted
	_, err = os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "database should be deleted")
}
