package chunker

import (
	"strings"

	"github.com/pommel-dev/pommel/internal/embedder"
	"github.com/pommel-dev/pommel/internal/models"
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
// The caller should call SetHashes() on the returned chunk to generate the ID and ContentHash.
func (sc SplitChunk) ToChunk(original *models.Chunk) *models.Chunk {
	return &models.Chunk{
		FilePath:      original.FilePath,
		Name:          original.Name,
		Level:         original.Level,
		Language:      original.Language,
		StartLine:     sc.StartLine,
		EndLine:       sc.EndLine,
		Content:       sc.Content,
		ParentID:      original.ParentID, // Preserve chunk hierarchy
		ParentChunkID: sc.ParentID,       // Track split relationship
		ChunkIndex:    sc.Index,
		IsPartial:     sc.IsPartial,
		LastModified:  original.LastModified,
		Signature:     original.Signature,
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
