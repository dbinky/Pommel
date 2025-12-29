package chunker

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/pommel-dev/pommel/internal/models"
)

// CSharpChunker extracts semantic chunks from C# source code.
type CSharpChunker struct {
	parser *Parser
}

// NewCSharpChunker creates a new CSharpChunker with the given parser.
func NewCSharpChunker(parser *Parser) *CSharpChunker {
	return &CSharpChunker{
		parser: parser,
	}
}

// Language returns the language this chunker handles.
func (c *CSharpChunker) Language() Language {
	return LangCSharp
}

// Chunk extracts semantic chunks from a C# source file.
func (c *CSharpChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
	result := &models.ChunkResult{
		File:   file,
		Chunks: []*models.Chunk{},
		Errors: []error{},
	}

	// Check for cancelled context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Handle empty file
	if len(file.Content) == 0 {
		return result, nil
	}

	// Parse the source code
	tree, err := c.parser.Parse(ctx, LangCSharp, file.Content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	// Create file-level chunk
	fileChunk := c.createFileChunk(file, rootNode)
	if fileChunk != nil {
		result.Chunks = append(result.Chunks, fileChunk)
	}

	// Walk the AST to find classes, structs, interfaces, records, enums
	c.walkNode(ctx, rootNode, file, fileChunk, result)

	return result, nil
}

// createFileChunk creates a file-level chunk for the entire file.
func (c *CSharpChunker) createFileChunk(file *models.SourceFile, rootNode *sitter.Node) *models.Chunk {
	content := string(file.Content)
	if content == "" {
		return nil
	}

	// Count total lines
	endLine := 1
	for _, ch := range content {
		if ch == '\n' {
			endLine++
		}
	}

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    1,
		EndLine:      endLine,
		Level:        models.ChunkLevelFile,
		Language:     "csharp",
		Content:      content,
		Name:         file.Path,
		LastModified: file.LastModified,
	}
	chunk.SetHashes()
	return chunk
}

// walkNode recursively walks the AST to find type declarations and methods.
func (c *CSharpChunker) walkNode(ctx context.Context, node *sitter.Node, file *models.SourceFile, parentChunk *models.Chunk, result *models.ChunkResult) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	if node == nil {
		return
	}

	nodeType := node.Type()

	// Handle type declarations (class, struct, interface, record, enum)
	if c.isTypeDeclaration(nodeType) {
		typeChunk := c.createTypeChunk(file, node, parentChunk)
		if typeChunk != nil {
			result.Chunks = append(result.Chunks, typeChunk)
			// Walk children to find methods and nested types
			c.walkTypeBody(ctx, node, file, typeChunk, result)
		}
		return // Don't recurse further from here; walkTypeBody handles it
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(ctx, child, file, parentChunk, result)
	}
}

// isTypeDeclaration returns true if the node type is a class-level declaration.
func (c *CSharpChunker) isTypeDeclaration(nodeType string) bool {
	switch nodeType {
	case "class_declaration", "struct_declaration", "interface_declaration",
		"record_declaration", "record_struct_declaration", "enum_declaration":
		return true
	default:
		return false
	}
}

// createTypeChunk creates a class-level chunk for a type declaration.
func (c *CSharpChunker) createTypeChunk(file *models.SourceFile, node *sitter.Node, parentChunk *models.Chunk) *models.Chunk {
	name := c.extractTypeName(file.Content, node)
	content := c.extractNodeContent(file.Content, node)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelClass,
		Language:     "csharp",
		Content:      content,
		Name:         name,
		Signature:    c.extractTypeSignature(file.Content, node),
		LastModified: file.LastModified,
	}

	if parentChunk != nil {
		chunk.ParentID = &parentChunk.ID
	}

	chunk.SetHashes()
	return chunk
}

// extractTypeName extracts the name from a type declaration node.
func (c *CSharpChunker) extractTypeName(source []byte, node *sitter.Node) string {
	// Look for identifier child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			return child.Content(source)
		}
	}
	return ""
}

// extractTypeSignature extracts the signature line of a type declaration.
func (c *CSharpChunker) extractTypeSignature(source []byte, node *sitter.Node) string {
	// Get the content up to the opening brace or end of first line
	startByte := node.StartByte()
	endByte := node.EndByte()

	content := string(source[startByte:endByte])

	// Find opening brace or first newline
	for i, ch := range content {
		if ch == '{' {
			return content[:i]
		}
		if ch == '\n' {
			return content[:i]
		}
	}
	return content
}

// walkTypeBody walks the body of a type declaration to find methods and nested types.
func (c *CSharpChunker) walkTypeBody(ctx context.Context, typeNode *sitter.Node, file *models.SourceFile, typeChunk *models.Chunk, result *models.ChunkResult) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Find the declaration_list (body of the type)
	for i := 0; i < int(typeNode.ChildCount()); i++ {
		child := typeNode.Child(i)
		if child.Type() == "declaration_list" {
			c.walkDeclarationList(ctx, child, file, typeChunk, result)
			return
		}
	}
}

// walkDeclarationList walks a declaration_list to extract members.
func (c *CSharpChunker) walkDeclarationList(ctx context.Context, listNode *sitter.Node, file *models.SourceFile, parentChunk *models.Chunk, result *models.ChunkResult) {
	for i := 0; i < int(listNode.ChildCount()); i++ {
		child := listNode.Child(i)
		nodeType := child.Type()

		// Handle nested type declarations
		if c.isTypeDeclaration(nodeType) {
			nestedChunk := c.createTypeChunk(file, child, parentChunk)
			if nestedChunk != nil {
				result.Chunks = append(result.Chunks, nestedChunk)
				c.walkTypeBody(ctx, child, file, nestedChunk, result)
			}
			continue
		}

		// Handle method declarations
		if c.isMethodDeclaration(nodeType) {
			methodChunk := c.createMethodChunk(file, child, parentChunk)
			if methodChunk != nil {
				result.Chunks = append(result.Chunks, methodChunk)
			}
			continue
		}
	}
}

// isMethodDeclaration returns true if the node type is a method-level declaration.
func (c *CSharpChunker) isMethodDeclaration(nodeType string) bool {
	switch nodeType {
	case "method_declaration", "property_declaration", "constructor_declaration":
		return true
	default:
		return false
	}
}

// createMethodChunk creates a method-level chunk.
func (c *CSharpChunker) createMethodChunk(file *models.SourceFile, node *sitter.Node, parentChunk *models.Chunk) *models.Chunk {
	name := c.extractMethodName(file.Content, node)
	content := c.extractNodeContent(file.Content, node)
	signature := c.extractMethodSignature(file.Content, node)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	chunk := &models.Chunk{
		FilePath:     file.Path,
		StartLine:    startLine,
		EndLine:      endLine,
		Level:        models.ChunkLevelMethod,
		Language:     "csharp",
		Content:      content,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}

	if parentChunk != nil {
		chunk.ParentID = &parentChunk.ID
	}

	chunk.SetHashes()
	return chunk
}

// extractMethodName extracts the name from a method, property, or constructor.
func (c *CSharpChunker) extractMethodName(source []byte, node *sitter.Node) string {
	nodeType := node.Type()

	switch nodeType {
	case "method_declaration":
		// Look for identifier child
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" {
				return child.Content(source)
			}
		}
	case "property_declaration":
		// Look for identifier child
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" {
				return child.Content(source)
			}
		}
	case "constructor_declaration":
		// Constructor name is the class name, look for identifier
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" {
				return child.Content(source)
			}
		}
	}

	return ""
}

// extractMethodSignature extracts the signature of a method/property.
func (c *CSharpChunker) extractMethodSignature(source []byte, node *sitter.Node) string {
	startByte := node.StartByte()
	endByte := node.EndByte()

	content := string(source[startByte:endByte])

	// Find opening brace or lambda arrow or first newline
	for i, ch := range content {
		if ch == '{' || ch == '\n' {
			return content[:i]
		}
	}
	return content
}

// extractNodeContent extracts the full text content of a node.
func (c *CSharpChunker) extractNodeContent(source []byte, node *sitter.Node) string {
	startByte := node.StartByte()
	endByte := node.EndByte()
	return string(source[startByte:endByte])
}
