package chunker

import (
	"context"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
	sitter "github.com/smacker/go-tree-sitter"
)

// GoChunker extracts chunks from Go source files
type GoChunker struct {
	parser *Parser
}

// NewGoChunker creates a new Go chunker
func NewGoChunker(parser *Parser) *GoChunker {
	return &GoChunker{parser: parser}
}

// Chunk extracts chunks from a Go source file
func (c *GoChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
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

	tree, err := c.parser.Parse(ctx, LangGo, file.Content)
	if err != nil {
		return nil, err
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	fileChunk.SetHashes()
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST for top-level declarations
	c.walkNode(tree.RootNode(), file, fileChunk.ID, result)

	// Set hashes for all chunks (file chunk already has hashes set)
	for _, chunk := range result.Chunks {
		if chunk.ID == "" {
			chunk.SetHashes()
		}
	}

	return result, nil
}

func (c *GoChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
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
		Language:     string(LangGo),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *GoChunker) walkNode(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		chunk := c.extractFunctionChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
		}
		return

	case "method_declaration":
		chunk := c.extractMethodChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
		}
		return

	case "type_declaration":
		// Type declarations can contain multiple type_spec nodes
		c.extractTypeDeclarations(node, file, parentID, result)
		return
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(child, file, parentID, result)
	}
}

func (c *GoChunker) extractTypeDeclarations(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult) {
	// Type declarations contain type_spec children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			chunk := c.extractTypeSpec(child, file, parentID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
			}
		}
	}
}

func (c *GoChunker) extractTypeSpec(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	// Get the type name
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
		Language:     string(LangGo),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *GoChunker) extractFunctionChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

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
		Level:        models.ChunkLevelMethod,
		Language:     string(LangGo),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *GoChunker) extractMethodChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Content(file.Content)

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
		Level:        models.ChunkLevelMethod,
		Language:     string(LangGo),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// Language returns the language this chunker handles
func (c *GoChunker) Language() Language {
	return LangGo
}
