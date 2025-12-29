package chunker

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/pommel-dev/pommel/internal/models"
)

// JavaScriptChunker handles chunking for JavaScript, TypeScript, JSX, and TSX files.
type JavaScriptChunker struct {
	parser   *Parser
	language Language
}

// NewJavaScriptChunker creates a new JavaScriptChunker for the specified language variant.
func NewJavaScriptChunker(parser *Parser, lang Language) *JavaScriptChunker {
	return &JavaScriptChunker{
		parser:   parser,
		language: lang,
	}
}

// Language returns the language this chunker handles.
func (c *JavaScriptChunker) Language() Language {
	return c.language
}

// Chunk parses a JavaScript/TypeScript file and extracts semantic chunks.
func (c *JavaScriptChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &models.ChunkResult{
		File:   file,
		Chunks: make([]*models.Chunk, 0),
		Errors: []error{},
	}

	// Parse the source file
	tree, err := c.parser.Parse(ctx, c.language, file.Content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	// Create file-level chunk
	fileChunk := c.createFileChunk(file)
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST to find classes, interfaces, functions, etc.
	c.walkNode(rootNode, file, fileChunk.ID, result)

	return result, nil
}

// createFileChunk creates a chunk for the entire file.
func (c *JavaScriptChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
	content := string(file.Content)
	endLine := 1
	if len(content) > 0 {
		endLine = strings.Count(content, "\n") + 1
	}

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      endLine,
		Level:        models.ChunkLevelFile,
		Language:     file.Language,
		Content:      content,
		Name:         file.Path,
		Signature:    "",
		LastModified: file.LastModified,
	}
	chunk.SetHashes()
	return chunk
}

// walkNode recursively walks the AST to find chunking targets.
func (c *JavaScriptChunker) walkNode(node *sitter.Node, file *models.SourceFile, fileChunkID string, result *models.ChunkResult) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "class_declaration", "class":
		c.handleClassDeclaration(node, file, fileChunkID, result)
	case "interface_declaration":
		c.handleInterfaceDeclaration(node, file, fileChunkID, result)
	case "type_alias_declaration":
		c.handleTypeAliasDeclaration(node, file, fileChunkID, result)
	case "function_declaration", "generator_function_declaration":
		c.handleFunctionDeclaration(node, file, fileChunkID, result)
	case "lexical_declaration", "variable_declaration":
		c.handleVariableDeclaration(node, file, fileChunkID, result)
	case "export_statement":
		c.handleExportStatement(node, file, fileChunkID, result)
	default:
		// Recursively walk children
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			c.walkNode(child, file, fileChunkID, result)
		}
	}
}

// handleClassDeclaration processes a class declaration node.
func (c *JavaScriptChunker) handleClassDeclaration(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	name := c.getNodeName(node, file.Content)
	if name == "" {
		// Anonymous class - try harder to find name
		// For `export default class Name`, the name might be an identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "identifier" {
				name = child.Content(file.Content)
				break
			}
		}
	}
	if name == "" {
		// Still anonymous, skip
		return
	}

	chunk := c.createChunk(node, file, name, models.ChunkLevelClass, parentID)
	result.Chunks = append(result.Chunks, chunk)

	// Find and process methods within the class body
	c.processClassBody(node, file, chunk.ID, result)
}

// handleInterfaceDeclaration processes a TypeScript interface declaration.
func (c *JavaScriptChunker) handleInterfaceDeclaration(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	name := c.getNodeName(node, file.Content)
	if name == "" {
		return
	}

	chunk := c.createChunk(node, file, name, models.ChunkLevelClass, parentID)
	result.Chunks = append(result.Chunks, chunk)
}

// handleTypeAliasDeclaration processes a TypeScript type alias declaration.
func (c *JavaScriptChunker) handleTypeAliasDeclaration(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	name := c.getNodeName(node, file.Content)
	if name == "" {
		return
	}

	chunk := c.createChunk(node, file, name, models.ChunkLevelClass, parentID)
	result.Chunks = append(result.Chunks, chunk)
}

// handleFunctionDeclaration processes a function declaration node.
func (c *JavaScriptChunker) handleFunctionDeclaration(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	name := c.getNodeName(node, file.Content)
	if name == "" {
		return
	}

	chunk := c.createChunk(node, file, name, models.ChunkLevelMethod, parentID)
	result.Chunks = append(result.Chunks, chunk)
}

// handleVariableDeclaration processes variable declarations looking for arrow functions.
func (c *JavaScriptChunker) handleVariableDeclaration(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	// Look for variable_declarator children that contain arrow functions
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "variable_declarator" {
			c.handleVariableDeclarator(node, child, file, parentID, result)
		}
	}
}

// handleVariableDeclarator processes a single variable declarator.
func (c *JavaScriptChunker) handleVariableDeclarator(declNode, node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	var name string
	var hasArrowFunction bool

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "identifier":
			name = child.Content(file.Content)
		case "arrow_function":
			hasArrowFunction = true
		}
	}

	if name != "" && hasArrowFunction {
		// Use the parent lexical_declaration node for proper line range
		chunk := c.createChunk(declNode, file, name, models.ChunkLevelMethod, parentID)
		result.Chunks = append(result.Chunks, chunk)
	}
}

// handleExportStatement processes an export statement, walking into exported declarations.
func (c *JavaScriptChunker) handleExportStatement(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	// Walk into the exported declaration
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		childType := child.Type()

		switch childType {
		case "class_declaration", "class":
			c.handleClassDeclaration(child, file, parentID, result)
		case "interface_declaration":
			c.handleInterfaceDeclaration(child, file, parentID, result)
		case "type_alias_declaration":
			c.handleTypeAliasDeclaration(child, file, parentID, result)
		case "function_declaration", "generator_function_declaration":
			c.handleFunctionDeclaration(child, file, parentID, result)
		case "lexical_declaration", "variable_declaration":
			c.handleVariableDeclaration(child, file, parentID, result)
		}
	}
}

// processClassBody finds and processes methods within a class body.
func (c *JavaScriptChunker) processClassBody(classNode *sitter.Node, file *models.SourceFile, classChunkID string, result *models.ChunkResult) {
	// Find the class_body node
	var classBody *sitter.Node
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(i)
		if child.Type() == "class_body" {
			classBody = child
			break
		}
	}

	if classBody == nil {
		return
	}

	// Process each member of the class body
	for i := 0; i < int(classBody.NamedChildCount()); i++ {
		member := classBody.NamedChild(i)
		memberType := member.Type()

		switch memberType {
		case "method_definition":
			c.handleMethodDefinition(member, file, classChunkID, result)
		case "field_definition", "public_field_definition":
			// Field definitions with arrow functions or nested classes
			c.handleFieldDefinition(member, file, classChunkID, result)
		}
	}
}

// handleMethodDefinition processes a method definition within a class.
func (c *JavaScriptChunker) handleMethodDefinition(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	name := c.getMethodName(node, file.Content)
	if name == "" {
		return
	}

	chunk := c.createChunk(node, file, name, models.ChunkLevelMethod, parentID)
	result.Chunks = append(result.Chunks, chunk)
}

// handleFieldDefinition processes a field definition looking for arrow function assignments or nested classes.
func (c *JavaScriptChunker) handleFieldDefinition(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	var name string
	var hasArrowFunction bool
	var hasClassExpression bool

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "property_identifier":
			name = child.Content(file.Content)
		case "arrow_function":
			hasArrowFunction = true
		case "class":
			hasClassExpression = true
			// Handle nested class in field
			c.handleClassDeclaration(child, file, parentID, result)
		}
	}

	if name != "" && hasArrowFunction {
		chunk := c.createChunk(node, file, name, models.ChunkLevelMethod, parentID)
		result.Chunks = append(result.Chunks, chunk)
	}
	// Note: hasClassExpression already handled above
	_ = hasClassExpression
}

// getNodeName extracts the name from a node based on its type.
func (c *JavaScriptChunker) getNodeName(node *sitter.Node, source []byte) string {
	nodeType := node.Type()

	switch nodeType {
	case "class_declaration", "class":
		// Look for identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "identifier" || child.Type() == "type_identifier" {
				return child.Content(source)
			}
		}
	case "interface_declaration":
		// Look for type_identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "type_identifier" {
				return child.Content(source)
			}
		}
	case "type_alias_declaration":
		// Look for type_identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "type_identifier" {
				return child.Content(source)
			}
		}
	case "function_declaration", "generator_function_declaration":
		// Look for identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "identifier" {
				return child.Content(source)
			}
		}
	}

	return ""
}

// getMethodName extracts the name from a method definition.
func (c *JavaScriptChunker) getMethodName(node *sitter.Node, source []byte) string {
	// Method definitions have property_identifier as name
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "property_identifier" {
			return child.Content(source)
		}
	}
	return ""
}

// createChunk creates a new chunk from a node.
func (c *JavaScriptChunker) createChunk(node *sitter.Node, file *models.SourceFile, name string, level models.ChunkLevel, parentID string) *models.Chunk {
	startLine := int(node.StartPoint().Row) + 1 // tree-sitter uses 0-based lines
	endLine := int(node.EndPoint().Row) + 1

	content := node.Content(file.Content)
	signature := c.extractSignature(node, file.Content)

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        level,
		Language:     file.Language,
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
	chunk.SetHashes()
	return chunk
}

// extractSignature extracts a signature from a node (first line or declaration part).
func (c *JavaScriptChunker) extractSignature(node *sitter.Node, source []byte) string {
	content := node.Content(source)

	// Find the first line or up to the opening brace
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// If the first line contains a brace, truncate there
		if idx := strings.Index(firstLine, "{"); idx > 0 {
			return strings.TrimSpace(firstLine[:idx])
		}
		return firstLine
	}
	return ""
}
