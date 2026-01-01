package chunker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LanguageConfig represents the configuration for a programming language's
// chunk extraction rules. This allows language support to be defined
// declaratively via YAML configuration files rather than hard-coded.
type LanguageConfig struct {
	// Language is the unique identifier for this language (e.g., "go", "python")
	Language string `yaml:"language"`

	// DisplayName is the human-readable name (e.g., "Go", "Python")
	DisplayName string `yaml:"display_name"`

	// Extensions lists the file extensions for this language (e.g., [".go", ".py"])
	Extensions []string `yaml:"extensions"`

	// TreeSitter contains tree-sitter grammar configuration
	TreeSitter TreeSitterConfig `yaml:"tree_sitter"`

	// ChunkMappings maps chunk levels to AST node types
	ChunkMappings ChunkMappings `yaml:"chunk_mappings"`

	// Extraction contains rules for extracting metadata from AST nodes
	Extraction ExtractionConfig `yaml:"extraction"`
}

// TreeSitterConfig contains tree-sitter specific configuration
type TreeSitterConfig struct {
	// Grammar is the tree-sitter grammar name (e.g., "go", "python")
	Grammar string `yaml:"grammar"`
}

// ChunkMappings maps chunk levels to their corresponding AST node types
type ChunkMappings struct {
	// Class contains node types that represent class-level constructs
	// (e.g., struct_type, class_declaration, interface_type)
	Class []string `yaml:"class"`

	// Method contains node types that represent method-level constructs
	// (e.g., function_declaration, method_declaration)
	Method []string `yaml:"method"`

	// Block contains node types that represent block-level constructs
	// (e.g., if_statement, for_statement) - optional
	Block []string `yaml:"block"`
}

// ExtractionConfig contains rules for extracting metadata from AST nodes
type ExtractionConfig struct {
	// NameField is the AST field name used to extract identifiers (e.g., "name")
	NameField string `yaml:"name_field"`

	// DocComments lists node types that represent documentation comments
	DocComments []string `yaml:"doc_comments"`

	// DocCommentPosition indicates where doc comments appear relative to the node
	// Valid values: "preceding_siblings", "first_child", "parent_first_child"
	DocCommentPosition string `yaml:"doc_comment_position"`
}

// ParseLanguageConfig parses YAML data into a LanguageConfig struct.
// It validates required fields and normalizes extensions to lowercase.
func ParseLanguageConfig(data []byte) (*LanguageConfig, error) {
	var config LanguageConfig

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Normalize extensions to lowercase
	for i, ext := range config.Extensions {
		config.Extensions[i] = strings.ToLower(ext)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadLanguageConfig reads and parses a language configuration from a file.
func LoadLanguageConfig(path string) (*LanguageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config, err := ParseLanguageConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return config, nil
}

// LoadAllLanguageConfigs loads all language configurations from a directory.
// It reads all .yaml and .yml files in the directory.
// Returns successfully loaded configs and any errors encountered (allowing
// partial success - valid configs are returned even if some fail).
func LoadAllLanguageConfigs(dir string) ([]*LanguageConfig, []error) {
	var configs []*LanguageConfig
	var errors []error

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read directory: %w", err)}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		config, err := LoadLanguageConfig(path)
		if err != nil {
			errors = append(errors, fmt.Errorf("skipping %s: %w", name, err))
			continue
		}

		configs = append(configs, config)
	}

	return configs, errors
}

// Validate checks that all required fields are present and valid.
func (c *LanguageConfig) Validate() error {
	var missing []string

	if c.Language == "" {
		missing = append(missing, "language")
	}

	if len(c.Extensions) == 0 {
		missing = append(missing, "extensions")
	}

	if c.TreeSitter.Grammar == "" {
		missing = append(missing, "tree_sitter.grammar")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	// Validate doc_comment_position if specified
	if c.Extraction.DocCommentPosition != "" {
		validPositions := map[string]bool{
			"preceding_siblings": true,
			"first_child":        true,
			"parent_first_child": true,
			"following_siblings": true,
		}
		if !validPositions[c.Extraction.DocCommentPosition] {
			return fmt.Errorf("invalid doc_comment_position: %s (valid values: preceding_siblings, first_child, parent_first_child, following_siblings)", c.Extraction.DocCommentPosition)
		}
	}

	return nil
}

// HasClassMappings returns true if class-level node types are configured.
func (c *LanguageConfig) HasClassMappings() bool {
	return len(c.ChunkMappings.Class) > 0
}

// HasMethodMappings returns true if method-level node types are configured.
func (c *LanguageConfig) HasMethodMappings() bool {
	return len(c.ChunkMappings.Method) > 0
}

// HasBlockMappings returns true if block-level node types are configured.
func (c *LanguageConfig) HasBlockMappings() bool {
	return len(c.ChunkMappings.Block) > 0
}

// IsClassNodeType returns true if the given node type is a class-level construct.
func (c *LanguageConfig) IsClassNodeType(nodeType string) bool {
	for _, t := range c.ChunkMappings.Class {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsMethodNodeType returns true if the given node type is a method-level construct.
func (c *LanguageConfig) IsMethodNodeType(nodeType string) bool {
	for _, t := range c.ChunkMappings.Method {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsBlockNodeType returns true if the given node type is a block-level construct.
func (c *LanguageConfig) IsBlockNodeType(nodeType string) bool {
	for _, t := range c.ChunkMappings.Block {
		if t == nodeType {
			return true
		}
	}
	return false
}

// MatchesExtension returns true if the given file extension matches this language.
// The comparison is case-insensitive.
func (c *LanguageConfig) MatchesExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range c.Extensions {
		if e == ext {
			return true
		}
	}
	return false
}
