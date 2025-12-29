package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

func createValidChunk() *Chunk {
	return &Chunk{
		FilePath:     "/path/to/file.go",
		StartLine:    10,
		EndLine:      20,
		Level:        ChunkLevelMethod,
		Language:     "go",
		Content:      "func example() {\n    return nil\n}",
		Name:         "example",
		Signature:    "func example()",
		LastModified: time.Now(),
	}
}

func createValidChunkWithContent(content string) *Chunk {
	chunk := createValidChunk()
	chunk.Content = content
	return chunk
}

// =============================================================================
// ChunkLevel Constants Tests
// =============================================================================

func TestChunkLevelConstants(t *testing.T) {
	t.Run("ChunkLevelFile has correct value", func(t *testing.T) {
		assert.Equal(t, ChunkLevel("file"), ChunkLevelFile)
	})

	t.Run("ChunkLevelClass has correct value", func(t *testing.T) {
		assert.Equal(t, ChunkLevel("class"), ChunkLevelClass)
	})

	t.Run("ChunkLevelMethod has correct value", func(t *testing.T) {
		assert.Equal(t, ChunkLevel("method"), ChunkLevelMethod)
	})
}

// =============================================================================
// GenerateID Tests
// =============================================================================

func TestGenerateID(t *testing.T) {
	t.Run("produces deterministic results for same chunk", func(t *testing.T) {
		chunk1 := createValidChunk()
		chunk2 := createValidChunk()

		// Same chunk data should produce same ID
		id1 := chunk1.GenerateID()
		id2 := chunk2.GenerateID()

		assert.Equal(t, id1, id2, "same chunk should produce same ID")
	})

	t.Run("produces different IDs for different file paths", func(t *testing.T) {
		chunk1 := createValidChunk()
		chunk2 := createValidChunk()
		chunk2.FilePath = "/different/path/file.go"

		id1 := chunk1.GenerateID()
		id2 := chunk2.GenerateID()

		assert.NotEqual(t, id1, id2, "different file paths should produce different IDs")
	})

	t.Run("produces different IDs for different start lines", func(t *testing.T) {
		chunk1 := createValidChunk()
		chunk2 := createValidChunk()
		chunk2.StartLine = 15

		id1 := chunk1.GenerateID()
		id2 := chunk2.GenerateID()

		assert.NotEqual(t, id1, id2, "different start lines should produce different IDs")
	})

	t.Run("produces different IDs for different end lines", func(t *testing.T) {
		chunk1 := createValidChunk()
		chunk2 := createValidChunk()
		chunk2.EndLine = 25

		id1 := chunk1.GenerateID()
		id2 := chunk2.GenerateID()

		assert.NotEqual(t, id1, id2, "different end lines should produce different IDs")
	})

	t.Run("produces different IDs for different levels", func(t *testing.T) {
		chunk1 := createValidChunk()
		chunk2 := createValidChunk()
		chunk2.Level = ChunkLevelClass

		id1 := chunk1.GenerateID()
		id2 := chunk2.GenerateID()

		assert.NotEqual(t, id1, id2, "different levels should produce different IDs")
	})

	t.Run("ID length is 32 hex characters", func(t *testing.T) {
		chunk := createValidChunk()
		id := chunk.GenerateID()

		assert.Len(t, id, 32, "ID should be 32 hex characters (MD5 or similar hash)")
	})

	t.Run("ID contains only valid hex characters", func(t *testing.T) {
		chunk := createValidChunk()
		id := chunk.GenerateID()

		for _, c := range id {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			assert.True(t, isHex, "ID should contain only hex characters, got: %c", c)
		}
	})

	t.Run("ID is stable across multiple calls", func(t *testing.T) {
		chunk := createValidChunk()

		id1 := chunk.GenerateID()
		id2 := chunk.GenerateID()
		id3 := chunk.GenerateID()

		assert.Equal(t, id1, id2)
		assert.Equal(t, id2, id3)
	})
}

// =============================================================================
// GenerateContentHash Tests
// =============================================================================

func TestGenerateContentHash(t *testing.T) {
	t.Run("produces deterministic results for same content", func(t *testing.T) {
		chunk1 := createValidChunkWithContent("func hello() {}")
		chunk2 := createValidChunkWithContent("func hello() {}")

		hash1 := chunk1.GenerateContentHash()
		hash2 := chunk2.GenerateContentHash()

		assert.Equal(t, hash1, hash2, "same content should produce same hash")
	})

	t.Run("produces different hashes for different content", func(t *testing.T) {
		chunk1 := createValidChunkWithContent("func hello() {}")
		chunk2 := createValidChunkWithContent("func goodbye() {}")

		hash1 := chunk1.GenerateContentHash()
		hash2 := chunk2.GenerateContentHash()

		assert.NotEqual(t, hash1, hash2, "different content should produce different hashes")
	})

	t.Run("hash length is 32 hex characters", func(t *testing.T) {
		chunk := createValidChunk()
		hash := chunk.GenerateContentHash()

		assert.Len(t, hash, 32, "hash should be 32 hex characters")
	})

	t.Run("hash contains only valid hex characters", func(t *testing.T) {
		chunk := createValidChunk()
		hash := chunk.GenerateContentHash()

		for _, c := range hash {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			assert.True(t, isHex, "hash should contain only hex characters, got: %c", c)
		}
	})

	t.Run("hash is stable across multiple calls", func(t *testing.T) {
		chunk := createValidChunk()

		hash1 := chunk.GenerateContentHash()
		hash2 := chunk.GenerateContentHash()
		hash3 := chunk.GenerateContentHash()

		assert.Equal(t, hash1, hash2)
		assert.Equal(t, hash2, hash3)
	})

	t.Run("empty content produces valid hash", func(t *testing.T) {
		chunk := createValidChunkWithContent("")
		hash := chunk.GenerateContentHash()

		assert.Len(t, hash, 32, "empty content should still produce 32-char hash")
	})

	t.Run("whitespace differences produce different hashes", func(t *testing.T) {
		chunk1 := createValidChunkWithContent("func hello() {}")
		chunk2 := createValidChunkWithContent("func hello()  {}")

		hash1 := chunk1.GenerateContentHash()
		hash2 := chunk2.GenerateContentHash()

		assert.NotEqual(t, hash1, hash2, "whitespace changes should produce different hashes")
	})
}

// =============================================================================
// SetHashes Tests
// =============================================================================

func TestSetHashes(t *testing.T) {
	t.Run("populates ID field", func(t *testing.T) {
		chunk := createValidChunk()
		assert.Empty(t, chunk.ID, "ID should be empty before SetHashes")

		chunk.SetHashes()

		assert.NotEmpty(t, chunk.ID, "ID should be populated after SetHashes")
		assert.Len(t, chunk.ID, 32)
	})

	t.Run("populates ContentHash field", func(t *testing.T) {
		chunk := createValidChunk()
		assert.Empty(t, chunk.ContentHash, "ContentHash should be empty before SetHashes")

		chunk.SetHashes()

		assert.NotEmpty(t, chunk.ContentHash, "ContentHash should be populated after SetHashes")
		assert.Len(t, chunk.ContentHash, 32)
	})

	t.Run("ID matches GenerateID result", func(t *testing.T) {
		chunk := createValidChunk()
		expectedID := chunk.GenerateID()

		chunk.SetHashes()

		assert.Equal(t, expectedID, chunk.ID)
	})

	t.Run("ContentHash matches GenerateContentHash result", func(t *testing.T) {
		chunk := createValidChunk()
		expectedHash := chunk.GenerateContentHash()

		chunk.SetHashes()

		assert.Equal(t, expectedHash, chunk.ContentHash)
	})

	t.Run("overwrites existing ID and ContentHash", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.ID = "old-id-value"
		chunk.ContentHash = "old-hash-value"

		chunk.SetHashes()

		assert.NotEqual(t, "old-id-value", chunk.ID)
		assert.NotEqual(t, "old-hash-value", chunk.ContentHash)
		assert.Len(t, chunk.ID, 32)
		assert.Len(t, chunk.ContentHash, 32)
	})
}

// =============================================================================
// IsValid Tests
// =============================================================================

func TestIsValid(t *testing.T) {
	t.Run("valid chunk passes validation", func(t *testing.T) {
		chunk := createValidChunk()

		err := chunk.IsValid()

		assert.NoError(t, err)
	})

	t.Run("missing file path returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.FilePath = ""

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "file path")
	})

	t.Run("start line less than 1 returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 0

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "start line")
	})

	t.Run("negative start line returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = -5

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "start line")
	})

	t.Run("end line less than start line returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 20
		chunk.EndLine = 10

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "end line")
	})

	t.Run("missing content returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Content = ""

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "content")
	})

	t.Run("missing level returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Level = ""

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "level")
	})

	t.Run("end line equal to start line is valid (single line chunk)", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 10
		chunk.EndLine = 10

		err := chunk.IsValid()

		assert.NoError(t, err)
	})

	t.Run("whitespace-only content returns error", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Content = "   \n\t\n   "

		err := chunk.IsValid()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "content")
	})

	t.Run("chunk with ParentID is valid", func(t *testing.T) {
		chunk := createValidChunk()
		parentID := "abc123"
		chunk.ParentID = &parentID

		err := chunk.IsValid()

		assert.NoError(t, err)
	})
}

// =============================================================================
// LineCount Tests
// =============================================================================

func TestLineCount(t *testing.T) {
	t.Run("returns correct count for multi-line chunk", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 10
		chunk.EndLine = 20

		count := chunk.LineCount()

		assert.Equal(t, 11, count) // 20 - 10 + 1 = 11
	})

	t.Run("returns 1 for single-line chunk", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 15
		chunk.EndLine = 15

		count := chunk.LineCount()

		assert.Equal(t, 1, count)
	})

	t.Run("returns correct count for two-line chunk", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 1
		chunk.EndLine = 2

		count := chunk.LineCount()

		assert.Equal(t, 2, count)
	})

	t.Run("returns correct count for large chunk", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.StartLine = 1
		chunk.EndLine = 1000

		count := chunk.LineCount()

		assert.Equal(t, 1000, count)
	})
}

// =============================================================================
// SourceFile Tests
// =============================================================================

func TestSourceFile(t *testing.T) {
	t.Run("can create SourceFile with all fields", func(t *testing.T) {
		now := time.Now()
		sf := &SourceFile{
			Path:         "/path/to/source.go",
			Content:      []byte("package main\n\nfunc main() {}"),
			Language:     "go",
			LastModified: now,
		}

		assert.Equal(t, "/path/to/source.go", sf.Path)
		assert.Equal(t, []byte("package main\n\nfunc main() {}"), sf.Content)
		assert.Equal(t, "go", sf.Language)
		assert.Equal(t, now, sf.LastModified)
	})

	t.Run("Content is byte slice", func(t *testing.T) {
		sf := &SourceFile{
			Content: []byte("hello world"),
		}

		// Verify it's a byte slice that can be converted to string
		assert.Equal(t, "hello world", string(sf.Content))
	})
}

// =============================================================================
// ChunkResult Tests
// =============================================================================

func TestChunkResult(t *testing.T) {
	t.Run("can create ChunkResult with file and chunks", func(t *testing.T) {
		sf := &SourceFile{
			Path:     "/path/to/file.go",
			Content:  []byte("package main"),
			Language: "go",
		}

		chunk1 := createValidChunk()
		chunk2 := createValidChunk()
		chunk2.StartLine = 30
		chunk2.EndLine = 40

		result := &ChunkResult{
			File:   sf,
			Chunks: []*Chunk{chunk1, chunk2},
			Errors: nil,
		}

		assert.Equal(t, sf, result.File)
		assert.Len(t, result.Chunks, 2)
		assert.Nil(t, result.Errors)
	})

	t.Run("can create ChunkResult with errors", func(t *testing.T) {
		sf := &SourceFile{
			Path: "/path/to/file.go",
		}

		err1 := assert.AnError
		err2 := assert.AnError

		result := &ChunkResult{
			File:   sf,
			Chunks: nil,
			Errors: []error{err1, err2},
		}

		assert.Equal(t, sf, result.File)
		assert.Nil(t, result.Chunks)
		assert.Len(t, result.Errors, 2)
	})

	t.Run("can have both chunks and errors", func(t *testing.T) {
		sf := &SourceFile{
			Path: "/path/to/file.go",
		}

		chunk := createValidChunk()

		result := &ChunkResult{
			File:   sf,
			Chunks: []*Chunk{chunk},
			Errors: []error{assert.AnError},
		}

		assert.Len(t, result.Chunks, 1)
		assert.Len(t, result.Errors, 1)
	})
}

// =============================================================================
// Edge Cases and Integration Tests
// =============================================================================

func TestChunkEdgeCases(t *testing.T) {
	t.Run("chunk with unicode content generates valid hashes", func(t *testing.T) {
		chunk := createValidChunkWithContent("func hello() { return \"Hello\" }")

		id := chunk.GenerateID()
		hash := chunk.GenerateContentHash()

		assert.Len(t, id, 32)
		assert.Len(t, hash, 32)
	})

	t.Run("chunk with very long content generates valid hashes", func(t *testing.T) {
		// Generate a long content string
		longContent := ""
		for i := 0; i < 10000; i++ {
			longContent += "x"
		}
		chunk := createValidChunkWithContent(longContent)

		id := chunk.GenerateID()
		hash := chunk.GenerateContentHash()

		assert.Len(t, id, 32)
		assert.Len(t, hash, 32)
	})

	t.Run("chunk with special characters in path", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.FilePath = "/path/with spaces/and-dashes/file_name.go"

		id := chunk.GenerateID()

		assert.Len(t, id, 32)
	})

	t.Run("chunk with nil ParentID is valid", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.ParentID = nil

		err := chunk.IsValid()

		assert.NoError(t, err)
	})

	t.Run("all ChunkLevel values can be used", func(t *testing.T) {
		levels := []ChunkLevel{ChunkLevelFile, ChunkLevelClass, ChunkLevelMethod}

		for _, level := range levels {
			chunk := createValidChunk()
			chunk.Level = level

			err := chunk.IsValid()

			assert.NoError(t, err, "level %s should be valid", level)
		}
	})

	t.Run("chunk with empty Name is valid", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Name = ""

		err := chunk.IsValid()

		assert.NoError(t, err, "empty name should be valid (file-level chunks may not have names)")
	})

	t.Run("chunk with empty Signature is valid", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Signature = ""

		err := chunk.IsValid()

		assert.NoError(t, err, "empty signature should be valid")
	})

	t.Run("chunk with empty Language is valid", func(t *testing.T) {
		chunk := createValidChunk()
		chunk.Language = ""

		err := chunk.IsValid()

		assert.NoError(t, err, "empty language should be valid (can be inferred later)")
	})
}
