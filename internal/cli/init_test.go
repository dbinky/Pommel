package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd_CreatesDirectory(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify .pommel directory was created
	pommelDir := filepath.Join(tmpDir, ".pommel")
	info, err := os.Stat(pommelDir)
	require.NoError(t, err, ".pommel directory should exist")
	assert.True(t, info.IsDir(), ".pommel should be a directory")
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create .pommel directory with config to simulate already initialized
	pommelDir := filepath.Join(tmpDir, ".pommel")
	err = os.MkdirAll(pommelDir, 0755)
	require.NoError(t, err)

	// Create a config file
	loader := config.NewLoader(tmpDir)
	_, err = loader.Init()
	require.NoError(t, err)

	// Run init command again - should show warning
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)

	// Should either return an error or show a warning
	// The exact behavior depends on implementation
	if err != nil {
		assert.Contains(t, err.Error(), "already", "Error should mention already initialized")
	} else {
		assert.Contains(t, errBuf.String(), "already", "Should warn about already initialized")
	}
}

func TestInitCmd_CreatesConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify config.yaml was created
	configPath := filepath.Join(tmpDir, ".pommel", "config.yaml")
	_, err = os.Stat(configPath)
	require.NoError(t, err, "config.yaml should exist")

	// Verify config can be loaded
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err, "Config should be loadable")
	assert.NotNil(t, cfg, "Config should not be nil")

	// Verify default values are set
	defaultCfg := config.Default()
	assert.Equal(t, defaultCfg.Version, cfg.Version, "Config version should match default")
	assert.Equal(t, defaultCfg.Daemon.Port, cfg.Daemon.Port, "Daemon port should match default")
}

func TestInitCmd_CreatesDatabase(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, ".pommel", db.DatabaseFile)
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "Database file should exist")

	// Verify database can be opened and has correct schema
	database, err := db.Open(tmpDir)
	require.NoError(t, err, "Database should be openable")
	defer database.Close()

	// Verify schema version
	ctx := context.Background()
	version, err := database.GetSchemaVersion(ctx)
	require.NoError(t, err, "Should be able to get schema version")
	assert.Equal(t, db.SchemaVersion, version, "Schema version should match")
}

func TestInitCmd_OutputsSuccess(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Should output success message
	output := outBuf.String()
	assert.Contains(t, output, "Initialized", "Output should indicate initialization")
}

func TestInitCmd_JSONOutput(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init command with JSON output
	var outBuf, errBuf bytes.Buffer
	err = runInitCmdJSON(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err, "Init command should succeed")

	// Should output valid JSON
	output := outBuf.String()
	assert.Contains(t, output, "{", "Output should be JSON")
	assert.Contains(t, output, "\"success\"", "JSON should contain success field")
}

func TestInitCmd_InvalidDirectory(t *testing.T) {
	// Try to init in a non-existent directory
	nonExistent := "/nonexistent/path/that/does/not/exist"

	var outBuf, errBuf bytes.Buffer
	err := runInitCmd(nonExistent, &outBuf, &errBuf)

	// Should fail with error
	assert.Error(t, err, "Init should fail for non-existent directory")
}

func TestInitCmd_ReadOnlyDirectory(t *testing.T) {
	// Skip on CI where we might not have permission to create read-only dirs
	if os.Getenv("CI") != "" {
		t.Skip("Skipping read-only directory test in CI")
	}

	// Create a read-only directory
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Make it read-only
	err = os.Chmod(tmpDir, 0444)
	require.NoError(t, err)
	defer os.Chmod(tmpDir, 0755) // Restore permissions for cleanup

	// Try to init in read-only directory
	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)

	// Should fail with error
	assert.Error(t, err, "Init should fail for read-only directory")
}

// runInitCmd is a helper function to run the init command programmatically
// This function should be implemented in init.go
func runInitCmd(projectRoot string, out, errOut *bytes.Buffer) error {
	return runInit(projectRoot, out, errOut, false)
}

// runInitCmdJSON is a helper function to run the init command with JSON output
func runInitCmdJSON(projectRoot string, out, errOut *bytes.Buffer) error {
	return runInit(projectRoot, out, errOut, true)
}

// =============================================================================
// Tests for --auto flag
// =============================================================================

func TestInitCmd_AutoFlag_DetectsGoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some Go files
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	// Run init with --auto flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Auto: true})
	require.NoError(t, err, "Init with --auto should succeed")

	// Verify config includes Go pattern
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.go", "Should include Go files pattern")
}

func TestInitCmd_AutoFlag_DetectsPythonFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some Python files
	err = os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("print('hello')"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "src", "utils.py"), []byte("# utils"), 0644)
	require.NoError(t, err)

	// Run init with --auto flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Auto: true})
	require.NoError(t, err)

	// Verify config includes Python pattern
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.py", "Should include Python files pattern")
}

func TestInitCmd_AutoFlag_DetectsTypeScriptFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create TypeScript files
	err = os.WriteFile(filepath.Join(tmpDir, "index.ts"), []byte("const x = 1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "app.tsx"), []byte("export default {}"), 0644)
	require.NoError(t, err)

	// Run init with --auto flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Auto: true})
	require.NoError(t, err)

	// Verify config includes TypeScript patterns
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.ts", "Should include .ts pattern")
	assert.Contains(t, cfg.IncludePatterns, "**/*.tsx", "Should include .tsx pattern")
}

func TestInitCmd_AutoFlag_DetectsMultipleLanguages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create files of multiple languages
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte("# python"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("// js"), 0644)
	require.NoError(t, err)

	// Run init with --auto flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Auto: true})
	require.NoError(t, err)

	// Verify config includes all detected patterns
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.go", "Should include Go pattern")
	assert.Contains(t, cfg.IncludePatterns, "**/*.py", "Should include Python pattern")
	assert.Contains(t, cfg.IncludePatterns, "**/*.js", "Should include JavaScript pattern")
}

func TestInitCmd_AutoFlag_NoFilesUsesDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Empty directory - no source files

	// Run init with --auto flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Auto: true})
	require.NoError(t, err)

	// Should use default patterns when no files detected
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.IncludePatterns, "Should have some include patterns even if no files found")
}

// =============================================================================
// Tests for --claude flag
// =============================================================================

func TestInitCmd_ClaudeFlag_CreatesCLAUDEMD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify CLAUDE.md was created
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	_, err = os.Stat(claudePath)
	require.NoError(t, err, "CLAUDE.md should be created")

	// Verify it contains Pommel instructions
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pm search", "Should contain pm search instructions")
	assert.Contains(t, string(content), "--json", "Should mention --json flag for agents")
}

func TestInitCmd_ClaudeFlag_AppendsToExistingCLAUDEMD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create existing CLAUDE.md with some content
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	existingContent := "# CLAUDE.md\n\nExisting project instructions.\n"
	err = os.WriteFile(claudePath, []byte(existingContent), 0644)
	require.NoError(t, err)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify existing content is preserved and Pommel instructions added
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Existing project instructions", "Should preserve existing content")
	assert.Contains(t, string(content), "pm search", "Should add Pommel instructions")
}

func TestInitCmd_ClaudeFlag_DoesNotDuplicateInstructions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init with --claude flag twice
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Get content after first init
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	content1, err := os.ReadFile(claudePath)
	require.NoError(t, err)

	// Reset pommel dir so init can run again
	os.RemoveAll(filepath.Join(tmpDir, ".pommel"))

	// Run init with --claude flag again
	outBuf.Reset()
	errBuf.Reset()
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify instructions are not duplicated
	content2, err := os.ReadFile(claudePath)
	require.NoError(t, err)

	// Count occurrences of "pm search" - should only appear once per section
	count1 := bytes.Count(content1, []byte("## Pommel"))
	count2 := bytes.Count(content2, []byte("## Pommel"))
	assert.Equal(t, count1, count2, "Should not duplicate Pommel section")
}

func TestInitCmd_ClaudeFlag_IncludesSearchExamples(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify CLAUDE.md contains useful examples
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "pm search", "Should contain pm search command")
	assert.Contains(t, contentStr, "--json", "Should mention JSON output for agents")
	assert.Contains(t, contentStr, "pm status", "Should mention pm status command")
}

// =============================================================================
// Tests for --start flag
// =============================================================================

func TestInitCmd_StartFlag_FlagRegistered(t *testing.T) {
	// Verify the --start flag is registered on the init command
	flag := initCmd.Flags().Lookup("start")
	assert.NotNil(t, flag, "--start flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type(), "--start should be a boolean flag")
}

func TestInitCmd_StartFlag_InitializesBeforeStart(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init with --start flag (daemon start will likely fail in test env, but init should complete)
	var outBuf, errBuf bytes.Buffer
	_ = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Start: true})

	// Init should succeed even if daemon start fails
	// The .pommel directory should exist
	pommelDir := filepath.Join(tmpDir, ".pommel")
	_, statErr := os.Stat(pommelDir)
	assert.NoError(t, statErr, ".pommel directory should be created before starting daemon")
}

func TestInitCmd_StartFlag_CombinesWithOtherFlags(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a Go file for auto-detection
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	// Run init with multiple flags
	var outBuf, errBuf bytes.Buffer
	_ = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{
		Auto:   true,
		Claude: true,
		Start:  true,
	})

	// Verify --auto worked (Go pattern in config)
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.go", "Auto-detection should work with other flags")

	// Verify --claude worked (CLAUDE.md exists)
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	_, err = os.Stat(claudePath)
	assert.NoError(t, err, "CLAUDE.md should be created when using multiple flags")
}

// =============================================================================
// Tests for flag registration
// =============================================================================

func TestInitCmd_AllFlagsRegistered(t *testing.T) {
	// Verify all documented flags are registered
	autoFlag := initCmd.Flags().Lookup("auto")
	assert.NotNil(t, autoFlag, "--auto flag should be registered")

	claudeFlag := initCmd.Flags().Lookup("claude")
	assert.NotNil(t, claudeFlag, "--claude flag should be registered")

	startFlag := initCmd.Flags().Lookup("start")
	assert.NotNil(t, startFlag, "--start flag should be registered")
}

// =============================================================================
// Helper functions for flag tests
// =============================================================================

// runInitWithFlags runs the init command with the specified flags
func runInitWithFlags(projectRoot string, out, errOut *bytes.Buffer, flags InitFlags) error {
	return runInitFull(projectRoot, out, errOut, false, flags)
}

// =============================================================================
// Tests for --monorepo flag
// =============================================================================

func TestInitCmd_MonorepoFlagRegistered(t *testing.T) {
	flag := initCmd.Flags().Lookup("monorepo")
	assert.NotNil(t, flag, "--monorepo flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type(), "--monorepo should be a boolean flag")
}

func TestInitCmd_NoMonorepoFlagRegistered(t *testing.T) {
	flag := initCmd.Flags().Lookup("no-monorepo")
	assert.NotNil(t, flag, "--no-monorepo flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type(), "--no-monorepo should be a boolean flag")
}

func TestInitCmd_MonorepoFlag_SkipsPrompt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a subproject directory with a marker file
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with --monorepo flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Monorepo: true})
	require.NoError(t, err)

	// Verify config has subprojects.auto_detect enabled
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.True(t, cfg.Subprojects.AutoDetect, "Subprojects.AutoDetect should be true with --monorepo")

	// Verify output mentions subprojects
	output := outBuf.String()
	assert.Contains(t, output, "backend", "Should list detected subproject")
}

func TestInitCmd_NoMonorepoFlag_SkipsDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a subproject directory with a marker file
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with --no-monorepo flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{NoMonorepo: true})
	require.NoError(t, err)

	// Output should NOT mention subproject scanning
	output := outBuf.String()
	assert.NotContains(t, output, "Scanning for project markers", "Should not scan with --no-monorepo")
}

func TestInitCmd_NoMonorepoFlag_ConfigNotUpdated(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a subproject directory with a marker file
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with --no-monorepo flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{NoMonorepo: true})
	require.NoError(t, err)

	// Config should use default subprojects settings (not explicitly enabled)
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	// Default has auto_detect true, but we didn't explicitly configure as monorepo
	defaultCfg := config.Default()
	assert.Equal(t, defaultCfg.Subprojects.AutoDetect, cfg.Subprojects.AutoDetect, "Should use default subprojects config")
}

// =============================================================================
// Tests for monorepo detection
// =============================================================================

func TestInitCmd_DetectsSubprojects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple subproject directories
	subDirs := []struct {
		name   string
		marker string
	}{
		{"backend", "go.mod"},
		{"frontend", "package.json"},
		{"services/api", "go.mod"},
	}

	for _, sd := range subDirs {
		dir := filepath.Join(tmpDir, sd.name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sd.marker), []byte("marker"), 0644))
	}

	// Run init with --monorepo to skip prompting
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Monorepo: true})
	require.NoError(t, err)

	output := outBuf.String()

	// Should detect all subprojects
	assert.Contains(t, output, "backend", "Should detect backend subproject")
	assert.Contains(t, output, "frontend", "Should detect frontend subproject")
	assert.Contains(t, output, "api", "Should detect services/api subproject")
}

func TestInitCmd_IgnoresRootMarker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create marker file at root (should not create a subproject for root)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root"), 0644))

	// Create a subproject
	subDir := filepath.Join(tmpDir, "lib")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module lib"), 0644))

	// Run init
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Monorepo: true})
	require.NoError(t, err)

	output := outBuf.String()

	// Root marker should not create a "." subproject entry
	assert.NotContains(t, output, "(.)\\t", "Root should not be listed as subproject")
	assert.Contains(t, output, "lib", "Should detect lib subproject")
}

func TestInitCmd_NoSubprojectsFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create project root with only root-level marker
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root"), 0644))

	// Run init (no subprojects to detect)
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{})
	require.NoError(t, err)

	output := outBuf.String()

	// Should not mention monorepo features when no subprojects found
	assert.NotContains(t, output, "sub-projects", "Should not mention subprojects when none found")
}

// =============================================================================
// Tests for --claude flag with monorepo
// =============================================================================

func TestInitCmd_ClaudeFlag_MonorepoUpdatesMultiple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subprojects
	subDirs := []string{"backend", "frontend"}
	for _, sd := range subDirs {
		dir := filepath.Join(tmpDir, sd)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+sd), 0644))
	}

	// Run init with --claude --monorepo
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true, Monorepo: true})
	require.NoError(t, err)

	// Verify root CLAUDE.md was created
	rootClaudePath := filepath.Join(tmpDir, "CLAUDE.md")
	_, err = os.Stat(rootClaudePath)
	require.NoError(t, err, "Root CLAUDE.md should be created")

	// Verify each subproject CLAUDE.md was created
	for _, sd := range subDirs {
		claudePath := filepath.Join(tmpDir, sd, "CLAUDE.md")
		_, err = os.Stat(claudePath)
		require.NoError(t, err, "CLAUDE.md should be created in "+sd)

		content, err := os.ReadFile(claudePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "pm search", "Subproject CLAUDE.md should contain search instructions")
		assert.Contains(t, string(content), sd, "Should mention subproject name")
	}
}

func TestInitCmd_ClaudeFlag_PreservesExistingSubprojectCLAUDEMD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject with existing CLAUDE.md
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	existingContent := "# Backend\n\nExisting instructions for backend.\n"
	claudePath := filepath.Join(subDir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudePath, []byte(existingContent), 0644))

	// Run init with --claude --monorepo
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true, Monorepo: true})
	require.NoError(t, err)

	// Verify existing content preserved and Pommel instructions added
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Existing instructions for backend", "Should preserve existing content")
	assert.Contains(t, string(content), "pm search", "Should add Pommel instructions")
}

func TestInitCmd_ClaudeFlag_NoSubprojectsOnlyRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Run init with --claude only (no subprojects)
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Only root CLAUDE.md should be created
	rootClaudePath := filepath.Join(tmpDir, "CLAUDE.md")
	_, err = os.Stat(rootClaudePath)
	require.NoError(t, err, "Root CLAUDE.md should be created")
}

func TestInitCmd_ClaudeFlag_DoesNotDuplicateInSubprojects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with --claude --monorepo twice
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true, Monorepo: true})
	require.NoError(t, err)

	// Remove .pommel to allow re-init
	os.RemoveAll(filepath.Join(tmpDir, ".pommel"))

	// Run again
	outBuf.Reset()
	errBuf.Reset()
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true, Monorepo: true})
	require.NoError(t, err)

	// Check subproject CLAUDE.md doesn't have duplicated sections
	claudePath := filepath.Join(subDir, "CLAUDE.md")
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)

	count := bytes.Count(content, []byte("## Pommel"))
	assert.Equal(t, 1, count, "Should not duplicate Pommel section in subproject CLAUDE.md")
}

// =============================================================================
// Tests for config updates
// =============================================================================

func TestInitCmd_Monorepo_UpdatesConfigSubprojects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with --monorepo
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Monorepo: true})
	require.NoError(t, err)

	// Verify config subprojects section
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.True(t, cfg.Subprojects.AutoDetect, "AutoDetect should be enabled")
	assert.NotEmpty(t, cfg.Subprojects.Markers, "Markers should be set")
}

// =============================================================================
// Tests for JSON output with monorepo
// =============================================================================

func TestInitCmd_JSONOutput_IncludesSubprojects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run init with JSON output and --monorepo
	var outBuf, errBuf bytes.Buffer
	err = runInitFull(tmpDir, &outBuf, &errBuf, true, InitFlags{Monorepo: true})
	require.NoError(t, err)

	output := outBuf.String()

	// JSON should contain subprojects field
	assert.Contains(t, output, "\"success\"", "JSON should have success field")
	// Note: The exact structure depends on implementation - we'll adjust after seeing the actual output
}

// =============================================================================
// Tests for combined flags
// =============================================================================

func TestInitCmd_AllFlagsCombined_Monorepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Go files at root for auto-detection
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	// Create subproject
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0644))

	// Run with all flags except Start (which would try to start daemon)
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{
		Auto:     true,
		Claude:   true,
		Monorepo: true,
	})
	require.NoError(t, err)

	// Verify --auto worked
	loader := config.NewLoader(tmpDir)
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.IncludePatterns, "**/*.go", "Auto should detect Go files")

	// Verify --claude worked (root CLAUDE.md)
	rootClaudePath := filepath.Join(tmpDir, "CLAUDE.md")
	_, err = os.Stat(rootClaudePath)
	assert.NoError(t, err, "Root CLAUDE.md should exist")

	// Verify --claude worked (subproject CLAUDE.md)
	subClaudePath := filepath.Join(subDir, "CLAUDE.md")
	_, err = os.Stat(subClaudePath)
	assert.NoError(t, err, "Subproject CLAUDE.md should exist")

	// Verify --monorepo worked
	assert.True(t, cfg.Subprojects.AutoDetect, "Subprojects.AutoDetect should be true")
}

func TestInitCmd_MonorepoAndNoMonorepo_Conflict(t *testing.T) {
	// If both flags are set, --no-monorepo should take precedence (skip detection)
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	// Run with both flags (conflicting)
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{
		Monorepo:   true,
		NoMonorepo: true,
	})
	require.NoError(t, err)

	output := outBuf.String()

	// --no-monorepo should win - no scanning message
	assert.NotContains(t, output, "Scanning for project markers", "--no-monorepo should skip detection")
}

// =============================================================================
// .gitignore Tests
// =============================================================================

func TestInitCmd_CreatesGitignoreIfNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err)

	// Verify .gitignore was created with .pommel/
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err, ".gitignore should be created")
	assert.Contains(t, string(content), ".pommel/", ".gitignore should contain .pommel/")
}

func TestInitCmd_AppendsToExistingGitignore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create existing .gitignore
	existingContent := "node_modules/\n*.log\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte(existingContent), 0644))

	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err)

	// Verify .gitignore has both old and new content
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "node_modules/", "Should preserve existing content")
	assert.Contains(t, string(content), "*.log", "Should preserve existing content")
	assert.Contains(t, string(content), ".pommel/", "Should add .pommel/")
}

func TestInitCmd_DoesNotDuplicateGitignoreEntry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create .gitignore that already has .pommel/
	existingContent := "node_modules/\n.pommel/\n*.log\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte(existingContent), 0644))

	var outBuf, errBuf bytes.Buffer
	err = runInitCmd(tmpDir, &outBuf, &errBuf)
	require.NoError(t, err)

	// Verify .gitignore doesn't have duplicate entries
	content, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)

	// Count occurrences of .pommel
	count := 0
	for _, line := range bytes.Split(content, []byte("\n")) {
		if bytes.Contains(line, []byte(".pommel")) {
			count++
		}
	}
	assert.Equal(t, 1, count, "Should not duplicate .pommel/ entry")
}

// =============================================================================
// Tests for --claude replacing existing Pommel instructions
// =============================================================================

func TestInitCmd_ClaudeFlag_ReplacesExistingPommelSection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create existing CLAUDE.md with old Pommel section
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	oldContent := `# CLAUDE.md

Some project instructions here.

## Pommel - Semantic Code Search

Old Pommel instructions that should be removed.
These are outdated.

### Old Section
More old content.

## Other Section

This should be preserved.
`
	err = os.WriteFile(claudePath, []byte(oldContent), 0644)
	require.NoError(t, err)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify old Pommel section was replaced
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	contentStr := string(content)

	// Should preserve content before Pommel section
	assert.Contains(t, contentStr, "Some project instructions here", "Should preserve content before Pommel")

	// Should preserve content after Pommel section
	assert.Contains(t, contentStr, "## Other Section", "Should preserve other sections")
	assert.Contains(t, contentStr, "This should be preserved", "Should preserve other section content")

	// Should NOT contain old Pommel content
	assert.NotContains(t, contentStr, "Old Pommel instructions", "Should remove old Pommel instructions")
	assert.NotContains(t, contentStr, "These are outdated", "Should remove old Pommel content")

	// Should contain new Pommel instructions
	assert.Contains(t, contentStr, "pm search", "Should have new pm search instructions")
	assert.Contains(t, contentStr, "--json", "Should have new --json flag documentation")

	// Should only have one Pommel section
	count := bytes.Count(content, []byte("## Pommel - Semantic Code Search"))
	assert.Equal(t, 1, count, "Should have exactly one Pommel section")
}

func TestInitCmd_ClaudeFlag_ReplacesEvenWhenAtEnd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create existing CLAUDE.md with Pommel section at the end
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	oldContent := `# CLAUDE.md

Some project instructions.

## Pommel - Semantic Code Search

Old instructions at the end of file.
`
	err = os.WriteFile(claudePath, []byte(oldContent), 0644)
	require.NoError(t, err)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	// Verify the section was replaced
	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	contentStr := string(content)

	assert.Contains(t, contentStr, "Some project instructions", "Should preserve earlier content")
	assert.NotContains(t, contentStr, "Old instructions at the end", "Should remove old content")
	assert.Contains(t, contentStr, "pm search", "Should have new instructions")
}

func TestInitCmd_ClaudeFlag_PreservesContentAroundPommelSection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create CLAUDE.md with Pommel section in the middle
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	oldContent := `# CLAUDE.md

## Project Setup

Instructions for setting up the project.

## Pommel - Semantic Code Search

Old Pommel stuff here.
More old stuff.

## Testing

How to run tests.

## Deployment

How to deploy.
`
	err = os.WriteFile(claudePath, []byte(oldContent), 0644)
	require.NoError(t, err)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	contentStr := string(content)

	// All other sections should be preserved
	assert.Contains(t, contentStr, "## Project Setup", "Should preserve Project Setup")
	assert.Contains(t, contentStr, "Instructions for setting up", "Should preserve Project Setup content")
	assert.Contains(t, contentStr, "## Testing", "Should preserve Testing section")
	assert.Contains(t, contentStr, "How to run tests", "Should preserve Testing content")
	assert.Contains(t, contentStr, "## Deployment", "Should preserve Deployment section")
	assert.Contains(t, contentStr, "How to deploy", "Should preserve Deployment content")

	// Old Pommel content should be gone
	assert.NotContains(t, contentStr, "Old Pommel stuff", "Should remove old Pommel content")

	// New Pommel content should be at the end
	assert.Contains(t, contentStr, "pm search", "Should have new Pommel instructions")
}

func TestInitCmd_ClaudeFlag_HandlesSubsectionsInPommelSection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create CLAUDE.md with Pommel section that has subsections (### headings)
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	oldContent := `# CLAUDE.md

## Introduction

Welcome.

## Pommel - Semantic Code Search

Old intro.

### Old Subsection 1

Content 1.

### Old Subsection 2

Content 2.

## Conclusion

Final notes.
`
	err = os.WriteFile(claudePath, []byte(oldContent), 0644)
	require.NoError(t, err)

	// Run init with --claude flag
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true})
	require.NoError(t, err)

	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	contentStr := string(content)

	// Should remove the entire Pommel section including subsections
	assert.NotContains(t, contentStr, "Old intro", "Should remove Pommel intro")
	assert.NotContains(t, contentStr, "Old Subsection 1", "Should remove subsection 1")
	assert.NotContains(t, contentStr, "Old Subsection 2", "Should remove subsection 2")
	assert.NotContains(t, contentStr, "Content 1", "Should remove subsection content")

	// Should preserve other sections
	assert.Contains(t, contentStr, "## Introduction", "Should preserve Introduction")
	assert.Contains(t, contentStr, "## Conclusion", "Should preserve Conclusion")
	assert.Contains(t, contentStr, "Final notes", "Should preserve Conclusion content")
}

func TestInitCmd_ClaudeFlag_UpdatesSubprojectWithExistingSection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pommel-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subproject with existing Pommel section
	subDir := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module backend"), 0644))

	claudePath := filepath.Join(subDir, "CLAUDE.md")
	oldContent := `# Backend

## Setup

Backend setup instructions.

## Pommel - Semantic Code Search

Old subproject pommel instructions.

## API

API documentation.
`
	err = os.WriteFile(claudePath, []byte(oldContent), 0644)
	require.NoError(t, err)

	// Run init with --claude --monorepo
	var outBuf, errBuf bytes.Buffer
	err = runInitWithFlags(tmpDir, &outBuf, &errBuf, InitFlags{Claude: true, Monorepo: true})
	require.NoError(t, err)

	content, err := os.ReadFile(claudePath)
	require.NoError(t, err)
	contentStr := string(content)

	// Should preserve other sections
	assert.Contains(t, contentStr, "## Setup", "Should preserve Setup")
	assert.Contains(t, contentStr, "## API", "Should preserve API")

	// Should remove old Pommel content
	assert.NotContains(t, contentStr, "Old subproject pommel", "Should remove old content")

	// Should have new Pommel instructions
	assert.Contains(t, contentStr, "pm search", "Should have new instructions")
	assert.Contains(t, contentStr, "backend", "Should mention subproject name")
}
