package db

import (
	"context"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// EmbeddingDimension is the dimensionality of the embedding vectors (Jina Code Embeddings).
const EmbeddingDimension = 768

// VectorSearchResult represents a single result from a similarity search.
type VectorSearchResult struct {
	ChunkID  string
	Distance float32
}

// InsertEmbedding inserts a single embedding for a chunk.
// If an embedding with the same chunk_id exists, it replaces it.
func (db *DB) InsertEmbedding(ctx context.Context, chunkID string, embedding []float32) error {
	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	// sqlite-vec's vec0 virtual table doesn't support INSERT OR REPLACE directly,
	// so we delete first then insert
	_, err = db.Exec(ctx, `DELETE FROM chunk_embeddings WHERE chunk_id = ?`, chunkID)
	if err != nil {
		return fmt.Errorf("failed to delete existing embedding: %w", err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO chunk_embeddings (chunk_id, embedding)
		VALUES (?, ?)
	`, chunkID, serialized)
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	return nil
}

// InsertEmbeddings inserts multiple embeddings in a single batch operation.
// The chunkIDs and embeddings slices must have the same length.
func (db *DB) InsertEmbeddings(ctx context.Context, chunkIDs []string, embeddings [][]float32) error {
	if len(chunkIDs) != len(embeddings) {
		return fmt.Errorf("chunkIDs and embeddings must have the same length: got %d and %d", len(chunkIDs), len(embeddings))
	}

	if len(chunkIDs) == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// sqlite-vec's vec0 virtual table doesn't support INSERT OR REPLACE directly,
	// so we delete first then insert
	deleteStmt, err := tx.PrepareContext(ctx, `DELETE FROM chunk_embeddings WHERE chunk_id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer deleteStmt.Close()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunk_embeddings (chunk_id, embedding)
		VALUES (?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insertStmt.Close()

	for i, chunkID := range chunkIDs {
		serialized, err := sqlite_vec.SerializeFloat32(embeddings[i])
		if err != nil {
			return fmt.Errorf("failed to serialize embedding for chunk %s: %w", chunkID, err)
		}

		_, err = deleteStmt.ExecContext(ctx, chunkID)
		if err != nil {
			return fmt.Errorf("failed to delete existing embedding for chunk %s: %w", chunkID, err)
		}

		_, err = insertStmt.ExecContext(ctx, chunkID, serialized)
		if err != nil {
			return fmt.Errorf("failed to insert embedding for chunk %s: %w", chunkID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteEmbedding deletes the embedding for a specific chunk.
// Returns nil if the chunk doesn't exist (idempotent).
func (db *DB) DeleteEmbedding(ctx context.Context, chunkID string) error {
	_, err := db.Exec(ctx, `DELETE FROM chunk_embeddings WHERE chunk_id = ?`, chunkID)
	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}
	return nil
}

// DeleteEmbeddingsByChunkIDs deletes embeddings for multiple chunks in a single operation.
func (db *DB) DeleteEmbeddingsByChunkIDs(ctx context.Context, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build placeholders for IN clause
	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs))
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`DELETE FROM chunk_embeddings WHERE chunk_id IN (%s)`, strings.Join(placeholders, ", "))
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// EmbeddingCount returns the total number of embeddings stored.
func (db *DB) EmbeddingCount(ctx context.Context) (int, error) {
	var count int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM chunk_embeddings`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count embeddings: %w", err)
	}
	return count, nil
}

// SearchSimilar finds the most similar embeddings to the query vector.
// Results are ordered by distance (ascending - smaller is more similar).
func (db *DB) SearchSimilar(ctx context.Context, queryEmbedding []float32, limit int) ([]VectorSearchResult, error) {
	serialized, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT chunk_id, distance
		FROM chunk_embeddings
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT ?
	`, serialized, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var result VectorSearchResult
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
		results = []VectorSearchResult{}
	}

	return results, nil
}

// SearchSimilarFiltered finds similar embeddings, but only among the specified chunk IDs.
// Results are ordered by distance (ascending - smaller is more similar).
func (db *DB) SearchSimilarFiltered(ctx context.Context, queryEmbedding []float32, limit int, chunkIDs []string) ([]VectorSearchResult, error) {
	if len(chunkIDs) == 0 {
		return []VectorSearchResult{}, nil
	}

	serialized, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs)+2) // query embedding + limit + chunk IDs
	args[0] = serialized
	args[1] = limit
	for i, id := range chunkIDs {
		placeholders[i] = "?"
		args[i+2] = id
	}

	query := fmt.Sprintf(`
		SELECT chunk_id, distance
		FROM chunk_embeddings
		WHERE embedding MATCH ? AND k = ?
		  AND chunk_id IN (%s)
		ORDER BY distance
	`, strings.Join(placeholders, ", "))

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var result VectorSearchResult
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
		results = []VectorSearchResult{}
	}

	return results, nil
}
