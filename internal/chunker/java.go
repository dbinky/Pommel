package chunker

import (
	"context"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaChunker extracts chunks from Java source files
type JavaChunker struct {
	parser *Parser
}

// NewJavaChunker creates a new Java chunker
func NewJavaChunker(parser *Parser) *JavaChunker {
	return &JavaChunker{parser: parser}
}

// Chunk extracts chunks from a Java source file
func (c *JavaChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
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

	tree, err := c.parser.Parse(ctx, LangJava, file.Content)
	if err != nil {
		return nil, err
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	fileChunk.SetHashes()
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST for declarations
	c.walkNode(tree.RootNode(), file, fileChunk.ID, "", result)

	// Set hashes for all chunks (file chunk already has hashes set)
	for _, chunk := range result.Chunks {
		if chunk.ID == "" {
			chunk.SetHashes()
		}
	}

	return result, nil
}

func (c *JavaChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
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
		Language:     string(LangJava),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *JavaChunker) walkNode(node *sitter.Node, file *models.SourceFile, fileID string, currentClassID string, result *models.ChunkResult) {
	nodeType := node.Type()

	switch nodeType {
	case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
		chunk := c.extractClassChunk(node, file, fileID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Recurse into the class body to find methods, using this class as parent
			c.walkClassBody(node, file, fileID, chunk.ID, result)
		}
		return

	case "method_declaration", "constructor_declaration":
		// Methods/constructors at top level (shouldn't happen in valid Java, but handle gracefully)
		chunk := c.extractMethodChunk(node, file, fileID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
		}
		return
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(child, file, fileID, currentClassID, result)
	}
}

func (c *JavaChunker) walkClassBody(node *sitter.Node, file *models.SourceFile, fileID string, classID string, result *models.ChunkResult) {
	// Find the class body
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == "class_body" || childType == "interface_body" || childType == "enum_body" || childType == "record_declaration_body" || childType == "annotation_type_body" {
			c.walkBodyContents(child, file, fileID, classID, result)
			return
		}
	}
}

func (c *JavaChunker) walkBodyContents(node *sitter.Node, file *models.SourceFile, fileID string, classID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "method_declaration", "constructor_declaration":
			chunk := c.extractMethodChunk(child, file, classID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
			}

		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			// Nested class - use file as parent (flat structure)
			chunk := c.extractClassChunk(child, file, fileID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
				// Recurse into nested class body
				c.walkClassBody(child, file, fileID, chunk.ID, result)
			}
		}
	}
}

func (c *JavaChunker) extractClassChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	// Get the class/interface/enum/record name
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
		Level:        models.ChunkLevelClass,
		Language:     string(LangJava),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *JavaChunker) extractMethodChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
	// Get the method/constructor name
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
		Language:     string(LangJava),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// Language returns the language this chunker handles
func (c *JavaChunker) Language() Language {
	return LangJava
}
