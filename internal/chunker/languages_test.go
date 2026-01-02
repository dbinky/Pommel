package chunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

// getLanguagesDir returns the path to the languages directory.
// The languages directory is at the repository root /languages/ relative to this test.
func getLanguagesDir(t *testing.T) string {
	t.Helper()

	// Get current working directory and navigate to repo root
	// Test file is at internal/chunker/languages_test.go
	// Languages dir is at /languages/
	wd, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")

	// Navigate up from internal/chunker to repo root
	repoRoot := filepath.Join(wd, "..", "..")
	languagesDir := filepath.Join(repoRoot, "languages")

	// Verify the directory exists
	info, err := os.Stat(languagesDir)
	require.NoError(t, err, "Languages directory should exist at %s", languagesDir)
	require.True(t, info.IsDir(), "Languages path should be a directory")

	return languagesDir
}

// loadTestLanguageConfig loads a language config from the languages directory.
func loadTestLanguageConfig(t *testing.T, filename string) *LanguageConfig {
	t.Helper()

	languagesDir := getLanguagesDir(t)
	configPath := filepath.Join(languagesDir, filename)

	cfg, err := LoadLanguageConfig(configPath)
	require.NoError(t, err, "Failed to load config %s", filename)
	require.NotNil(t, cfg, "Config should not be nil")

	return cfg
}

// validateLanguageConfig performs standard validation checks on a language config.
func validateLanguageConfig(t *testing.T, name string, cfg *LanguageConfig) {
	t.Helper()

	// Language field must be non-empty
	assert.NotEmpty(t, cfg.Language, "%s: language field should not be empty", name)

	// DisplayName must be non-empty
	assert.NotEmpty(t, cfg.DisplayName, "%s: display_name field should not be empty", name)

	// Extensions must be non-empty
	assert.NotEmpty(t, cfg.Extensions, "%s: extensions field should not be empty", name)

	// All extensions should start with '.'
	for _, ext := range cfg.Extensions {
		assert.True(t, strings.HasPrefix(ext, "."),
			"%s: extension %q should start with '.'", name, ext)
	}

	// TreeSitter.Grammar must be non-empty
	assert.NotEmpty(t, cfg.TreeSitter.Grammar, "%s: tree_sitter.grammar should not be empty", name)

	// Should have at least one chunk mapping (class or method)
	hasClassMapping := len(cfg.ChunkMappings.Class) > 0
	hasMethodMapping := len(cfg.ChunkMappings.Method) > 0
	assert.True(t, hasClassMapping || hasMethodMapping,
		"%s: config should have at least one chunk mapping (class or method)", name)
}

// =============================================================================
// Individual Language Config Tests
// =============================================================================

func TestLanguageConfig_CSharp(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "csharp.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "csharp", cfg)

	// Validate specific expected values
	assert.Equal(t, "csharp", cfg.Language, "Language should be 'csharp'")
	assert.Equal(t, "C#", cfg.DisplayName, "DisplayName should be 'C#'")
	assert.Contains(t, cfg.Extensions, ".cs", "Extensions should contain '.cs'")
	assert.Equal(t, "c_sharp", cfg.TreeSitter.Grammar, "Grammar should be 'c_sharp'")

	// C# should have class mappings for class, struct, interface, record, enum
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "C# should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "struct_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "interface_declaration")

	// C# should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "C# should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "method_declaration")
}

func TestLanguageConfig_Dart(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "dart.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "dart", cfg)

	// Validate specific expected values
	assert.Equal(t, "dart", cfg.Language, "Language should be 'dart'")
	assert.Equal(t, "Dart", cfg.DisplayName, "DisplayName should be 'Dart'")
	assert.Contains(t, cfg.Extensions, ".dart", "Extensions should contain '.dart'")
	assert.Equal(t, "dart", cfg.TreeSitter.Grammar, "Grammar should be 'dart'")

	// Dart should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Dart should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_definition")

	// Dart should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Dart should have method mappings")
}

func TestLanguageConfig_Elixir(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "elixir.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "elixir", cfg)

	// Validate specific expected values
	assert.Equal(t, "elixir", cfg.Language, "Language should be 'elixir'")
	assert.Equal(t, "Elixir", cfg.DisplayName, "DisplayName should be 'Elixir'")
	assert.Contains(t, cfg.Extensions, ".ex", "Extensions should contain '.ex'")
	assert.Contains(t, cfg.Extensions, ".exs", "Extensions should contain '.exs'")
	assert.Equal(t, "elixir", cfg.TreeSitter.Grammar, "Grammar should be 'elixir'")

	// Elixir uses 'call' node for both modules and functions
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Elixir should have class mappings")
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Elixir should have method mappings")
}

func TestLanguageConfig_Go(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "go.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "go", cfg)

	// Validate specific expected values
	assert.Equal(t, "go", cfg.Language, "Language should be 'go'")
	assert.Equal(t, "Go", cfg.DisplayName, "DisplayName should be 'Go'")
	assert.Contains(t, cfg.Extensions, ".go", "Extensions should contain '.go'")
	assert.Equal(t, "go", cfg.TreeSitter.Grammar, "Grammar should be 'go'")

	// Go should have class mappings for struct/interface
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Go should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "type_spec")

	// Go should have method mappings for functions
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Go should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_declaration")
	assert.Contains(t, cfg.ChunkMappings.Method, "method_declaration")
}

func TestLanguageConfig_Java(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "java.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "java", cfg)

	// Validate specific expected values
	assert.Equal(t, "java", cfg.Language, "Language should be 'java'")
	assert.Equal(t, "Java", cfg.DisplayName, "DisplayName should be 'Java'")
	assert.Contains(t, cfg.Extensions, ".java", "Extensions should contain '.java'")
	assert.Equal(t, "java", cfg.TreeSitter.Grammar, "Grammar should be 'java'")

	// Java should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Java should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "interface_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "enum_declaration")

	// Java should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Java should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "method_declaration")
	assert.Contains(t, cfg.ChunkMappings.Method, "constructor_declaration")
}

func TestLanguageConfig_JavaScript(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "javascript.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "javascript", cfg)

	// Validate specific expected values
	assert.Equal(t, "javascript", cfg.Language, "Language should be 'javascript'")
	assert.Equal(t, "JavaScript", cfg.DisplayName, "DisplayName should be 'JavaScript'")
	assert.Contains(t, cfg.Extensions, ".js", "Extensions should contain '.js'")
	assert.Contains(t, cfg.Extensions, ".jsx", "Extensions should contain '.jsx'")
	assert.Contains(t, cfg.Extensions, ".mjs", "Extensions should contain '.mjs'")
	assert.Contains(t, cfg.Extensions, ".cjs", "Extensions should contain '.cjs'")
	assert.Equal(t, "javascript", cfg.TreeSitter.Grammar, "Grammar should be 'javascript'")

	// JavaScript should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "JavaScript should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")

	// JavaScript should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "JavaScript should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_declaration")
	assert.Contains(t, cfg.ChunkMappings.Method, "arrow_function")
}

func TestLanguageConfig_Kotlin(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "kotlin.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "kotlin", cfg)

	// Validate specific expected values
	assert.Equal(t, "kotlin", cfg.Language, "Language should be 'kotlin'")
	assert.Equal(t, "Kotlin", cfg.DisplayName, "DisplayName should be 'Kotlin'")
	assert.Contains(t, cfg.Extensions, ".kt", "Extensions should contain '.kt'")
	assert.Contains(t, cfg.Extensions, ".kts", "Extensions should contain '.kts'")
	assert.Equal(t, "kotlin", cfg.TreeSitter.Grammar, "Grammar should be 'kotlin'")

	// Kotlin should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Kotlin should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "object_declaration")

	// Kotlin should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Kotlin should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_declaration")
}

func TestLanguageConfig_PHP(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "php.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "php", cfg)

	// Validate specific expected values
	assert.Equal(t, "php", cfg.Language, "Language should be 'php'")
	assert.Equal(t, "PHP", cfg.DisplayName, "DisplayName should be 'PHP'")
	assert.Contains(t, cfg.Extensions, ".php", "Extensions should contain '.php'")
	assert.Equal(t, "php", cfg.TreeSitter.Grammar, "Grammar should be 'php'")

	// PHP has many extensions
	assert.GreaterOrEqual(t, len(cfg.Extensions), 2, "PHP should have multiple extensions")

	// PHP should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "PHP should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "trait_declaration")

	// PHP should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "PHP should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_definition")
	assert.Contains(t, cfg.ChunkMappings.Method, "method_declaration")
}

func TestLanguageConfig_Python(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "python.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "python", cfg)

	// Validate specific expected values
	assert.Equal(t, "python", cfg.Language, "Language should be 'python'")
	assert.Equal(t, "Python", cfg.DisplayName, "DisplayName should be 'Python'")
	assert.Contains(t, cfg.Extensions, ".py", "Extensions should contain '.py'")
	assert.Contains(t, cfg.Extensions, ".pyi", "Extensions should contain '.pyi'")
	assert.Equal(t, "python", cfg.TreeSitter.Grammar, "Grammar should be 'python'")

	// Python should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Python should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_definition")

	// Python should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Python should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_definition")

	// Python uses first_child for docstrings
	assert.Equal(t, "first_child", cfg.Extraction.DocCommentPosition,
		"Python should use first_child for doc comment position")
}

func TestLanguageConfig_Rust(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "rust.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "rust", cfg)

	// Validate specific expected values
	assert.Equal(t, "rust", cfg.Language, "Language should be 'rust'")
	assert.Equal(t, "Rust", cfg.DisplayName, "DisplayName should be 'Rust'")
	assert.Contains(t, cfg.Extensions, ".rs", "Extensions should contain '.rs'")
	assert.Equal(t, "rust", cfg.TreeSitter.Grammar, "Grammar should be 'rust'")

	// Rust should have class mappings for struct, enum, trait, impl
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Rust should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "struct_item")
	assert.Contains(t, cfg.ChunkMappings.Class, "enum_item")
	assert.Contains(t, cfg.ChunkMappings.Class, "trait_item")
	assert.Contains(t, cfg.ChunkMappings.Class, "impl_item")

	// Rust should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Rust should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_item")
}

func TestLanguageConfig_Solidity(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "solidity.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "solidity", cfg)

	// Validate specific expected values
	assert.Equal(t, "solidity", cfg.Language, "Language should be 'solidity'")
	assert.Equal(t, "Solidity", cfg.DisplayName, "DisplayName should be 'Solidity'")
	assert.Contains(t, cfg.Extensions, ".sol", "Extensions should contain '.sol'")
	assert.Equal(t, "solidity", cfg.TreeSitter.Grammar, "Grammar should be 'solidity'")

	// Solidity should have class mappings for contract, interface, library
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Solidity should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "contract_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "interface_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "library_declaration")

	// Solidity should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Solidity should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_definition")
	assert.Contains(t, cfg.ChunkMappings.Method, "modifier_definition")
}

func TestLanguageConfig_Swift(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "swift.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "swift", cfg)

	// Validate specific expected values
	assert.Equal(t, "swift", cfg.Language, "Language should be 'swift'")
	assert.Equal(t, "Swift", cfg.DisplayName, "DisplayName should be 'Swift'")
	assert.Contains(t, cfg.Extensions, ".swift", "Extensions should contain '.swift'")
	assert.Equal(t, "swift", cfg.TreeSitter.Grammar, "Grammar should be 'swift'")

	// Swift should have class mappings for class, struct, protocol, enum, extension, actor
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "Swift should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "struct_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "protocol_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "actor_declaration")

	// Swift should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "Swift should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_declaration")
	assert.Contains(t, cfg.ChunkMappings.Method, "init_declaration")
}

func TestLanguageConfig_TypeScript(t *testing.T) {
	cfg := loadTestLanguageConfig(t, "typescript.yaml")

	// Validate common requirements
	validateLanguageConfig(t, "typescript", cfg)

	// Validate specific expected values
	assert.Equal(t, "typescript", cfg.Language, "Language should be 'typescript'")
	assert.Equal(t, "TypeScript", cfg.DisplayName, "DisplayName should be 'TypeScript'")
	assert.Contains(t, cfg.Extensions, ".ts", "Extensions should contain '.ts'")
	assert.Contains(t, cfg.Extensions, ".tsx", "Extensions should contain '.tsx'")
	assert.Contains(t, cfg.Extensions, ".mts", "Extensions should contain '.mts'")
	assert.Contains(t, cfg.Extensions, ".cts", "Extensions should contain '.cts'")
	assert.Equal(t, "typescript", cfg.TreeSitter.Grammar, "Grammar should be 'typescript'")

	// TypeScript should have class mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Class, "TypeScript should have class mappings")
	assert.Contains(t, cfg.ChunkMappings.Class, "class_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "interface_declaration")
	assert.Contains(t, cfg.ChunkMappings.Class, "type_alias_declaration")

	// TypeScript should have method mappings
	assert.NotEmpty(t, cfg.ChunkMappings.Method, "TypeScript should have method mappings")
	assert.Contains(t, cfg.ChunkMappings.Method, "function_declaration")
	assert.Contains(t, cfg.ChunkMappings.Method, "arrow_function")
}

// =============================================================================
// Aggregate Tests
// =============================================================================

// expectedLanguageConfigs defines all 14 language configs that should be shipped.
var expectedLanguageConfigs = []struct {
	filename string
	language string
}{
	{"csharp.yaml", "csharp"},
	{"dart.yaml", "dart"},
	{"elixir.yaml", "elixir"},
	{"go.yaml", "go"},
	{"java.yaml", "java"},
	{"javascript.yaml", "javascript"},
	{"kotlin.yaml", "kotlin"},
	{"markdown.yaml", "markdown"},
	{"php.yaml", "php"},
	{"python.yaml", "python"},
	{"rust.yaml", "rust"},
	{"solidity.yaml", "solidity"},
	{"swift.yaml", "swift"},
	{"typescript.yaml", "typescript"},
}

func TestAllLanguageConfigs_Load(t *testing.T) {
	// All 14 configs should load without error
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load without error", expected.filename)
			require.NotNil(t, cfg, "Config %s should not be nil", expected.filename)

			// Verify the language matches expected
			assert.Equal(t, expected.language, cfg.Language,
				"Config %s should have language '%s'", expected.filename, expected.language)
		})
	}
}

func TestAllLanguageConfigs_UniqueLanguageIDs(t *testing.T) {
	// No duplicate language identifiers should exist
	languagesDir := getLanguagesDir(t)
	seenLanguages := make(map[string]string) // language -> filename

	for _, expected := range expectedLanguageConfigs {
		configPath := filepath.Join(languagesDir, expected.filename)

		cfg, err := LoadLanguageConfig(configPath)
		require.NoError(t, err, "Config %s should load", expected.filename)

		if existingFile, exists := seenLanguages[cfg.Language]; exists {
			t.Errorf("Duplicate language identifier '%s' found in both %s and %s",
				cfg.Language, existingFile, expected.filename)
		}
		seenLanguages[cfg.Language] = expected.filename
	}

	// Verify we checked all 14 configs
	assert.Len(t, seenLanguages, 14, "Should have 14 unique language identifiers")
}

func TestAllLanguageConfigs_UniqueExtensions(t *testing.T) {
	// No overlapping extensions between languages
	languagesDir := getLanguagesDir(t)
	seenExtensions := make(map[string]string) // extension -> language

	for _, expected := range expectedLanguageConfigs {
		configPath := filepath.Join(languagesDir, expected.filename)

		cfg, err := LoadLanguageConfig(configPath)
		require.NoError(t, err, "Config %s should load", expected.filename)

		for _, ext := range cfg.Extensions {
			// Normalize extension to lowercase for comparison
			normalizedExt := strings.ToLower(ext)

			if existingLang, exists := seenExtensions[normalizedExt]; exists {
				t.Errorf("Duplicate extension '%s' found in both %s and %s",
					ext, existingLang, cfg.Language)
			}
			seenExtensions[normalizedExt] = cfg.Language
		}
	}

	// Verify we have extensions
	assert.NotEmpty(t, seenExtensions, "Should have collected extensions")
}

func TestAllLanguageConfigs_HaveChunkMappings(t *testing.T) {
	// All configs should have at least class or method mappings
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load", expected.filename)

			hasClassMapping := len(cfg.ChunkMappings.Class) > 0
			hasMethodMapping := len(cfg.ChunkMappings.Method) > 0

			assert.True(t, hasClassMapping || hasMethodMapping,
				"Config %s should have at least one chunk mapping (class: %d, method: %d)",
				expected.filename, len(cfg.ChunkMappings.Class), len(cfg.ChunkMappings.Method))
		})
	}
}

// =============================================================================
// Additional Validation Tests
// =============================================================================

func TestAllLanguageConfigs_ExtensionsStartWithDot(t *testing.T) {
	// All extensions in all configs should start with '.'
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load", expected.filename)

			for _, ext := range cfg.Extensions {
				assert.True(t, strings.HasPrefix(ext, "."),
					"Extension '%s' in %s should start with '.'", ext, expected.filename)
			}
		})
	}
}

func TestAllLanguageConfigs_HaveDisplayName(t *testing.T) {
	// All configs should have a non-empty display name
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load", expected.filename)

			assert.NotEmpty(t, cfg.DisplayName,
				"Config %s should have a non-empty display_name", expected.filename)
		})
	}
}

func TestAllLanguageConfigs_HaveGrammar(t *testing.T) {
	// All configs should have a non-empty tree_sitter.grammar
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load", expected.filename)

			assert.NotEmpty(t, cfg.TreeSitter.Grammar,
				"Config %s should have a non-empty tree_sitter.grammar", expected.filename)
		})
	}
}

func TestAllLanguageConfigs_Count(t *testing.T) {
	// Verify exactly 13 language configs exist in the languages directory
	languagesDir := getLanguagesDir(t)

	entries, err := os.ReadDir(languagesDir)
	require.NoError(t, err, "Should be able to read languages directory")

	yamlCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			yamlCount++
		}
	}

	assert.Equal(t, 14, yamlCount,
		"Languages directory should contain exactly 14 YAML config files")
}

func TestAllLanguageConfigs_FilenameMatchesLanguage(t *testing.T) {
	// Each config filename (minus .yaml) should match its language field
	languagesDir := getLanguagesDir(t)

	for _, expected := range expectedLanguageConfigs {
		t.Run(expected.filename, func(t *testing.T) {
			configPath := filepath.Join(languagesDir, expected.filename)

			cfg, err := LoadLanguageConfig(configPath)
			require.NoError(t, err, "Config %s should load", expected.filename)

			// Extract expected language from filename (remove .yaml)
			expectedLang := strings.TrimSuffix(expected.filename, ".yaml")

			assert.Equal(t, expectedLang, cfg.Language,
				"Config filename '%s' should match language field '%s'",
				expected.filename, cfg.Language)
		})
	}
}
