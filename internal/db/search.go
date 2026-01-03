package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/pommel-dev/pommel/internal/models"
)

// ErrChunkNotFound is returned when a chunk cannot be found.
var ErrChunkNotFound = errors.New("chunk not found")

// SearchOptions specifies parameters for semantic search.
type SearchOptions struct {
	Embedding  []float32 // Query embedding vector
	Limit      int       // Maximum number of results to return
	Levels     []string  // Filter by chunk levels (e.g., "file", "method", "class")
	PathPrefix string    // Filter by file path prefix
}

// VectorResult represents a single search result with similarity distance.
type VectorResult struct {
	ChunkID  string
	Distance float32
}

// SearchChunks performs a semantic search with optional filtering by level and path.
// Results are ordered by distance (ascending - smaller is more similar).
func (db *DB) SearchChunks(ctx context.Context, opts SearchOptions) ([]VectorResult, error) {
	// Return empty slice for limit 0
	if opts.Limit <= 0 {
		return []VectorResult{}, nil
	}

	// Serialize the query embedding
	serialized, err := sqlite_vec.SerializeFloat32(opts.Embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	// Check if we need filtering
	hasLevelFilter := len(opts.Levels) > 0
	hasPathFilter := opts.PathPrefix != ""

	if !hasLevelFilter && !hasPathFilter {
		// No filtering needed, use simple vector search
		rows, err := db.Query(ctx, `
			SELECT chunk_id, distance
			FROM chunk_embeddings
			WHERE embedding MATCH ?
			ORDER BY distance
			LIMIT ?
		`, serialized, opts.Limit)
		if err != nil {
			return nil, fmt.Errorf("failed to search embeddings: %w", err)
		}
		defer rows.Close()

		return scanVectorResults(rows)
	}

	// Build a filtered query using a JOIN with the chunks and files tables
	// We need to first get matching chunk IDs, then search within those
	var whereConditions []string
	var filterArgs []any

	if hasLevelFilter {
		placeholders := make([]string, len(opts.Levels))
		for i, level := range opts.Levels {
			placeholders[i] = "?"
			filterArgs = append(filterArgs, level)
		}
		whereConditions = append(whereConditions, fmt.Sprintf("c.level IN (%s)", strings.Join(placeholders, ", ")))
	}

	if hasPathFilter {
		whereConditions = append(whereConditions, "f.path LIKE ?")
		filterArgs = append(filterArgs, opts.PathPrefix+"%")
	}

	// Get all matching chunk IDs first
	filterQuery := fmt.Sprintf(`
		SELECT c.id
		FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE %s
	`, strings.Join(whereConditions, " AND "))

	filterRows, err := db.Query(ctx, filterQuery, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to filter chunks: %w", err)
	}
	defer filterRows.Close()

	var matchingChunkIDs []string
	for filterRows.Next() {
		var id string
		if err := filterRows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan chunk ID: %w", err)
		}
		matchingChunkIDs = append(matchingChunkIDs, id)
	}

	if err := filterRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunk IDs: %w", err)
	}

	// If no chunks match the filter, return empty results
	if len(matchingChunkIDs) == 0 {
		return []VectorResult{}, nil
	}

	// Build IN clause for the embedding search
	placeholders := make([]string, len(matchingChunkIDs))
	searchArgs := make([]any, len(matchingChunkIDs)+2)
	searchArgs[0] = serialized
	searchArgs[1] = opts.Limit
	for i, id := range matchingChunkIDs {
		placeholders[i] = "?"
		searchArgs[i+2] = id
	}

	searchQuery := fmt.Sprintf(`
		SELECT chunk_id, distance
		FROM chunk_embeddings
		WHERE embedding MATCH ? AND k = ?
		  AND chunk_id IN (%s)
		ORDER BY distance
	`, strings.Join(placeholders, ", "))

	rows, err := db.Query(ctx, searchQuery, searchArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	return scanVectorResults(rows)
}

// scanVectorResults scans rows into a slice of VectorResult.
func scanVectorResults(rows *sql.Rows) ([]VectorResult, error) {
	var results []VectorResult
	for rows.Next() {
		var result VectorResult
		if err := rows.Scan(&result.ChunkID, &result.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	// Ensure we return an empty slice, not nil
	if results == nil {
		results = []VectorResult{}
	}

	return results, nil
}

// GetChunk retrieves a chunk by ID, returning ErrChunkNotFound if not found.
func (db *DB) GetChunk(ctx context.Context, id string) (*models.Chunk, error) {
	var chunk models.Chunk
	var parentID sql.NullString
	var filePath string
	var level string
	var language string

	err := db.QueryRow(ctx, `
		SELECT c.id, f.path, f.language, c.start_line, c.end_line, c.level, c.name, c.content, c.content_hash, c.parent_id
		FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE c.id = ?
	`, id).Scan(&chunk.ID, &filePath, &language, &chunk.StartLine, &chunk.EndLine, &level, &chunk.Name, &chunk.Content, &chunk.ContentHash, &parentID)

	if err == sql.ErrNoRows {
		return nil, ErrChunkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	chunk.FilePath = filePath
	chunk.Language = language
	chunk.Level = models.ChunkLevel(level)
	if parentID.Valid {
		chunk.ParentID = &parentID.String
	}

	return &chunk, nil
}
