package chunker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// FallbackChunker Initialization Tests
// =============================================================================

func TestNewFallbackChunker(t *testing.T) {
	chunker := NewFallbackChunker()
	assert.NotNil(t, chunker, "NewFallbackChunker should return a non-nil chunker")
}

func TestFallbackChunker_Language(t *testing.T) {
	chunker := NewFallbackChunker()
	// FallbackChunker handles all unknown/unsupported languages
	assert.Equal(t, LangUnknown, chunker.Language(), "FallbackChunker should return LangUnknown")
}

// =============================================================================
// Single Chunk Tests
// =============================================================================

func TestFallbackChunker_SingleChunk(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'\nputs 'world'"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	assert.Len(t, result.Chunks, 1, "FallbackChunker should produce exactly 1 chunk")
	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level, "Chunk level should be file")
	assert.Equal(t, 1, result.Chunks[0].StartLine, "StartLine should be 1")
	assert.Equal(t, 2, result.Chunks[0].EndLine, "EndLine should be 2 for 2 lines of content")
}

func TestFallbackChunker_AlwaysProducesSingleChunk(t *testing.T) {
	chunker := NewFallbackChunker()

	// Test with multiple file types - all should produce exactly 1 chunk
	files := []*models.SourceFile{
		{
			Path:     "main.go",
			Content:  []byte("package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"),
			Language: "go",
		},
		{
			Path:     "app.rb",
			Content:  []byte("class App\n  def run\n    puts 'running'\n  end\nend"),
			Language: "ruby",
		},
		{
			Path:     "lib.rs",
			Content:  []byte("fn main() {\n    println!(\"Hello\");\n}"),
			Language: "rust",
		},
	}

	for _, file := range files {
		t.Run(file.Path, func(t *testing.T) {
			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)
			assert.Len(t, result.Chunks, 1, "Should always produce exactly 1 chunk for %s", file.Path)
			assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level)
		})
	}
}

// =============================================================================
// Content Preservation Tests
// =============================================================================

func TestFallbackChunker_ContentPreservation(t *testing.T) {
	chunker := NewFallbackChunker()

	content := "puts 'hello'\nputs 'world'\nputs 'goodbye'"
	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte(content),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, content, result.Chunks[0].Content, "Chunk content should match original file content exactly")
}

func TestFallbackChunker_PreservesWhitespace(t *testing.T) {
	chunker := NewFallbackChunker()

	// Content with various whitespace: tabs, multiple spaces, blank lines
	content := "line one\n\n\tindented line\n    spaces\n\nfinal line"
	file := &models.SourceFile{
		Path:     "test.txt",
		Content:  []byte(content),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, content, result.Chunks[0].Content, "Whitespace should be preserved exactly")
}

func TestFallbackChunker_PreservesUnicode(t *testing.T) {
	chunker := NewFallbackChunker()

	content := "Hello World\nUnicode: symbols, characters\nEmoji: (emojis)"
	file := &models.SourceFile{
		Path:     "unicode.txt",
		Content:  []byte(content),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, content, result.Chunks[0].Content, "Unicode content should be preserved")
}

// =============================================================================
// Line Count Tests
// =============================================================================

func TestFallbackChunker_LineCount_SingleLine(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "single.txt",
		Content:  []byte("just one line"),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, 1, result.Chunks[0].StartLine, "StartLine should be 1")
	assert.Equal(t, 1, result.Chunks[0].EndLine, "EndLine should be 1 for single line")
}

func TestFallbackChunker_LineCount_MultipleLines(t *testing.T) {
	chunker := NewFallbackChunker()

	testCases := []struct {
		name          string
		content       string
		expectedLines int
	}{
		{"two lines", "line1\nline2", 2},
		{"three lines", "line1\nline2\nline3", 3},
		{"five lines", "1\n2\n3\n4\n5", 5},
		{"ten lines", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := &models.SourceFile{
				Path:     "test.txt",
				Content:  []byte(tc.content),
				Language: "text",
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)
			require.Len(t, result.Chunks, 1)

			assert.Equal(t, 1, result.Chunks[0].StartLine, "StartLine should always be 1")
			assert.Equal(t, tc.expectedLines, result.Chunks[0].EndLine, "EndLine should match line count")
		})
	}
}

func TestFallbackChunker_LineCount_TrailingNewline(t *testing.T) {
	chunker := NewFallbackChunker()

	// Content with trailing newline
	file := &models.SourceFile{
		Path:     "trailing.txt",
		Content:  []byte("line1\nline2\n"),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	// Trailing newline creates an empty 3rd line
	assert.Equal(t, 1, result.Chunks[0].StartLine)
	assert.Equal(t, 3, result.Chunks[0].EndLine, "Trailing newline should count as additional line")
}

// =============================================================================
// Chunk Fields Tests
// =============================================================================

func TestFallbackChunker_ChunkFields_Name(t *testing.T) {
	chunker := NewFallbackChunker()

	paths := []string{
		"script.rb",
		"path/to/file.go",
		"deeply/nested/path/to/module.rs",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			file := &models.SourceFile{
				Path:     path,
				Content:  []byte("content"),
				Language: "unknown",
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)
			require.Len(t, result.Chunks, 1)

			assert.Equal(t, path, result.Chunks[0].Name, "Chunk name should be the file path")
		})
	}
}

func TestFallbackChunker_ChunkFields_FilePath(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "/absolute/path/to/file.rb",
		Content:  []byte("content"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, "/absolute/path/to/file.rb", result.Chunks[0].FilePath, "FilePath should match source file path")
}

func TestFallbackChunker_ChunkFields_Language(t *testing.T) {
	chunker := NewFallbackChunker()

	languages := []string{"ruby", "rust", "go", "haskell", "text", "unknown"}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			file := &models.SourceFile{
				Path:     "file.ext",
				Content:  []byte("content"),
				Language: lang,
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err)
			require.Len(t, result.Chunks, 1)

			assert.Equal(t, lang, result.Chunks[0].Language, "Chunk language should match source file language")
		})
	}
}

func TestFallbackChunker_ChunkFields_ParentID(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "file.rb",
		Content:  []byte("content"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Nil(t, result.Chunks[0].ParentID, "File-level chunk should have nil ParentID")
}

func TestFallbackChunker_ChunkFields_Level(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "file.rb",
		Content:  []byte("content"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, models.ChunkLevelFile, result.Chunks[0].Level, "Chunk level should be ChunkLevelFile")
}

func TestFallbackChunker_ChunkFields_LastModified(t *testing.T) {
	chunker := NewFallbackChunker()

	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	file := &models.SourceFile{
		Path:         "file.rb",
		Content:      []byte("content"),
		Language:     "ruby",
		LastModified: modTime,
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.Equal(t, modTime, result.Chunks[0].LastModified, "Chunk LastModified should match source file")
}

// =============================================================================
// Various File Types Tests
// =============================================================================

func TestFallbackChunker_VariousFileTypes(t *testing.T) {
	chunker := NewFallbackChunker()

	testCases := []struct {
		path     string
		language string
		content  string
	}{
		{
			path:     "main.go",
			language: "go",
			content:  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
		},
		{
			path:     "script.rb",
			language: "ruby",
			content:  "class Hello\n  def say\n    puts 'hello'\n  end\nend",
		},
		{
			path:     "lib.rs",
			language: "rust",
			content:  "fn main() {\n    println!(\"Hello, world!\");\n}",
		},
		{
			path:     "README.md",
			language: "markdown",
			content:  "# Title\n\nThis is a paragraph.\n\n## Section\n\nMore content here.",
		},
		{
			path:     "notes.txt",
			language: "text",
			content:  "Just some plain text\nWith multiple lines\nAnd nothing special",
		},
		{
			path:     "config.yaml",
			language: "yaml",
			content:  "key: value\nlist:\n  - item1\n  - item2",
		},
		{
			path:     "data.json",
			language: "json",
			content:  "{\n  \"name\": \"test\",\n  \"value\": 123\n}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			file := &models.SourceFile{
				Path:     tc.path,
				Content:  []byte(tc.content),
				Language: tc.language,
			}

			result, err := chunker.Chunk(context.Background(), file)
			require.NoError(t, err, "Should not error for %s", tc.path)
			require.Len(t, result.Chunks, 1, "Should produce exactly 1 chunk for %s", tc.path)

			chunk := result.Chunks[0]
			assert.Equal(t, models.ChunkLevelFile, chunk.Level, "Level should be file for %s", tc.path)
			assert.Equal(t, tc.content, chunk.Content, "Content should match for %s", tc.path)
			assert.Equal(t, tc.language, chunk.Language, "Language should match for %s", tc.path)
			assert.Equal(t, tc.path, chunk.FilePath, "FilePath should match for %s", tc.path)
		})
	}
}

// =============================================================================
// Empty File Tests
// =============================================================================

func TestFallbackChunker_EmptyFile(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "empty.txt",
		Content:  []byte(""),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err, "Empty file should not cause an error")
	require.Len(t, result.Chunks, 1, "Empty file should still produce a chunk")

	chunk := result.Chunks[0]
	assert.Equal(t, models.ChunkLevelFile, chunk.Level)
	assert.Equal(t, "", chunk.Content, "Empty file chunk should have empty content")
	assert.Equal(t, "empty.txt", chunk.FilePath)
}

func TestFallbackChunker_WhitespaceOnlyFile(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "whitespace.txt",
		Content:  []byte("   \n\t\n   \n"),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err, "Whitespace-only file should not cause an error")
	require.Len(t, result.Chunks, 1, "Whitespace-only file should produce a chunk")

	chunk := result.Chunks[0]
	assert.Equal(t, models.ChunkLevelFile, chunk.Level)
	assert.Equal(t, "   \n\t\n   \n", chunk.Content, "Whitespace should be preserved")
}

// =============================================================================
// Deterministic ID Tests
// =============================================================================

func TestFallbackChunker_DeterministicIDs(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'\nputs 'world'"),
		Language: "ruby",
	}

	// Parse the same file twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	require.Len(t, result1.Chunks, 1)
	require.Len(t, result2.Chunks, 1)

	assert.Equal(t, result1.Chunks[0].ID, result2.Chunks[0].ID,
		"Same file should produce identical chunk IDs")
}

func TestFallbackChunker_DeterministicContentHash(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'\nputs 'world'"),
		Language: "ruby",
	}

	// Parse the same file twice
	result1, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	require.Len(t, result1.Chunks, 1)
	require.Len(t, result2.Chunks, 1)

	assert.Equal(t, result1.Chunks[0].ContentHash, result2.Chunks[0].ContentHash,
		"Same content should produce identical content hashes")
}

func TestFallbackChunker_DifferentFilesHaveDifferentIDs(t *testing.T) {
	chunker := NewFallbackChunker()

	file1 := &models.SourceFile{
		Path:     "file1.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	file2 := &models.SourceFile{
		Path:     "file2.rb",
		Content:  []byte("puts 'hello'"), // Same content, different path
		Language: "ruby",
	}

	result1, err := chunker.Chunk(context.Background(), file1)
	require.NoError(t, err)

	result2, err := chunker.Chunk(context.Background(), file2)
	require.NoError(t, err)

	require.Len(t, result1.Chunks, 1)
	require.Len(t, result2.Chunks, 1)

	assert.NotEqual(t, result1.Chunks[0].ID, result2.Chunks[0].ID,
		"Different file paths should produce different chunk IDs")

	// But content hashes should be the same since content is identical
	assert.Equal(t, result1.Chunks[0].ContentHash, result2.Chunks[0].ContentHash,
		"Same content should have same content hash regardless of path")
}

func TestFallbackChunker_IDsAreNonEmpty(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	assert.NotEmpty(t, result.Chunks[0].ID, "Chunk ID should not be empty")
	assert.NotEmpty(t, result.Chunks[0].ContentHash, "ContentHash should not be empty")
}

// =============================================================================
// ChunkResult Tests
// =============================================================================

func TestFallbackChunker_ChunkResult_File(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	assert.Equal(t, file, result.File, "ChunkResult.File should reference the original source file")
}

func TestFallbackChunker_ChunkResult_NoErrors(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)

	assert.Empty(t, result.Errors, "ChunkResult.Errors should be empty for valid input")
}

// =============================================================================
// Context Handling Tests
// =============================================================================

func TestFallbackChunker_CancelledContext(t *testing.T) {
	chunker := NewFallbackChunker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	// The fallback chunker is simple enough that it might complete
	// before checking context, which is acceptable behavior
	_, err := chunker.Chunk(ctx, file)

	// Either no error (completed before context check) or context error
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled, "If error occurs, should be context.Canceled")
	}
}

// =============================================================================
// Nil Input Tests
// =============================================================================

func TestFallbackChunker_NilFile(t *testing.T) {
	chunker := NewFallbackChunker()

	result, err := chunker.Chunk(context.Background(), nil)
	assert.Error(t, err, "Should return error for nil file")
	assert.Nil(t, result, "Result should be nil for nil input")
}

func TestFallbackChunker_NilContent(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  nil,
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	// This could either error or treat nil as empty - either is acceptable
	if err == nil {
		require.Len(t, result.Chunks, 1)
		assert.Empty(t, result.Chunks[0].Content, "Nil content should be treated as empty")
	}
}

// =============================================================================
// Large File Tests
// =============================================================================

func TestFallbackChunker_LargeFile(t *testing.T) {
	chunker := NewFallbackChunker()

	// Create a large file with 10000 lines
	var builder strings.Builder
	for i := 0; i < 10000; i++ {
		builder.WriteString("This is line number ")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString("\n")
	}
	content := builder.String()

	file := &models.SourceFile{
		Path:     "large.txt",
		Content:  []byte(content),
		Language: "text",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err, "Large file should not cause an error")
	require.Len(t, result.Chunks, 1, "Large file should still produce exactly 1 chunk")

	chunk := result.Chunks[0]
	assert.Equal(t, models.ChunkLevelFile, chunk.Level)
	assert.Equal(t, 1, chunk.StartLine)
	assert.Equal(t, 10001, chunk.EndLine, "Should have 10001 lines (10000 + trailing newline)")
}

// =============================================================================
// Signature Field Tests
// =============================================================================

func TestFallbackChunker_ChunkFields_Signature(t *testing.T) {
	chunker := NewFallbackChunker()

	file := &models.SourceFile{
		Path:     "script.rb",
		Content:  []byte("puts 'hello'"),
		Language: "ruby",
	}

	result, err := chunker.Chunk(context.Background(), file)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)

	// For file-level chunks, signature might be empty or contain the file path
	// The implementation should define this behavior
	chunk := result.Chunks[0]
	// Either empty or file path is acceptable for file-level signature
	if chunk.Signature != "" {
		assert.Equal(t, "script.rb", chunk.Signature, "If signature is set, should be file path")
	}
}
