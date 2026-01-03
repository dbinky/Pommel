package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ChunkLevel represents the granularity of a code chunk
type ChunkLevel string

const (
	ChunkLevelFile    ChunkLevel = "file"
	ChunkLevelClass   ChunkLevel = "class"
	ChunkLevelSection ChunkLevel = "section"
	ChunkLevelMethod  ChunkLevel = "method"
)

// Chunk represents a semantic unit of code
type Chunk struct {
	ID           string
	FilePath     string
	StartLine    int
	EndLine      int
	Level        ChunkLevel
	Language     string
	Content      string
	ParentID     *string
	Name         string
	Signature    string
	ContentHash  string
	LastModified time.Time

	// Subproject fields for v0.2 multi-repo support
	SubprojectID   *string `json:"subproject_id,omitempty"`
	SubprojectPath *string `json:"subproject_path,omitempty"`
}

// GenerateID creates a deterministic ID for the chunk
// ID is based on file path + level + start line + end line
func (c *Chunk) GenerateID() string {
	data := fmt.Sprintf("%s:%s:%d:%d", c.FilePath, c.Level, c.StartLine, c.EndLine)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32 hex chars
}

// GenerateContentHash creates a hash of the chunk content
func (c *Chunk) GenerateContentHash() string {
	hash := sha256.Sum256([]byte(c.Content))
	return hex.EncodeToString(hash[:16])
}

// SetHashes generates and sets both ID and content hash
func (c *Chunk) SetHashes() {
	c.ID = c.GenerateID()
	c.ContentHash = c.GenerateContentHash()
}

// IsValid checks if the chunk has required fields
func (c *Chunk) IsValid() error {
	if c.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	if c.StartLine < 1 {
		return fmt.Errorf("start line must be >= 1")
	}
	if c.EndLine < c.StartLine {
		return fmt.Errorf("end line must be >= start line")
	}
	if c.Content == "" || strings.TrimSpace(c.Content) == "" {
		return fmt.Errorf("content is required")
	}
	if c.Level == "" {
		return fmt.Errorf("level is required")
	}
	return nil
}

// LineCount returns the number of lines in the chunk
func (c *Chunk) LineCount() int {
	return c.EndLine - c.StartLine + 1
}

// SetSubproject associates this chunk with a subproject.
func (c *Chunk) SetSubproject(sp *Subproject) {
	if sp == nil {
		c.SubprojectID = nil
		c.SubprojectPath = nil
		return
	}
	c.SubprojectID = &sp.ID
	c.SubprojectPath = &sp.Path
}

// HasSubproject returns true if this chunk belongs to a subproject.
func (c *Chunk) HasSubproject() bool {
	return c.SubprojectID != nil && *c.SubprojectID != ""
}

// SourceFile represents a file to be chunked
type SourceFile struct {
	Path         string
	Content      []byte
	Language     string
	LastModified time.Time
}

// ChunkResult contains chunks extracted from a file
type ChunkResult struct {
	File   *SourceFile
	Chunks []*Chunk
	Errors []error
}
