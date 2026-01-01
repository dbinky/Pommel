package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/pommel-dev/pommel/internal/models"
)

// FTSResult represents a result from full-text search.
type FTSResult struct {
	ChunkID string
	Score   float64 // BM25 score (normalized: higher is better)
}

// CreateFTSTable creates the FTS5 virtual table for full-text search.
// Uses porter tokenizer for stemming and unicode61 for unicode support.
func (db *DB) CreateFTSTable(ctx context.Context) error {
	_, err := db.Exec(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
			chunk_id UNINDEXED,
			content,
			name,
			file_path,
			tokenize='porter unicode61'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}
	return nil
}

// FTSTableExists checks if the FTS table exists.
func (db *DB) FTSTableExists(ctx context.Context) (bool, error) {
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='chunks_fts'
	`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check FTS table existence: %w", err)
	}
	return count > 0, nil
}

// EnsureFTSTable creates the FTS table if it doesn't exist.
// Returns true if the table was created, false if it already existed.
func (db *DB) EnsureFTSTable(ctx context.Context) (bool, error) {
	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := db.CreateFTSTable(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// InsertFTSEntry adds a chunk to the FTS index.
// If an entry with the same chunk_id exists, it will be replaced.
func (db *DB) InsertFTSEntry(ctx context.Context, chunk *models.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk is required")
	}
	if chunk.ID == "" {
		return fmt.Errorf("chunk ID is required")
	}

	// Check if FTS table exists
	exists, err := db.FTSTableExists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("FTS table does not exist")
	}

	// FTS5 doesn't support INSERT OR REPLACE well, so delete first if exists
	_, _ = db.Exec(ctx, `DELETE FROM chunks_fts WHERE chunk_id = ?`, chunk.ID)

	// Insert the new entry
	_, err = db.Exec(ctx, `
		INSERT INTO chunks_fts (chunk_id, content, name, file_path)
		VALUES (?, ?, ?, ?)
	`, chunk.ID, chunk.Content, chunk.Name, chunk.FilePath)
	if err != nil {
		return fmt.Errorf("failed to insert FTS entry: %w", err)
	}
	return nil
}

// UpdateFTSEntry updates an existing FTS entry.
func (db *DB) UpdateFTSEntry(ctx context.Context, chunk *models.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk is required")
	}
	if chunk.ID == "" {
		return fmt.Errorf("chunk ID is required")
	}

	// Check if entry exists
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM chunks_fts WHERE chunk_id = ?`, chunk.ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check FTS entry existence: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("FTS entry not found for chunk ID: %s", chunk.ID)
	}

	// Delete old entry and insert new one (FTS5 doesn't support UPDATE well)
	if err := db.DeleteFTSEntry(ctx, chunk.ID); err != nil {
		return fmt.Errorf("failed to delete old FTS entry: %w", err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO chunks_fts (chunk_id, content, name, file_path)
		VALUES (?, ?, ?, ?)
	`, chunk.ID, chunk.Content, chunk.Name, chunk.FilePath)
	if err != nil {
		return fmt.Errorf("failed to insert updated FTS entry: %w", err)
	}
	return nil
}

// DeleteFTSEntry removes a chunk from the FTS index.
func (db *DB) DeleteFTSEntry(ctx context.Context, chunkID string) error {
	if chunkID == "" {
		return fmt.Errorf("chunk ID is required")
	}

	_, err := db.Exec(ctx, `DELETE FROM chunks_fts WHERE chunk_id = ?`, chunkID)
	if err != nil {
		return fmt.Errorf("failed to delete FTS entry: %w", err)
	}
	return nil
}

// DeleteFTSEntriesByFile removes all FTS entries for a given file path.
func (db *DB) DeleteFTSEntriesByFile(ctx context.Context, filePath string) error {
	_, err := db.Exec(ctx, `DELETE FROM chunks_fts WHERE file_path = ?`, filePath)
	if err != nil {
		return fmt.Errorf("failed to delete FTS entries by file: %w", err)
	}
	return nil
}

// FTSSearch performs a full-text search and returns matching chunk IDs with scores.
// The query can use FTS5 syntax (AND, OR, NOT, prefix*, "phrases").
func (db *DB) FTSSearch(ctx context.Context, query string, limit int) ([]FTSResult, error) {
	// Handle empty query
	if strings.TrimSpace(query) == "" {
		return []FTSResult{}, nil
	}

	// Handle zero or negative limit
	if limit <= 0 {
		return []FTSResult{}, nil
	}

	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Escape special characters for safety, but preserve FTS5 operators
	safeQuery := sanitizeFTSQuery(query)
	if safeQuery == "" {
		return []FTSResult{}, nil
	}

	// BM25 returns negative scores where more negative = more relevant
	// We negate to make higher scores = more relevant
	rows, err := db.Query(ctx, `
		SELECT chunk_id, -bm25(chunks_fts) as score
		FROM chunks_fts
		WHERE chunks_fts MATCH ?
		ORDER BY score DESC
		LIMIT ?
	`, safeQuery, limit)
	if err != nil {
		// Check if it's a context error
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("failed to search FTS: %w", err)
	}
	defer rows.Close()

	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		if err := rows.Scan(&r.ChunkID, &r.Score); err != nil {
			return nil, fmt.Errorf("failed to scan FTS result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating FTS results: %w", err)
	}

	return results, nil
}

// sanitizeFTSQuery makes a query safe for FTS5 while preserving useful syntax.
func sanitizeFTSQuery(query string) string {
	// FTS5 special characters that need careful handling
	// We'll allow: AND, OR, NOT, *, "phrases"
	// We'll escape or remove: ( ) { } [ ] ^ ~ : @ # $ % &

	result := query

	// Remove dangerous characters that could cause SQL issues
	dangerousChars := []string{"{", "}", "[", "]", "^", "~", "@", "#", "$", "%", "&", ";", "--", "/*", "*/"}
	for _, char := range dangerousChars {
		result = strings.ReplaceAll(result, char, " ")
	}

	// Handle parentheses - remove them for safety
	result = strings.ReplaceAll(result, "(", " ")
	result = strings.ReplaceAll(result, ")", " ")

	// Collapse multiple spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	return strings.TrimSpace(result)
}

// PopulateFTSFromChunks rebuilds the FTS index from the chunks table.
// This clears existing FTS entries and repopulates from the chunks table.
// Returns the number of entries populated.
func (db *DB) PopulateFTSFromChunks(ctx context.Context) (int, error) {
	// Check context
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// Clear existing FTS entries
	_, err := db.Exec(ctx, `DELETE FROM chunks_fts`)
	if err != nil {
		return 0, fmt.Errorf("failed to clear FTS table: %w", err)
	}

	// Query all chunks with their file paths
	rows, err := db.Query(ctx, `
		SELECT c.id, c.content, c.name, f.path
		FROM chunks c
		JOIN files f ON c.file_id = f.id
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		// Check context periodically
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}

		var id, content, name, path string
		if err := rows.Scan(&id, &content, &name, &path); err != nil {
			return count, fmt.Errorf("failed to scan chunk: %w", err)
		}

		_, err := db.Exec(ctx, `
			INSERT INTO chunks_fts (chunk_id, content, name, file_path)
			VALUES (?, ?, ?, ?)
		`, id, content, name, path)
		if err != nil {
			return count, fmt.Errorf("failed to insert FTS entry for chunk %s: %w", id, err)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("error iterating chunks: %w", err)
	}

	return count, nil
}

// ClearFTS removes all entries from the FTS table.
func (db *DB) ClearFTS(ctx context.Context) error {
	_, err := db.Exec(ctx, `DELETE FROM chunks_fts`)
	if err != nil {
		return fmt.Errorf("failed to clear FTS table: %w", err)
	}
	return nil
}
