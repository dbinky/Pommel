# Chunk Splitting Phase 2: Splitting Logic

**Parent Design:** [2026-01-04-chunk-splitting-design.md](./2026-01-04-chunk-splitting-design.md)
**Prerequisite:** [Phase 1: Foundation](./chunk-splitting-phase-1.md)
**Phase:** 2 of 3
**Goal:** Implement the splitter that handles oversized chunks at file, class, and method levels

## TDD Requirements

**STRICT TDD PROCESS - Follow for every task:**

1. **Write tests FIRST** - No implementation code until tests exist
2. **Run tests, verify they FAIL** - Red phase confirms tests are valid
3. **Write minimal implementation** - Only enough to pass tests
4. **Run tests, verify they PASS** - Green phase confirms implementation
5. **Refactor if needed** - Clean up while keeping tests green
6. **Commit** - Small, atomic commits after each green phase

**Test Categories Required for Each Component:**

| Category | Description | Example |
|----------|-------------|---------|
| Happy Path | Normal, expected usage | Split 50KB method into 3 parts |
| Success | Valid inputs that should work | Small chunk passes through unchanged |
| Failure | Invalid inputs handled gracefully | Empty content, nil chunk |
| Error | Error conditions properly reported | Malformed input |
| Edge Cases | Boundary conditions | Exactly at token limit, 1 byte over |

---

## Task 2.1: Splitter Core Types

### Files to Create
- `internal/chunker/splitter.go`
- `internal/chunker/splitter_test.go`

### Test Specifications

Write these tests FIRST in `internal/chunker/splitter_test.go`:

```go
package chunker

import (
	"strings"
	"testing"

	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Splitter Construction Tests
// =============================================================================

// --- Happy Path Tests ---

func TestNewSplitter_DefaultValues(t *testing.T) {
	s := NewSplitter(8000)

	assert.Equal(t, 8000, s.maxTokens)
	assert.Equal(t, 256, s.overlapTokens)
}

func TestNewSplitter_DifferentProviderLimits(t *testing.T) {
	tests := []struct {
		name      string
		maxTokens int
	}{
		{"Ollama", 8000},
		{"OpenAI", 8000},
		{"Voyage", 15000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSplitter(tt.maxTokens)
			assert.Equal(t, tt.maxTokens, s.maxTokens)
		})
	}
}

// --- Edge Case Tests ---

func TestNewSplitter_ZeroMaxTokens(t *testing.T) {
	s := NewSplitter(0)
	// Should handle gracefully - maybe use a minimum
	assert.True(t, s.maxTokens >= 0)
}

func TestNewSplitter_NegativeMaxTokens(t *testing.T) {
	s := NewSplitter(-100)
	// Should handle gracefully
	assert.True(t, s.maxTokens >= 0)
}

// =============================================================================
// SplitChunk Type Tests
// =============================================================================

func TestSplitChunk_ToChunk_NonSplit(t *testing.T) {
	original := &models.Chunk{
		ID:        "original-1",
		FileID:    "file-1",
		FilePath:  "src/main.go",
		Name:      "main",
		Level:     models.ChunkLevelMethod,
		StartLine: 10,
		EndLine:   20,
	}

	split := SplitChunk{
		Content:   "func main() {}",
		StartLine: 10,
		EndLine:   20,
		Index:     0,
		IsPartial: false,
		ParentID:  "",
	}

	chunk := split.ToChunk(original)

	assert.NotEqual(t, original.ID, chunk.ID) // New ID generated
	assert.Equal(t, "file-1", chunk.FileID)
	assert.Equal(t, "src/main.go", chunk.FilePath)
	assert.Equal(t, "main", chunk.Name)
	assert.Equal(t, 10, chunk.StartLine)
	assert.Equal(t, 20, chunk.EndLine)
	assert.Equal(t, "func main() {}", chunk.Content)
	assert.False(t, chunk.IsPartial)
	assert.Equal(t, "", chunk.ParentChunkID)
}

func TestSplitChunk_ToChunk_Split(t *testing.T) {
	original := &models.Chunk{
		ID:        "original-1",
		FileID:    "file-1",
		FilePath:  "src/main.go",
		Name:      "bigMethod",
		Level:     models.ChunkLevelMethod,
		StartLine: 10,
		EndLine:   100,
	}

	split := SplitChunk{
		Content:   "first part of method",
		StartLine: 10,
		EndLine:   50,
		Index:     0,
		IsPartial: true,
		ParentID:  "original-1",
	}

	chunk := split.ToChunk(original)

	assert.True(t, chunk.IsPartial)
	assert.Equal(t, "original-1", chunk.ParentChunkID)
	assert.Equal(t, 0, chunk.ChunkIndex)
	assert.Equal(t, 10, chunk.StartLine)
	assert.Equal(t, 50, chunk.EndLine)
}

func TestSplitChunk_ToChunk_PreservesLevel(t *testing.T) {
	levels := []models.ChunkLevel{
		models.ChunkLevelFile,
		models.ChunkLevelClass,
		models.ChunkLevelMethod,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			original := &models.Chunk{Level: level}
			split := SplitChunk{Content: "test"}
			chunk := split.ToChunk(original)
			assert.Equal(t, level, chunk.Level)
		})
	}
}
```

### Implementation

```go
package chunker

import (
	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
	"github.com/google/uuid"
)

const (
	// MaxFileSizeForFileChunk is the maximum file size to create a file-level chunk
	MaxFileSizeForFileChunk = 100 * 1024 // 100KB

	// OverlapTokens is the number of tokens to overlap between splits
	OverlapTokens = 256

	// MinMaxTokens is the minimum allowed maxTokens value
	MinMaxTokens = 100
)

// SplitChunk represents a chunk that may be part of a split.
type SplitChunk struct {
	Content   string
	StartLine int
	EndLine   int
	Index     int
	IsPartial bool
	ParentID  string
}

// ToChunk converts a SplitChunk back to a models.Chunk.
func (sc SplitChunk) ToChunk(original *models.Chunk) *models.Chunk {
	return &models.Chunk{
		ID:            uuid.New().String(),
		FileID:        original.FileID,
		FilePath:      original.FilePath,
		Name:          original.Name,
		Level:         original.Level,
		StartLine:     sc.StartLine,
		EndLine:       sc.EndLine,
		Content:       sc.Content,
		ParentChunkID: sc.ParentID,
		ChunkIndex:    sc.Index,
		IsPartial:     sc.IsPartial,
	}
}

// Splitter handles splitting oversized chunks.
type Splitter struct {
	maxTokens     int
	overlapTokens int
}

// NewSplitter creates a new Splitter with the given token limit.
func NewSplitter(maxTokens int) *Splitter {
	if maxTokens < MinMaxTokens {
		maxTokens = MinMaxTokens
	}
	return &Splitter{
		maxTokens:     maxTokens,
		overlapTokens: OverlapTokens,
	}
}
```

---

## Task 2.2: File-Level Chunk Handling

### Test Specifications

Add to `internal/chunker/splitter_test.go`:

```go
// =============================================================================
// HandleFileChunk Tests
// =============================================================================

// --- Happy Path Tests ---

func TestHandleFileChunk_SmallFile_PassThrough(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content:   "package main\n\nfunc main() {}\n",
		Level:     models.ChunkLevelFile,
		StartLine: 1,
		EndLine:   3,
	}

	result := s.HandleFileChunk(chunk, len(chunk.Content))

	require.NotNil(t, result)
	assert.Equal(t, chunk.Content, result.Content)
	assert.False(t, result.IsPartial)
}

func TestHandleFileChunk_MediumFile_WithinTokenLimit(t *testing.T) {
	s := NewSplitter(8000)
	// Create content that's under token limit but decent size
	content := strings.Repeat("func foo() { return 1 }\n", 500) // ~12KB
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelFile,
		StartLine: 1,
		EndLine:   500,
	}

	result := s.HandleFileChunk(chunk, len(content))

	require.NotNil(t, result)
	assert.Equal(t, content, result.Content)
	assert.False(t, result.IsPartial)
}

// --- Success Tests: Truncation ---

func TestHandleFileChunk_OversizedContent_Truncates(t *testing.T) {
	s := NewSplitter(1000) // Low limit for testing
	// Create content that exceeds token limit
	content := strings.Repeat("x", 5000) // ~1428 tokens, over 1000 limit
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelFile,
		StartLine: 1,
		EndLine:   1,
	}

	result := s.HandleFileChunk(chunk, len(content))

	require.NotNil(t, result)
	assert.True(t, result.IsPartial)
	assert.Less(t, len(result.Content), len(content))
	assert.Contains(t, result.Content, "[truncated]")
}

func TestHandleFileChunk_TruncatesAtLineBreak(t *testing.T) {
	s := NewSplitter(100) // Very low limit
	content := "line1\nline2\nline3\nline4\nline5\n" + strings.Repeat("x", 500)
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelFile,
		StartLine: 1,
		EndLine:   10,
	}

	result := s.HandleFileChunk(chunk, len(content))

	require.NotNil(t, result)
	assert.True(t, result.IsPartial)
	// Should end at a line break, not mid-line
	lines := strings.Split(result.Content, "\n")
	lastLine := lines[len(lines)-1]
	assert.True(t, lastLine == "// ... [truncated]" || strings.HasSuffix(result.Content, "[truncated]"))
}

// --- Success Tests: Skip Very Large Files ---

func TestHandleFileChunk_VeryLargeFile_ReturnsNil(t *testing.T) {
	s := NewSplitter(8000)
	content := strings.Repeat("x", 50*1024) // 50KB content
	chunk := &models.Chunk{
		Content: content,
		Level:   models.ChunkLevelFile,
	}

	// File size is 150KB, over 100KB limit
	result := s.HandleFileChunk(chunk, 150*1024)

	assert.Nil(t, result, "Should skip file-level chunk for files > 100KB")
}

func TestHandleFileChunk_ExactlyAtSizeLimit_NotSkipped(t *testing.T) {
	s := NewSplitter(8000)
	content := "small content"
	chunk := &models.Chunk{
		Content: content,
		Level:   models.ChunkLevelFile,
	}

	// Exactly at 100KB limit
	result := s.HandleFileChunk(chunk, 100*1024)

	require.NotNil(t, result, "Should not skip file at exactly 100KB")
}

func TestHandleFileChunk_JustOverSizeLimit_Skipped(t *testing.T) {
	s := NewSplitter(8000)
	content := "small content"
	chunk := &models.Chunk{
		Content: content,
		Level:   models.ChunkLevelFile,
	}

	// 1 byte over 100KB limit
	result := s.HandleFileChunk(chunk, 100*1024+1)

	assert.Nil(t, result, "Should skip file just over 100KB")
}

// --- Edge Case Tests ---

func TestHandleFileChunk_EmptyContent(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content: "",
		Level:   models.ChunkLevelFile,
	}

	result := s.HandleFileChunk(chunk, 0)

	require.NotNil(t, result)
	assert.Equal(t, "", result.Content)
	assert.False(t, result.IsPartial)
}

func TestHandleFileChunk_NilChunk(t *testing.T) {
	s := NewSplitter(8000)
	result := s.HandleFileChunk(nil, 1000)
	assert.Nil(t, result)
}

func TestHandleFileChunk_ZeroFileSize(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content: "some content",
		Level:   models.ChunkLevelFile,
	}

	result := s.HandleFileChunk(chunk, 0)
	require.NotNil(t, result)
}

func TestHandleFileChunk_SingleLineContent(t *testing.T) {
	s := NewSplitter(100) // Low limit
	content := strings.Repeat("x", 500) // Single line, will be truncated
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelFile,
		StartLine: 1,
		EndLine:   1,
	}

	result := s.HandleFileChunk(chunk, len(content))

	require.NotNil(t, result)
	assert.True(t, result.IsPartial)
}

func TestHandleFileChunk_PreservesLineNumbers(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content:   "content",
		Level:     models.ChunkLevelFile,
		StartLine: 5,
		EndLine:   10,
	}

	result := s.HandleFileChunk(chunk, len(chunk.Content))

	assert.Equal(t, 5, result.StartLine)
	assert.Equal(t, 10, result.EndLine)
}
```

### Implementation

Add to `internal/chunker/splitter.go`:

```go
// HandleFileChunk processes a file-level chunk.
// Returns nil if the file is too large (>100KB) to create a file-level chunk.
// Truncates content if it exceeds the token limit.
func (s *Splitter) HandleFileChunk(chunk *models.Chunk, fileSize int) *SplitChunk {
	if chunk == nil {
		return nil
	}

	// Skip file-level chunk for very large files
	if fileSize > MaxFileSizeForFileChunk {
		return nil
	}

	content := chunk.Content
	tokens := embedder.EstimateTokens(content)

	// If within limit, return as-is
	if tokens <= s.maxTokens {
		return &SplitChunk{
			Content:   content,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Index:     0,
			IsPartial: false,
		}
	}

	// Truncate to fit within token limit
	maxChars := embedder.MaxCharsForTokens(s.maxTokens)
	if maxChars >= len(content) {
		maxChars = len(content) - 1
	}

	truncated := content[:maxChars]

	// Find last complete line
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > 0 {
		truncated = truncated[:lastNewline]
	}

	// Add truncation marker
	truncated = truncated + "\n// ... [truncated]"

	return &SplitChunk{
		Content:   truncated,
		StartLine: chunk.StartLine,
		EndLine:   chunk.EndLine,
		Index:     0,
		IsPartial: true,
	}
}
```

---

## Task 2.3: Class-Level Chunk Handling

### Test Specifications

Add to `internal/chunker/splitter_test.go`:

```go
// =============================================================================
// HandleClassChunk Tests
// =============================================================================

// --- Happy Path Tests ---

func TestHandleClassChunk_SmallClass_PassThrough(t *testing.T) {
	s := NewSplitter(8000)
	content := `class Calculator:
    def __init__(self):
        self.value = 0

    def add(self, x):
        self.value += x
`
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelClass,
		StartLine: 1,
		EndLine:   7,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	assert.Equal(t, content, result.Content)
	assert.False(t, result.IsPartial)
}

// --- Success Tests: Truncation ---

func TestHandleClassChunk_LargeClass_Truncates(t *testing.T) {
	s := NewSplitter(500) // Low limit for testing
	// Create large class content
	methods := strings.Repeat("    def method(self):\n        pass\n\n", 100)
	content := "class BigClass:\n" + methods
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelClass,
		StartLine: 1,
		EndLine:   300,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	assert.True(t, result.IsPartial)
	assert.Less(t, len(result.Content), len(content))
	assert.Contains(t, result.Content, "[truncated]")
}

func TestHandleClassChunk_PreservesClassSignature(t *testing.T) {
	s := NewSplitter(200) // Very low limit
	content := `type DatabaseConnection struct {
	host     string
	port     int
	username string
	password string
}

func (db *DatabaseConnection) Connect() error {
	// lots of code here
` + strings.Repeat("    // more code\n", 50) + "}"

	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelClass,
		StartLine: 1,
		EndLine:   60,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	// Should preserve the struct definition at minimum
	assert.Contains(t, result.Content, "type DatabaseConnection struct")
}

// --- Edge Case Tests ---

func TestHandleClassChunk_EmptyClass(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content:   "class Empty:\n    pass\n",
		Level:     models.ChunkLevelClass,
		StartLine: 1,
		EndLine:   2,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	assert.False(t, result.IsPartial)
}

func TestHandleClassChunk_NilChunk(t *testing.T) {
	s := NewSplitter(8000)
	result := s.HandleClassChunk(nil)
	assert.Nil(t, result)
}

func TestHandleClassChunk_EmptyContent(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content: "",
		Level:   models.ChunkLevelClass,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	assert.Equal(t, "", result.Content)
}

func TestHandleClassChunk_SingleLineClass(t *testing.T) {
	s := NewSplitter(100) // Low limit
	content := "class Foo { " + strings.Repeat("x", 500) + " }"
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelClass,
		StartLine: 1,
		EndLine:   1,
	}

	result := s.HandleClassChunk(chunk)

	require.NotNil(t, result)
	assert.True(t, result.IsPartial)
}

func TestHandleClassChunk_PreservesLineNumbers(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content:   "class Foo {}",
		Level:     models.ChunkLevelClass,
		StartLine: 15,
		EndLine:   25,
	}

	result := s.HandleClassChunk(chunk)

	assert.Equal(t, 15, result.StartLine)
	assert.Equal(t, 25, result.EndLine)
}
```

### Implementation

Add to `internal/chunker/splitter.go`:

```go
// HandleClassChunk processes a class-level chunk.
// Truncates content if it exceeds the token limit, preserving the class signature.
func (s *Splitter) HandleClassChunk(chunk *models.Chunk) *SplitChunk {
	if chunk == nil {
		return nil
	}

	content := chunk.Content
	tokens := embedder.EstimateTokens(content)

	// If within limit, return as-is
	if tokens <= s.maxTokens {
		return &SplitChunk{
			Content:   content,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Index:     0,
			IsPartial: false,
		}
	}

	// Truncate to fit within token limit
	maxChars := embedder.MaxCharsForTokens(s.maxTokens)
	if maxChars >= len(content) {
		maxChars = len(content) - 1
	}

	truncated := content[:maxChars]

	// Find last complete line
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > 0 {
		truncated = truncated[:lastNewline]
	}

	// Add truncation marker
	truncated = truncated + "\n// ... [truncated]"

	return &SplitChunk{
		Content:   truncated,
		StartLine: chunk.StartLine,
		EndLine:   chunk.EndLine,
		Index:     0,
		IsPartial: true,
	}
}
```

---

## Task 2.4: Method-Level Chunk Splitting

### Test Specifications

Add to `internal/chunker/splitter_test.go`:

```go
// =============================================================================
// SplitMethod Tests
// =============================================================================

// --- Happy Path Tests ---

func TestSplitMethod_SmallMethod_NoSplit(t *testing.T) {
	s := NewSplitter(8000)
	content := `func add(a, b int) int {
	return a + b
}`
	chunk := &models.Chunk{
		ID:        "chunk-1",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   3,
	}

	splits := s.SplitMethod(chunk)

	require.Len(t, splits, 1)
	assert.Equal(t, content, splits[0].Content)
	assert.False(t, splits[0].IsPartial)
	assert.Equal(t, "", splits[0].ParentID)
	assert.Equal(t, 0, splits[0].Index)
}

func TestSplitMethod_LargeMethod_CreatesSplits(t *testing.T) {
	s := NewSplitter(500) // Low limit to force splitting
	// Create large method content
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "    x := doSomething()\n"
	}
	content := "func bigMethod() {\n" + strings.Join(lines, "") + "}\n"

	chunk := &models.Chunk{
		ID:        "chunk-1",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   102,
	}

	splits := s.SplitMethod(chunk)

	assert.Greater(t, len(splits), 1, "Should create multiple splits")
	for i, split := range splits {
		assert.True(t, split.IsPartial)
		assert.Equal(t, "chunk-1", split.ParentID)
		assert.Equal(t, i, split.Index)
	}
}

func TestSplitMethod_SplitsHaveOverlap(t *testing.T) {
	s := NewSplitter(300) // Low limit
	// Create content that will be split
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("line %d: some code here\n", i))
	}
	content := strings.Join(lines, "")

	chunk := &models.Chunk{
		ID:        "chunk-1",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   50,
	}

	splits := s.SplitMethod(chunk)

	require.Greater(t, len(splits), 1)

	// Check that consecutive splits have overlapping content
	for i := 0; i < len(splits)-1; i++ {
		current := splits[i]
		next := splits[i+1]

		// Next split should start before current ends (overlap)
		assert.LessOrEqual(t, next.StartLine, current.EndLine,
			"Split %d and %d should overlap", i, i+1)
	}
}

func TestSplitMethod_CoverageComplete(t *testing.T) {
	s := NewSplitter(400) // Force splitting
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("line%d\n", i))
	}
	content := strings.Join(lines, "")

	chunk := &models.Chunk{
		ID:        "chunk-1",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   60,
	}

	splits := s.SplitMethod(chunk)

	// Verify all content is covered
	allContent := ""
	for _, split := range splits {
		allContent += split.Content
	}

	// Original content should be fully represented (with possible overlap duplicates)
	for _, line := range lines {
		assert.Contains(t, allContent, strings.TrimSpace(line),
			"All original lines should be in splits")
	}
}

// --- Success Tests ---

func TestSplitMethod_ExactlyAtLimit_NoSplit(t *testing.T) {
	s := NewSplitter(100)
	// Create content exactly at 100 tokens (~350 chars)
	content := strings.Repeat("x", 350)
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   1,
	}

	splits := s.SplitMethod(chunk)

	assert.Len(t, splits, 1)
	assert.False(t, splits[0].IsPartial)
}

func TestSplitMethod_JustOverLimit_CreatesSplits(t *testing.T) {
	s := NewSplitter(100)
	// Create content just over 100 tokens
	content := strings.Repeat("x\n", 200) // ~400 chars, ~114 tokens
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   200,
	}

	splits := s.SplitMethod(chunk)

	assert.Greater(t, len(splits), 1)
}

// --- Edge Case Tests ---

func TestSplitMethod_NilChunk(t *testing.T) {
	s := NewSplitter(8000)
	splits := s.SplitMethod(nil)
	assert.Empty(t, splits)
}

func TestSplitMethod_EmptyContent(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content: "",
		Level:   models.ChunkLevelMethod,
	}

	splits := s.SplitMethod(chunk)

	assert.Len(t, splits, 1)
	assert.Equal(t, "", splits[0].Content)
}

func TestSplitMethod_SingleCharacter(t *testing.T) {
	s := NewSplitter(8000)
	chunk := &models.Chunk{
		Content:   "x",
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   1,
	}

	splits := s.SplitMethod(chunk)

	assert.Len(t, splits, 1)
	assert.Equal(t, "x", splits[0].Content)
}

func TestSplitMethod_VeryLongSingleLine(t *testing.T) {
	s := NewSplitter(500)
	// Single line that exceeds token limit
	content := strings.Repeat("x", 5000)
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   1,
	}

	splits := s.SplitMethod(chunk)

	assert.Greater(t, len(splits), 1)
	// Should handle gracefully even without line breaks
}

func TestSplitMethod_LineNumbersCorrect(t *testing.T) {
	s := NewSplitter(200) // Force splitting
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d\n", i))
	}
	content := strings.Join(lines, "")

	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 100, // Starting at line 100
		EndLine:   129,
	}

	splits := s.SplitMethod(chunk)

	// First split should start at original start line
	assert.Equal(t, 100, splits[0].StartLine)

	// Last split should end at or before original end line
	lastSplit := splits[len(splits)-1]
	assert.LessOrEqual(t, lastSplit.EndLine, 129)

	// Line numbers should be sequential
	for i := 0; i < len(splits)-1; i++ {
		current := splits[i]
		next := splits[i+1]
		assert.LessOrEqual(t, next.StartLine, current.EndLine+1)
	}
}

func TestSplitMethod_PreservesOriginalID(t *testing.T) {
	s := NewSplitter(200)
	content := strings.Repeat("line\n", 50)
	chunk := &models.Chunk{
		ID:        "original-method-123",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   50,
	}

	splits := s.SplitMethod(chunk)

	for _, split := range splits {
		if split.IsPartial {
			assert.Equal(t, "original-method-123", split.ParentID)
		}
	}
}

func TestSplitMethod_IndexesSequential(t *testing.T) {
	s := NewSplitter(200)
	content := strings.Repeat("line of code here\n", 40)
	chunk := &models.Chunk{
		ID:        "chunk-1",
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   40,
	}

	splits := s.SplitMethod(chunk)

	for i, split := range splits {
		assert.Equal(t, i, split.Index, "Index should be sequential")
	}
}

// --- Failure Scenario Tests ---

func TestSplitMethod_ContentExceedsSafeLimit(t *testing.T) {
	s := NewSplitter(50) // Very small limit
	// Content much larger than limit
	content := strings.Repeat("word ", 500)
	chunk := &models.Chunk{
		Content:   content,
		Level:     models.ChunkLevelMethod,
		StartLine: 1,
		EndLine:   1,
	}

	// Should not panic, should handle gracefully
	splits := s.SplitMethod(chunk)

	assert.NotEmpty(t, splits)
	// All splits should be within token limit
	for _, split := range splits {
		tokens := embedder.EstimateTokens(split.Content)
		assert.LessOrEqual(t, tokens, s.maxTokens+s.overlapTokens,
			"Split content should be within limit")
	}
}
```

### Implementation

Add to `internal/chunker/splitter.go`:

```go
// SplitMethod splits a method-level chunk into overlapping pieces.
// Returns a single-element slice if the chunk fits within the token limit.
func (s *Splitter) SplitMethod(chunk *models.Chunk) []SplitChunk {
	if chunk == nil {
		return nil
	}

	content := chunk.Content
	tokens := embedder.EstimateTokens(content)

	// If within limit, return as single chunk
	if tokens <= s.maxTokens {
		return []SplitChunk{{
			Content:   content,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Index:     0,
			IsPartial: false,
			ParentID:  "",
		}}
	}

	// Split into overlapping windows
	return s.splitAtBoundaries(chunk)
}

// splitAtBoundaries splits content at line boundaries with overlap.
func (s *Splitter) splitAtBoundaries(chunk *models.Chunk) []SplitChunk {
	content := chunk.Content
	lines := strings.Split(content, "\n")

	// Target size per split (accounting for overlap)
	targetTokens := s.maxTokens - s.overlapTokens
	if targetTokens < 100 {
		targetTokens = 100
	}

	var splits []SplitChunk
	currentStart := 0
	splitIndex := 0

	for currentStart < len(lines) {
		// Find end position that fits within token limit
		currentEnd := s.findSplitEnd(lines, currentStart, targetTokens)

		// Build split content
		splitLines := lines[currentStart:currentEnd]
		splitContent := strings.Join(splitLines, "\n")

		// Calculate line numbers
		startLine := chunk.StartLine + currentStart
		endLine := chunk.StartLine + currentEnd - 1
		if endLine < startLine {
			endLine = startLine
		}

		splits = append(splits, SplitChunk{
			Content:   splitContent,
			StartLine: startLine,
			EndLine:   endLine,
			Index:     splitIndex,
			IsPartial: true,
			ParentID:  chunk.ID,
		})

		// Calculate overlap for next split
		overlapLines := s.calculateOverlapLines(lines, currentEnd, s.overlapTokens)
		nextStart := currentEnd - overlapLines

		// Ensure progress
		if nextStart <= currentStart {
			nextStart = currentEnd
		}
		if nextStart >= len(lines) {
			break
		}

		currentStart = nextStart
		splitIndex++
	}

	return splits
}

// findSplitEnd finds the best line to end a split at.
func (s *Splitter) findSplitEnd(lines []string, start, targetTokens int) int {
	currentTokens := 0
	end := start

	for i := start; i < len(lines); i++ {
		lineTokens := embedder.EstimateTokens(lines[i])

		// If adding this line exceeds target, stop (unless we haven't added anything)
		if currentTokens+lineTokens > targetTokens && i > start {
			break
		}

		currentTokens += lineTokens
		end = i + 1
	}

	// Ensure we include at least one line
	if end <= start && start < len(lines) {
		end = start + 1
	}

	return end
}

// calculateOverlapLines calculates how many lines to overlap for the given token count.
func (s *Splitter) calculateOverlapLines(lines []string, fromEnd, targetTokens int) int {
	tokens := 0
	count := 0

	for i := fromEnd - 1; i >= 0 && tokens < targetTokens; i-- {
		tokens += embedder.EstimateTokens(lines[i])
		count++
	}

	return count
}
```

---

## Task 2.5: Chunker Integration

### Test Specifications

Add integration tests to verify the chunker uses the splitter:

```go
// =============================================================================
// Chunker Integration Tests
// =============================================================================

func TestChunker_SkipsMinifiedFiles(t *testing.T) {
	// Create a minified file
	content := []byte(strings.Repeat("x", 20*1024)) // 20KB single line

	chunker := NewChunker(8000)
	chunks, err := chunker.ChunkFile("bundle.min.js", content)

	require.NoError(t, err)
	assert.Empty(t, chunks, "Should skip minified files")
}

func TestChunker_LargeFileNoFileChunk(t *testing.T) {
	// Create a large file (>100KB)
	var lines []string
	for i := 0; i < 5000; i++ {
		lines = append(lines, fmt.Sprintf("func method%d() {}\n", i))
	}
	content := []byte(strings.Join(lines, ""))

	chunker := NewChunker(8000)
	chunks, err := chunker.ChunkFile("large.go", content)

	require.NoError(t, err)

	// Should have method chunks but no file chunk
	hasFileChunk := false
	for _, chunk := range chunks {
		if chunk.Level == models.ChunkLevelFile {
			hasFileChunk = true
		}
	}
	assert.False(t, hasFileChunk, "Should not have file-level chunk for large files")
}

func TestChunker_LargeMethodGetsSplit(t *testing.T) {
	// Create a file with one very large method
	methodContent := strings.Repeat("    x := 1\n", 1000)
	content := []byte("package main\n\nfunc bigMethod() {\n" + methodContent + "}\n")

	chunker := NewChunker(500) // Low limit to force splitting
	chunks, err := chunker.ChunkFile("main.go", content)

	require.NoError(t, err)

	// Find method chunks
	var methodChunks []*models.Chunk
	for _, chunk := range chunks {
		if chunk.Level == models.ChunkLevelMethod {
			methodChunks = append(methodChunks, chunk)
		}
	}

	assert.Greater(t, len(methodChunks), 1, "Large method should be split")

	// Verify split metadata
	for _, chunk := range methodChunks {
		if chunk.IsPartial {
			assert.NotEmpty(t, chunk.ParentChunkID)
		}
	}
}
```

### Implementation Notes

Modify `internal/chunker/chunker.go` to integrate the splitter. The chunker should:

1. Check `IsMinified()` before processing
2. Create a `Splitter` with the provider's max context tokens
3. Route chunks through appropriate handler based on level
4. Convert `SplitChunk` results back to `*models.Chunk`

---

## Verification Checklist

```bash
# Run all Phase 2 tests
go test -v -race ./internal/chunker/... -run "Splitter|Split|Handle"

# Run specific test groups
go test -v ./internal/chunker/ -run "TestNewSplitter"
go test -v ./internal/chunker/ -run "TestHandleFileChunk"
go test -v ./internal/chunker/ -run "TestHandleClassChunk"
go test -v ./internal/chunker/ -run "TestSplitMethod"

# Run integration tests
go test -v ./internal/chunker/ -run "TestChunker_"

# Check test coverage
go test -coverprofile=coverage.out ./internal/chunker/...
go tool cover -html=coverage.out
```

## Acceptance Criteria

| Criterion | Test Coverage |
|-----------|---------------|
| Splitter creates correctly | ✅ TestNewSplitter_* |
| File chunks pass through when small | ✅ TestHandleFileChunk_SmallFile_PassThrough |
| File chunks truncate when oversized | ✅ TestHandleFileChunk_OversizedContent_Truncates |
| Very large files skip file chunk | ✅ TestHandleFileChunk_VeryLargeFile_ReturnsNil |
| Class chunks pass through when small | ✅ TestHandleClassChunk_SmallClass_PassThrough |
| Class chunks truncate when oversized | ✅ TestHandleClassChunk_LargeClass_Truncates |
| Small methods not split | ✅ TestSplitMethod_SmallMethod_NoSplit |
| Large methods split with overlap | ✅ TestSplitMethod_LargeMethod_CreatesSplits |
| Splits have correct parent IDs | ✅ TestSplitMethod_PreservesOriginalID |
| Splits have sequential indexes | ✅ TestSplitMethod_IndexesSequential |
| All content covered by splits | ✅ TestSplitMethod_CoverageComplete |
| Minified files skipped | ✅ TestChunker_SkipsMinifiedFiles |
| Nil/empty handled gracefully | ✅ Multiple nil/empty tests |

## Next Phase

After all Phase 2 tests pass, proceed to [Phase 3: Search Integration](./chunk-splitting-phase-3.md).
