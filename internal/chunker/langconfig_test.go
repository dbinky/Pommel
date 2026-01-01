package chunker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLanguageConfig_ValidYAML(t *testing.T) {
	yaml := `
language: go
display_name: Go
extensions:
  - .go
tree_sitter:
  grammar: go
chunk_mappings:
  class:
    - type_spec
  method:
    - function_declaration
    - method_declaration
extraction:
  name_field: name
  doc_comments:
    - comment
  doc_comment_position: preceding_siblings
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	assert.Equal(t, "go", config.Language)
	assert.Equal(t, "Go", config.DisplayName)
	assert.Equal(t, []string{".go"}, config.Extensions)
	assert.Equal(t, "go", config.TreeSitter.Grammar)
	assert.Equal(t, []string{"type_spec"}, config.ChunkMappings.Class)
	assert.Equal(t, []string{"function_declaration", "method_declaration"}, config.ChunkMappings.Method)
	assert.Equal(t, "name", config.Extraction.NameField)
	assert.Equal(t, []string{"comment"}, config.Extraction.DocComments)
	assert.Equal(t, "preceding_siblings", config.Extraction.DocCommentPosition)
}

func TestParseLanguageConfig_AllFields(t *testing.T) {
	yaml := `
language: python
display_name: Python
extensions:
  - .py
  - .pyw
tree_sitter:
  grammar: python
chunk_mappings:
  class:
    - class_definition
  method:
    - function_definition
  block:
    - if_statement
    - for_statement
    - while_statement
extraction:
  name_field: name
  doc_comments:
    - string
    - comment
  doc_comment_position: first_child
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	assert.Equal(t, "python", config.Language)
	assert.Equal(t, "Python", config.DisplayName)
	assert.Equal(t, []string{".py", ".pyw"}, config.Extensions)
	assert.Equal(t, "python", config.TreeSitter.Grammar)
	assert.Equal(t, []string{"class_definition"}, config.ChunkMappings.Class)
	assert.Equal(t, []string{"function_definition"}, config.ChunkMappings.Method)
	assert.Equal(t, []string{"if_statement", "for_statement", "while_statement"}, config.ChunkMappings.Block)
	assert.Equal(t, "name", config.Extraction.NameField)
	assert.Equal(t, []string{"string", "comment"}, config.Extraction.DocComments)
	assert.Equal(t, "first_child", config.Extraction.DocCommentPosition)
}

func TestParseLanguageConfig_MinimalConfig(t *testing.T) {
	yaml := `
language: rust
extensions:
  - .rs
tree_sitter:
  grammar: rust
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	assert.Equal(t, "rust", config.Language)
	assert.Equal(t, "", config.DisplayName) // Optional field
	assert.Equal(t, []string{".rs"}, config.Extensions)
	assert.Equal(t, "rust", config.TreeSitter.Grammar)
	assert.Empty(t, config.ChunkMappings.Class)
	assert.Empty(t, config.ChunkMappings.Method)
	assert.Empty(t, config.ChunkMappings.Block)
	assert.Equal(t, "", config.Extraction.NameField)
}

func TestParseLanguageConfig_InvalidYAML(t *testing.T) {
	invalidYAML := `
language: go
extensions
  - .go
`

	_, err := ParseLanguageConfig([]byte(invalidYAML))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestParseLanguageConfig_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "missing language",
			yaml: `
extensions:
  - .go
tree_sitter:
  grammar: go
`,
			expectedErr: "language",
		},
		{
			name: "missing extensions",
			yaml: `
language: go
tree_sitter:
  grammar: go
`,
			expectedErr: "extensions",
		},
		{
			name: "missing grammar",
			yaml: `
language: go
extensions:
  - .go
`,
			expectedErr: "tree_sitter.grammar",
		},
		{
			name: "missing all required",
			yaml: `
display_name: Go
chunk_mappings:
  class:
    - type_spec
`,
			expectedErr: "language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseLanguageConfig([]byte(tt.yaml))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestLoadLanguageConfig_FromFile(t *testing.T) {
	// Create a temporary directory and file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "go.yaml")

	yaml := `
language: go
display_name: Go
extensions:
  - .go
tree_sitter:
  grammar: go
chunk_mappings:
  method:
    - function_declaration
`

	err := os.WriteFile(configPath, []byte(yaml), 0644)
	require.NoError(t, err)

	config, err := LoadLanguageConfig(configPath)

	require.NoError(t, err)
	assert.Equal(t, "go", config.Language)
	assert.Equal(t, "Go", config.DisplayName)
	assert.Equal(t, []string{".go"}, config.Extensions)
}

func TestLoadLanguageConfig_FileNotFound(t *testing.T) {
	_, err := LoadLanguageConfig("/nonexistent/path/config.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadAllLanguageConfigs_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go config
	goYAML := `
language: go
extensions:
  - .go
tree_sitter:
  grammar: go
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.yaml"), []byte(goYAML), 0644)
	require.NoError(t, err)

	// Create Python config
	pyYAML := `
language: python
extensions:
  - .py
tree_sitter:
  grammar: python
`
	err = os.WriteFile(filepath.Join(tmpDir, "python.yml"), []byte(pyYAML), 0644)
	require.NoError(t, err)

	configs, errors := LoadAllLanguageConfigs(tmpDir)

	assert.Len(t, errors, 0)
	assert.Len(t, configs, 2)

	// Check that both configs were loaded (order not guaranteed)
	languages := make(map[string]bool)
	for _, c := range configs {
		languages[c.Language] = true
	}
	assert.True(t, languages["go"])
	assert.True(t, languages["python"])
}

func TestLoadAllLanguageConfigs_SkipsMalformed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid config
	validYAML := `
language: go
extensions:
  - .go
tree_sitter:
  grammar: go
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.yaml"), []byte(validYAML), 0644)
	require.NoError(t, err)

	// Create malformed config (missing required fields)
	malformedYAML := `
language: python
# Missing extensions and grammar
`
	err = os.WriteFile(filepath.Join(tmpDir, "python.yaml"), []byte(malformedYAML), 0644)
	require.NoError(t, err)

	// Create invalid YAML config
	invalidYAML := `
language: rust
extensions
  - .rs
`
	err = os.WriteFile(filepath.Join(tmpDir, "rust.yaml"), []byte(invalidYAML), 0644)
	require.NoError(t, err)

	configs, errors := LoadAllLanguageConfigs(tmpDir)

	// Should have loaded the valid config
	assert.Len(t, configs, 1)
	assert.Equal(t, "go", configs[0].Language)

	// Should have 2 errors (malformed + invalid)
	assert.Len(t, errors, 2)
	for _, err := range errors {
		assert.Contains(t, err.Error(), "skipping")
	}
}

func TestLoadAllLanguageConfigs_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	configs, errors := LoadAllLanguageConfigs(tmpDir)

	assert.Len(t, configs, 0)
	assert.Len(t, errors, 0)
}

func TestLoadAllLanguageConfigs_NonexistentDirectory(t *testing.T) {
	configs, errors := LoadAllLanguageConfigs("/nonexistent/directory")

	assert.Nil(t, configs)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "failed to read directory")
}

func TestLoadAllLanguageConfigs_SkipsSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid config in the main directory
	validYAML := `
language: go
extensions:
  - .go
tree_sitter:
  grammar: go
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.yaml"), []byte(validYAML), 0644)
	require.NoError(t, err)

	// Create a subdirectory with a config
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "python.yaml"), []byte(validYAML), 0644)
	require.NoError(t, err)

	configs, errors := LoadAllLanguageConfigs(tmpDir)

	assert.Len(t, errors, 0)
	assert.Len(t, configs, 1) // Only the main directory config
	assert.Equal(t, "go", configs[0].Language)
}

func TestLoadAllLanguageConfigs_SkipsNonYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid config
	validYAML := `
language: go
extensions:
  - .go
tree_sitter:
  grammar: go
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.yaml"), []byte(validYAML), 0644)
	require.NoError(t, err)

	// Create non-YAML files
	err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	configs, errors := LoadAllLanguageConfigs(tmpDir)

	assert.Len(t, errors, 0)
	assert.Len(t, configs, 1)
	assert.Equal(t, "go", configs[0].Language)
}

func TestParseLanguageConfig_ExtraFields(t *testing.T) {
	// YAML with extra/unknown fields should be ignored
	yaml := `
language: go
display_name: Go
extensions:
  - .go
tree_sitter:
  grammar: go
  version: 0.20.0
  unknown_field: ignored
chunk_mappings:
  class:
    - type_spec
  extra_level:
    - some_node
custom_field: ignored
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	assert.Equal(t, "go", config.Language)
	assert.Equal(t, "Go", config.DisplayName)
	assert.Equal(t, []string{".go"}, config.Extensions)
	assert.Equal(t, "go", config.TreeSitter.Grammar)
	assert.Equal(t, []string{"type_spec"}, config.ChunkMappings.Class)
}

func TestParseLanguageConfig_CaseInsensitiveExtensions(t *testing.T) {
	yaml := `
language: go
extensions:
  - .GO
  - .Go
  - .go
tree_sitter:
  grammar: go
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	// All extensions should be normalized to lowercase
	assert.Equal(t, []string{".go", ".go", ".go"}, config.Extensions)
}

func TestLanguageConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      LanguageConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: LanguageConfig{
				Language:   "go",
				Extensions: []string{".go"},
				TreeSitter: TreeSitterConfig{Grammar: "go"},
			},
			expectError: false,
		},
		{
			name: "valid with doc comment position",
			config: LanguageConfig{
				Language:   "python",
				Extensions: []string{".py"},
				TreeSitter: TreeSitterConfig{Grammar: "python"},
				Extraction: ExtractionConfig{DocCommentPosition: "first_child"},
			},
			expectError: false,
		},
		{
			name: "invalid doc comment position",
			config: LanguageConfig{
				Language:   "go",
				Extensions: []string{".go"},
				TreeSitter: TreeSitterConfig{Grammar: "go"},
				Extraction: ExtractionConfig{DocCommentPosition: "invalid_position"},
			},
			expectError: true,
			errorMsg:    "invalid doc_comment_position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLanguageConfig_HasMappings(t *testing.T) {
	config := LanguageConfig{
		ChunkMappings: ChunkMappings{
			Class:  []string{"type_spec"},
			Method: []string{"function_declaration"},
		},
	}

	assert.True(t, config.HasClassMappings())
	assert.True(t, config.HasMethodMappings())
	assert.False(t, config.HasBlockMappings())

	// Empty config
	emptyConfig := LanguageConfig{}
	assert.False(t, emptyConfig.HasClassMappings())
	assert.False(t, emptyConfig.HasMethodMappings())
	assert.False(t, emptyConfig.HasBlockMappings())
}

func TestLanguageConfig_IsNodeType(t *testing.T) {
	config := LanguageConfig{
		ChunkMappings: ChunkMappings{
			Class:  []string{"type_spec", "interface_type"},
			Method: []string{"function_declaration", "method_declaration"},
			Block:  []string{"if_statement"},
		},
	}

	// Class types
	assert.True(t, config.IsClassNodeType("type_spec"))
	assert.True(t, config.IsClassNodeType("interface_type"))
	assert.False(t, config.IsClassNodeType("function_declaration"))

	// Method types
	assert.True(t, config.IsMethodNodeType("function_declaration"))
	assert.True(t, config.IsMethodNodeType("method_declaration"))
	assert.False(t, config.IsMethodNodeType("type_spec"))

	// Block types
	assert.True(t, config.IsBlockNodeType("if_statement"))
	assert.False(t, config.IsBlockNodeType("for_statement"))
}

func TestLanguageConfig_MatchesExtension(t *testing.T) {
	config := LanguageConfig{
		Extensions: []string{".go", ".golang"},
	}

	// Matching extensions
	assert.True(t, config.MatchesExtension(".go"))
	assert.True(t, config.MatchesExtension(".golang"))

	// Case insensitive
	assert.True(t, config.MatchesExtension(".GO"))
	assert.True(t, config.MatchesExtension(".Go"))
	assert.True(t, config.MatchesExtension(".GOLANG"))

	// Non-matching extensions
	assert.False(t, config.MatchesExtension(".py"))
	assert.False(t, config.MatchesExtension(".java"))
	assert.False(t, config.MatchesExtension(""))
}

func TestValidDocCommentPositions(t *testing.T) {
	validPositions := []string{
		"preceding_siblings",
		"first_child",
		"parent_first_child",
		"following_siblings",
	}

	for _, pos := range validPositions {
		yaml := `
language: test
extensions:
  - .test
tree_sitter:
  grammar: test
extraction:
  doc_comment_position: ` + pos

		config, err := ParseLanguageConfig([]byte(yaml))
		require.NoError(t, err, "position %s should be valid", pos)
		assert.Equal(t, pos, config.Extraction.DocCommentPosition)
	}
}

func TestParseLanguageConfig_ComplexRealWorldExample(t *testing.T) {
	// Test a complex, real-world config for Java
	yaml := `
language: java
display_name: Java
extensions:
  - .java
tree_sitter:
  grammar: java
chunk_mappings:
  class:
    - class_declaration
    - interface_declaration
    - enum_declaration
    - annotation_type_declaration
    - record_declaration
  method:
    - method_declaration
    - constructor_declaration
  block:
    - if_statement
    - for_statement
    - enhanced_for_statement
    - while_statement
    - do_statement
    - try_statement
    - switch_expression
extraction:
  name_field: name
  doc_comments:
    - block_comment
    - line_comment
  doc_comment_position: preceding_siblings
`

	config, err := ParseLanguageConfig([]byte(yaml))

	require.NoError(t, err)
	assert.Equal(t, "java", config.Language)
	assert.Equal(t, "Java", config.DisplayName)
	assert.Len(t, config.ChunkMappings.Class, 5)
	assert.Len(t, config.ChunkMappings.Method, 2)
	assert.Len(t, config.ChunkMappings.Block, 7)
	assert.Contains(t, config.ChunkMappings.Class, "record_declaration")
	assert.Contains(t, config.ChunkMappings.Block, "switch_expression")
}

func TestParseLanguageConfig_EmptyExtensionsArray(t *testing.T) {
	yaml := `
language: go
extensions: []
tree_sitter:
  grammar: go
`

	_, err := ParseLanguageConfig([]byte(yaml))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "extensions")
}
