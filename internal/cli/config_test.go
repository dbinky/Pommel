package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProject is a helper that creates a temporary project directory with Pommel config
type testProject struct {
	Dir    string
	Config *config.Config
	Loader *config.Loader
}

// newTestProject creates a new temporary project with default config
func newTestProject(t *testing.T) *testProject {
	t.Helper()

	dir := t.TempDir()
	loader := config.NewLoader(dir)

	cfg, err := loader.Init()
	require.NoError(t, err)

	return &testProject{
		Dir:    dir,
		Config: cfg,
		Loader: loader,
	}
}

// executeConfigCmd executes the config command with given args and returns output
func executeConfigCmd(t *testing.T, projectDir string, args ...string) (string, string, error) {
	t.Helper()

	// Save original values
	origProjectRoot := projectRoot
	origJSONOutput := jsonOutput

	// Cleanup
	defer func() {
		projectRoot = origProjectRoot
		jsonOutput = origJSONOutput
	}()

	// Set up for test
	projectRoot = projectDir
	jsonOutput = false

	// Check if --json flag is in args
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Capture output
	var outBuf, errBuf bytes.Buffer
	configCmd.SetOut(&outBuf)
	configCmd.SetErr(&errBuf)

	// Call runConfig directly instead of using Execute() to avoid help text issues
	err := runConfig(configCmd, filteredArgs)

	return outBuf.String(), errBuf.String(), err
}

func TestConfigCmd_ShowAll(t *testing.T) {
	// Test that 'pm config' with no args shows full config
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir)

	// Should succeed
	require.NoError(t, err, "config command should succeed with no args")

	// Should contain key config sections
	assert.Contains(t, stdout, "daemon", "output should contain daemon section")
	assert.Contains(t, stdout, "embedding", "output should contain embedding section")
	assert.Contains(t, stdout, "watcher", "output should contain watcher section")
	assert.Contains(t, stdout, "search", "output should contain search section")

	// Should show port field (nil means hash-based, shown as "null")
	assert.Contains(t, stdout, "port", "output should contain port field")
}

func TestConfigCmd_ShowAllJSON(t *testing.T) {
	// Test that 'pm config --json' outputs valid JSON
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "--json")

	// Should succeed
	require.NoError(t, err, "config command with --json should succeed")

	// Should be valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Should contain expected sections
	assert.Contains(t, result, "daemon", "JSON should contain daemon")
	assert.Contains(t, result, "embedding", "JSON should contain embedding")
	assert.Contains(t, result, "watcher", "JSON should contain watcher")
	assert.Contains(t, result, "search", "JSON should contain search")

	// Check nested values
	daemon, ok := result["daemon"].(map[string]interface{})
	require.True(t, ok, "daemon should be an object")
	assert.Nil(t, daemon["port"], "daemon.port should be nil (hash-based default)")
}

func TestConfigCmd_Get(t *testing.T) {
	// Test that 'pm config get daemon.port' returns the port value (nil for default)
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon.port")

	// Should succeed
	require.NoError(t, err, "config get daemon.port should succeed")

	// Should output nil for default (hash-based port)
	assert.Contains(t, stdout, "<nil>", "output should indicate nil (hash-based port)")
}

func TestConfigCmd_GetNested(t *testing.T) {
	// Test that 'pm config get daemon' returns the entire daemon section
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon")

	// Should succeed
	require.NoError(t, err, "config get daemon should succeed")

	// Should contain all daemon fields
	assert.Contains(t, stdout, "host", "output should contain host")
	assert.Contains(t, stdout, "port", "output should contain port")
	assert.Contains(t, stdout, "log_level", "output should contain log_level")
	assert.Contains(t, stdout, "127.0.0.1", "output should contain default host value")
	assert.Contains(t, stdout, "null", "output should contain null for default port (hash-based)")
	assert.Contains(t, stdout, "info", "output should contain default log_level value")
}

func TestConfigCmd_GetJSON(t *testing.T) {
	// Test that 'pm config get daemon.port --json' returns JSON
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "daemon.port", "--json")

	// Should succeed
	require.NoError(t, err, "config get daemon.port --json should succeed")

	// Should be valid JSON with the value
	var result map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Nil(t, result["value"], "JSON value should be nil for default (hash-based port)")
	assert.Equal(t, "daemon.port", result["key"], "JSON key should be daemon.port")
}

func TestConfigCmd_Set(t *testing.T) {
	// Test that 'pm config set daemon.port 9000' changes config
	proj := newTestProject(t)

	// Execute set command
	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "9000")

	// Should succeed
	require.NoError(t, err, "config set daemon.port 9000 should succeed")

	// Verify the change by loading config
	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Daemon.Port, "daemon.port should not be nil")
	assert.Equal(t, 9000, *cfg.Daemon.Port, "daemon.port should be updated to 9000")
}

func TestConfigCmd_SetString(t *testing.T) {
	// Test setting a string value
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.log_level", "debug")

	require.NoError(t, err, "config set daemon.log_level debug should succeed")

	// Verify the change
	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Daemon.LogLevel, "daemon.log_level should be updated to debug")
}

func TestConfigCmd_SetValidation(t *testing.T) {
	// Test that setting an invalid value returns error
	proj := newTestProject(t)

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		// Note: port 0 is now valid (system-assigned port)
		{
			name:  "invalid port negative",
			key:   "daemon.port",
			value: "-1",
		},
		{
			name:  "invalid port too high",
			key:   "daemon.port",
			value: "70000",
		},
		{
			name:  "invalid log level",
			key:   "daemon.log_level",
			value: "invalid",
		},
		{
			name:  "invalid batch size",
			key:   "embedding.batch_size",
			value: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeConfigCmd(t, proj.Dir, "set", tc.key, tc.value)
			assert.Error(t, err, "setting invalid value should return error")
		})
	}
}

func TestConfigCmd_UnknownKey(t *testing.T) {
	// Test that getting/setting unknown key returns error
	proj := newTestProject(t)

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "get unknown key",
			args: []string{"get", "unknown.key"},
		},
		{
			name: "get unknown nested key",
			args: []string{"get", "daemon.unknown"},
		},
		{
			name: "set unknown key",
			args: []string{"set", "unknown.key", "value"},
		},
		{
			name: "set unknown nested key",
			args: []string{"set", "daemon.unknown", "value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeConfigCmd(t, proj.Dir, tc.args...)
			require.Error(t, err, "unknown key should return error")
			assert.Contains(t, err.Error(), "unknown", "error should mention unknown key")
		})
	}
}

func TestConfigCmd_SavesPersistently(t *testing.T) {
	// Test that changes persist to disk and survive reload
	proj := newTestProject(t)

	// Set a value
	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "8888")
	require.NoError(t, err)

	// Create a new loader to simulate fresh load
	newLoader := config.NewLoader(proj.Dir)
	cfg, err := newLoader.Load()
	require.NoError(t, err)

	// Verify the value persisted
	require.NotNil(t, cfg.Daemon.Port, "daemon.port should not be nil")
	assert.Equal(t, 8888, *cfg.Daemon.Port, "daemon.port should persist to 8888")

	// Verify config file exists on disk
	configPath := filepath.Join(proj.Dir, config.PommelDir, config.ConfigFileName+"."+config.ConfigFileExt)
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "config file should exist on disk")
}

func TestConfigCmd_NoConfigFile(t *testing.T) {
	// Test behavior when no config file exists
	dir := t.TempDir()

	_, _, err := executeConfigCmd(t, dir)

	// Should fail with meaningful error
	require.Error(t, err, "should error when no config file exists")
	assert.Contains(t, err.Error(), "config", "error should mention config")
}

func TestConfigCmd_InvalidSubcommand(t *testing.T) {
	// Test that invalid subcommand returns error
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "invalid")

	assert.Error(t, err, "invalid subcommand should return error")
}

func TestConfigCmd_SetMissingValue(t *testing.T) {
	// Test that 'set key' without value returns error
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port")

	assert.Error(t, err, "set without value should return error")
}

func TestConfigCmd_SetTypeConversion(t *testing.T) {
	// Test that values are correctly converted to appropriate types

	testCases := []struct {
		name     string
		key      string
		value    string
		validate func(t *testing.T, cfg *config.Config)
	}{
		{
			name:  "integer port",
			key:   "daemon.port",
			value: "9999",
			validate: func(t *testing.T, cfg *config.Config) {
				require.NotNil(t, cfg.Daemon.Port)
				assert.Equal(t, 9999, *cfg.Daemon.Port)
			},
		},
		{
			name:  "string host",
			key:   "daemon.host",
			value: "0.0.0.0",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "0.0.0.0", cfg.Daemon.Host)
			},
		},
		{
			name:  "integer debounce",
			key:   "watcher.debounce_ms",
			value: "1000",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 1000, cfg.Watcher.DebounceMs)
			},
		},
		{
			name:  "large integer max_file_size",
			key:   "watcher.max_file_size",
			value: "10485760",
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, int64(10485760), cfg.Watcher.MaxFileSize)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh project for each test
			testProj := newTestProject(t)

			_, _, err := executeConfigCmd(t, testProj.Dir, "set", tc.key, tc.value)
			require.NoError(t, err)

			cfg, err := testProj.Loader.Load()
			require.NoError(t, err)
			tc.validate(t, cfg)
		})
	}
}

func TestConfigCmd_GetTopLevel(t *testing.T) {
	// Test getting top-level scalar values
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "version")

	require.NoError(t, err, "config get version should succeed")
	assert.Contains(t, stdout, "1", "output should contain version 1")
}

func TestConfigCmd_SetConfirmation(t *testing.T) {
	// Test that set command outputs confirmation
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "9000")

	require.NoError(t, err)
	// Should output some confirmation message
	assert.NotEmpty(t, stdout, "set command should output confirmation")
}

// =============================================================================
// Tests for watcher config get/set
// =============================================================================

func TestConfigCmd_GetWatcher(t *testing.T) {
	proj := newTestProject(t)

	// Get entire watcher section
	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "watcher")
	require.NoError(t, err)

	assert.Contains(t, stdout, "debounce_ms")
	assert.Contains(t, stdout, "max_file_size")
}

func TestConfigCmd_GetWatcherDebounce(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "watcher.debounce_ms")
	require.NoError(t, err)
	// Should contain some numeric value
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_GetWatcherMaxFileSize(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "watcher.max_file_size")
	require.NoError(t, err)
	// Should contain some value
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_SetWatcherDebounce(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "watcher.debounce_ms", "200")
	require.NoError(t, err)

	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 200, cfg.Watcher.DebounceMs)
}

func TestConfigCmd_SetWatcherMaxFileSize(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "watcher.max_file_size", "2097152")
	require.NoError(t, err)

	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, int64(2097152), cfg.Watcher.MaxFileSize)
}

func TestConfigCmd_SetWatcherUnknownKey(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "watcher.unknown", "value")
	require.Error(t, err, "setting unknown key should return error")
	assert.Contains(t, err.Error(), "unknown")
}

// =============================================================================
// Tests for embedding config get/set
// =============================================================================

func TestConfigCmd_GetEmbedding(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "embedding")
	require.NoError(t, err)

	assert.Contains(t, stdout, "model")
	assert.Contains(t, stdout, "batch_size")
	assert.Contains(t, stdout, "cache_size")
}

func TestConfigCmd_GetEmbeddingModel(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "embedding.model")
	require.NoError(t, err)
	// Should contain default model name
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_GetEmbeddingBatchSize(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "embedding.batch_size")
	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_GetEmbeddingCacheSize(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "embedding.cache_size")
	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_SetEmbeddingModel(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "embedding.model", "custom-model")
	require.NoError(t, err)

	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "custom-model", cfg.Embedding.Model)
}

func TestConfigCmd_SetEmbeddingCacheSize(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "embedding.cache_size", "5000")
	require.NoError(t, err)

	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 5000, cfg.Embedding.CacheSize)
}

func TestConfigCmd_SetEmbeddingUnknownKey(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "embedding.unknown", "value")
	require.Error(t, err, "setting unknown key should return error")
	assert.Contains(t, err.Error(), "unknown")
}

// =============================================================================
// Tests for search config get/set
// =============================================================================

func TestConfigCmd_GetSearch(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "search")
	require.NoError(t, err)

	assert.Contains(t, stdout, "default_limit")
}

func TestConfigCmd_GetSearchDefaultLimit(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "search.default_limit")
	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_GetSearchDefaultLevels(t *testing.T) {
	proj := newTestProject(t)

	stdout, _, err := executeConfigCmd(t, proj.Dir, "get", "search.default_levels")
	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
}

func TestConfigCmd_SetSearchDefaultLimit(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "search.default_limit", "20")
	require.NoError(t, err)

	cfg, err := proj.Loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 20, cfg.Search.DefaultLimit)
}

func TestConfigCmd_SetSearchUnknownKey(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "search.unknown", "value")
	require.Error(t, err, "setting unknown key should return error")
	assert.Contains(t, err.Error(), "unknown")
}

// =============================================================================
// Tests for error cases in get/set
// =============================================================================

func TestConfigCmd_GetMissingKey(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "get")
	require.Error(t, err, "get without key should return error")
	// Error message says "Missing key argument"
	errStr := strings.ToLower(err.Error())
	assert.True(t, strings.Contains(errStr, "missing") || strings.Contains(errStr, "requires") || strings.Contains(errStr, "key"),
		"Error should mention missing/requires/key, got: %s", err.Error())
}

func TestConfigCmd_SetInvalidIntValue(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "daemon.port", "not-a-number")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_SetInvalidWatcherDebounce(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "watcher.debounce_ms", "abc")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_SetInvalidWatcherMaxFileSize(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "watcher.max_file_size", "xyz")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_SetInvalidEmbeddingBatchSize(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "embedding.batch_size", "not-int")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_SetInvalidEmbeddingCacheSize(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "embedding.cache_size", "bad")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_SetInvalidSearchLimit(t *testing.T) {
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "set", "search.default_limit", "nope")
	require.Error(t, err, "setting invalid integer should return error")
	assert.Contains(t, strings.ToLower(err.Error()), "invalid")
}

func TestConfigCmd_GetVersionSubkey(t *testing.T) {
	// Test that version.something returns unknown key error
	proj := newTestProject(t)

	_, _, err := executeConfigCmd(t, proj.Dir, "get", "version.something")
	require.Error(t, err, "getting unknown subkey should return error")
	// Error may contain "Unknown" (capital) or "unknown" (lowercase)
	errStr := strings.ToLower(err.Error())
	assert.Contains(t, errStr, "unknown")
}
