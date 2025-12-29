package chunker

import (
	"context"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
	sitter "github.com/smacker/go-tree-sitter"
)

// PythonChunker extracts chunks from Python source files
type PythonChunker struct {
	parser *Parser
}

// NewPythonChunker creates a new Python chunker
func NewPythonChunker(parser *Parser) *PythonChunker {
	return &PythonChunker{parser: parser}
}

// Chunk extracts chunks from a Python source file
func (c *PythonChunker) Chunk(ctx context.Context, file *models.SourceFile) (*models.ChunkResult, error) {
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

	tree, err := c.parser.Parse(ctx, LangPython, file.Content)
	if err != nil {
		return nil, err
	}

	// Add file-level chunk
	fileChunk := c.createFileChunk(file)
	fileChunk.SetHashes()
	result.Chunks = append(result.Chunks, fileChunk)

	// Walk the AST - top-level only first
	c.walkNode(tree.RootNode(), file, fileChunk.ID, result, true)

	// Set hashes for all chunks (file chunk already has hashes set)
	for _, chunk := range result.Chunks {
		if chunk.ID == "" {
			chunk.SetHashes()
		}
	}

	return result, nil
}

func (c *PythonChunker) createFileChunk(file *models.SourceFile) *models.Chunk {
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
		Language:     string(LangPython),
		Content:      string(file.Content),
		Name:         file.Path,
		LastModified: file.LastModified,
	}
}

func (c *PythonChunker) walkNode(node *sitter.Node, file *models.SourceFile, parentID string, result *models.ChunkResult, isTopLevel bool) {
	nodeType := node.Type()

	switch nodeType {
	case "class_definition":
		chunk := c.extractClassChunk(node, file, parentID)
		if chunk != nil {
			chunk.SetHashes()
			result.Chunks = append(result.Chunks, chunk)
			// Walk class body for methods and nested classes
			body := node.ChildByFieldName("body")
			if body != nil {
				c.walkClassBody(body, file, chunk.ID, result)
			}
			return
		}

	case "function_definition":
		// Top-level functions are at method level
		// Methods inside classes are handled by walkClassBody
		if isTopLevel {
			chunk := c.extractFunctionChunk(node, file, parentID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
			}
		}
		return

	case "decorated_definition":
		// Handle decorated classes/functions
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "class_definition" || child.Type() == "function_definition" {
				c.walkNode(child, file, parentID, result, isTopLevel)
				return
			}
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkNode(child, file, parentID, result, isTopLevel)
	}
}

func (c *PythonChunker) walkClassBody(node *sitter.Node, file *models.SourceFile, classID string, result *models.ChunkResult) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_definition":
			chunk := c.extractFunctionChunk(child, file, classID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
			}
		case "decorated_definition":
			// Find the function or class inside the decorator
			for j := 0; j < int(child.ChildCount()); j++ {
				inner := child.Child(j)
				if inner.Type() == "function_definition" {
					chunk := c.extractFunctionChunk(inner, file, classID)
					if chunk != nil {
						chunk.SetHashes()
						result.Chunks = append(result.Chunks, chunk)
					}
				} else if inner.Type() == "class_definition" {
					chunk := c.extractClassChunk(inner, file, classID)
					if chunk != nil {
						chunk.SetHashes()
						result.Chunks = append(result.Chunks, chunk)
						body := inner.ChildByFieldName("body")
						if body != nil {
							c.walkClassBody(body, file, chunk.ID, result)
						}
					}
				}
			}
		case "class_definition":
			// Nested class
			chunk := c.extractClassChunk(child, file, classID)
			if chunk != nil {
				chunk.SetHashes()
				result.Chunks = append(result.Chunks, chunk)
				body := child.ChildByFieldName("body")
				if body != nil {
					c.walkClassBody(body, file, chunk.ID, result)
				}
			}
		}
	}
}

func (c *PythonChunker) extractClassChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
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
		Language:     string(LangPython),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

func (c *PythonChunker) extractFunctionChunk(node *sitter.Node, file *models.SourceFile, parentID string) *models.Chunk {
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
		Language:     string(LangPython),
		Content:      content,
		ParentID:     &parentID,
		Name:         name,
		Signature:    signature,
		LastModified: file.LastModified,
	}
}

// Language returns the language this chunker handles
func (c *PythonChunker) Language() Language {
	return LangPython
}
