package chunker

import (
	"context"
	"fmt"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
	sitter "github.com/smacker/go-tree-sitter"
)

// GenericChunker extracts chunks from source files using a config-driven approach.
// It uses LanguageConfig to determine which AST node types map to which chunk levels,
// allowing new languages to be supported declaratively via YAML configuration.
type GenericChunker struct {
	config   *LanguageConfig
	parser   *Parser
	language Language
}

// NewGenericChunker creates a new config-driven chunker for the specified language.
// It validates that the parser supports the language's grammar and returns an error
// if the configuration is invalid.
func NewGenericChunker(parser *Parser, config *LanguageConfig) (*GenericChunker, error) {
	if parser == nil {
		return nil, fmt.Errorf("parser is required")
	}
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Determine the Language enum from the config
	lang := Language(config.TreeSitter.Grammar)

	// Verify the parser supports this language
	if !parser.IsSupported(lang) {
		return nil, fmt.Errorf("parser does not support language: %s", config.TreeSitter.Grammar)
	}

	return &GenericChunker{
		config:   config,
		parser:   parser,
		language: lang,
	}, nil
}

// Chunk extracts chunks from a source file using the language configuration.
func (c *GenericChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	// Check for context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &models.ChunkResult{
		File:   file,
		Chunks: make([]*models.Chunk, 0),
	}

	// Handle empty files - return empty result with no chunks
	if len(file.Content) == 0 {
		return result, nil
	}

	// Parse the file using the configured language
	tree, err := c.parser.Parse(ctx, c.language, file.Content)
	if err != nil {
		return nil, err
	}

	// Add file-level chunk first
	fileChunk := c.createFileChunk(file)
	fileChunk.SetHashes()
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST looking for matching node types
	c.walkNode(ctx, tree.RootNode(), file, fileChunk.ID, "", result)

	// Set hashes for all chunks that don't have them yet
	for _, chunk := range result.Chunks {
		if chunk.ID == "" {
			chunk.SetHashes()
		}
	}

	return result, nil
}

// Language returns the language this chunker handles.
func (c *GenericChunker) Language() Language {
	return c.language
}

// isClassNode returns true if the node type represents a class-level construct.
func (c *GenericChunker) isClassNode(nodeType string) bool {
	return c.config.IsClassNodeType(nodeType)
}

// isMethodNode returns true if the node type represents a method-level construct.
func (c *GenericChunker) isMethodNode(nodeType string) bool {
	return c.config.IsMethodNodeType(nodeType)
}

// createFileChunk creates a file-level chunk for the entire source file.
func (c *GenericChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
	lines := strings.Split(string(file.Content), "\n")
	endLine := len(lines)
	if endLine == 0 {
		endLine = 1
	}
	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      endLine,
		Level:        models.ChunkLevelFile,
		Language:     c.config.Language,
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

// walkNode recursively walks the AST, creating chunks for matching node types.
// parentID is the ID of the current parent chunk (file or class).
// currentClassID is the ID of the current class scope (for method parent linking).
func (c *GenericChunker) walkNode(ctx context.Context, node *sitter.Node, file *models.SourceFile, fileID string, currentClassID string, result *models.ChunkResult) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	nodeType := node.Type()

	// Determine parent ID based on current context
	parentID := fileID
	if currentClassID != "" {
		parentID = currentClassID
	}

	// Check if this is a class-level node
	if c.isClassNode(nodeType) {
		chunk := c.extractChunk(node, file, fileID, models.ChunkLevelClass)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Recurse into children with this class as the current context
			c.walkChildren(ctx, node, file, fileID, chunk.ID, result)
			return
		}
	}

	// Check if this is a method-level node
	if c.isMethodNode(nodeType) {
		chunk := c.extractChunk(node, file, parentID, models.ChunkLevelMethod)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
		}
		// Don't recurse into method bodies for nested functions (keep it simple for now)
		return
	}

	// Handle Go's special case: type_declaration wraps type_spec which contains struct/interface
	if nodeType == "type_declaration" {
		c.handleGoTypeDeclaration(ctx, node, file, fileID, currentClassID, result)
		return
	}

	// Handle Python's decorated definitions
	if nodeType == "decorated_definition" {
		c.handleDecoratedDefinition(ctx, node, file, fileID, currentClassID, result)
		return
	}

	// Recurse into children
	c.walkChildren(ctx, node, file, fileID, currentClassID, result)
}

// walkChildren recursively processes all child nodes.
func (c *GenericChunker) walkChildren(ctx context.Context, node *sitter.Node, file *models.SourceFile, fileID string, currentClassID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(ctx, child, file, fileID, currentClassID, result)
	}
}

// extractChunk creates a chunk from an AST node.
func (c *GenericChunker) extractChunk(node *sitter.Node, file *models.SourceFile, parentID string, level models.ChunkLevel) *models.Chunk {
	name := c.extractName(node, file.Content)
	if name == "" {
		// Try to get name from type_spec child (Go structs/interfaces)
		name = c.extractNameFromTypeSpec(node, file.Content)
	}
	if name == "" {
		return nil
	}

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	// Get first line as signature
	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        level,
		Language:     c.config.Language,
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// extractName extracts the identifier name from a node using the configured name_field.
func (c *GenericChunker) extractName(node *sitter.Node, source []byte) string {
	nameField := c.config.Extraction.NameField
	if nameField == "" {
		nameField = "name" // Default
	}

	nameNode := node.ChildByFieldName(nameField)
	if nameNode == nil {
		return ""
	}
	return nameNode.Content(source)
}

// extractNameFromTypeSpec extracts name from a Go type_spec node.
// This handles the case where Go wraps struct/interface in type_declaration -> type_spec.
func (c *GenericChunker) extractNameFromTypeSpec(node *sitter.Node, source []byte) string {
	// Look for type_spec child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(source)
			}
		}
	}
	return ""
}

// handleGoTypeDeclaration handles Go's type_declaration which wraps type_spec nodes.
func (c *GenericChunker) handleGoTypeDeclaration(ctx context.Context, node *sitter.Node, file *models.SourceFile, fileID string, currentClassID string, result *models.ChunkResult) {
	// Type declarations can contain multiple type_spec children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			// Check if the type is a struct or interface
			typeNode := child.ChildByFieldName("type")
			if typeNode != nil {
				typeKind := typeNode.Type()
				if c.isClassNode(typeKind) || c.isClassNode("type_spec") {
					chunk := c.extractTypeSpecChunk(child, file, fileID)
					if chunk != nil {
						chunk.SetHashes()
						result.Chunks = append(result.Chunks, chunk)
					}
				}
			}
		}
	}
}

// extractTypeSpecChunk extracts a chunk from a Go type_spec node.
func (c *GenericChunker) extractTypeSpecChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

	// Get the type definition (struct_type, interface_type, etc.)
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return nil
	}

	typeKind := typeNode.Type()

	// Only extract struct and interface types as class-level chunks
	if typeKind != "struct_type" && typeKind != "interface_type" {
		return nil
	}

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	content := node.Content(file.Content)

	// Get first line as signature
	lines := strings.Split(content, "\n")
	signature := strings.TrimSpace(lines[0])

	return &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass,
		Language:     c.config.Language,
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// handleDecoratedDefinition handles Python's decorated_definition which wraps classes/functions.
func (c *GenericChunker) handleDecoratedDefinition(ctx context.Context, node *sitter.Node, file *models.SourceFile, fileID string, currentClassID string, result *models.ChunkResult) {
	// Find the actual class/function definition inside the decorator
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()
		if c.isClassNode(childType) || c.isMethodNode(childType) {
			c.walkNode(ctx, child, file, fileID, currentClassID, result)
			return
		}
	}
}
