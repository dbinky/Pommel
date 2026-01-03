package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pommel-dev/pommel/internal/models"
)

// InsertFile inserts a file record and returns its ID.
// If the file already exists, it updates the existing record and returns its ID.
func (db *DB) InsertFile(ctx context.Context, path, contentHash, language string, size int64, modifiedAt time.Time) (int64, error) {
	// Try to update existing record first
	result, err := db.Exec(ctx, `
		UPDATE files
		SET content_hash = ?, size = ?, modified_at = ?, indexed_at = CURRENT_TIMESTAMP, language = ?
		WHERE path = ?
	`, contentHash, size, modifiedAt, language, path)
	if err != nil {
		return 0, fmt.Errorf("failed to update file: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		// File was updated, get its ID
		var id int64
		err := db.QueryRow(ctx, `SELECT id FROM files WHERE path = ?`, path).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to get file ID: %w", err)
		}
		return id, nil
	}

	// File doesn't exist, insert it
	result, err = db.Exec(ctx, `
		INSERT INTO files (path, content_hash, size, modified_at, language)
		VALUES (?, ?, ?, ?, ?)
	`, path, contentHash, size, modifiedAt, language)
	if err != nil {
		return 0, fmt.Errorf("failed to insert file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetFileIDByPath returns the file ID for the given path, or 0 if not found.
func (db *DB) GetFileIDByPath(ctx context.Context, path string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx, `SELECT id FROM files WHERE path = ?`, path).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get file ID: %w", err)
	}
	return id, nil
}

// DeleteFileByPath deletes a file record by path.
// This also cascades to delete associated chunks.
func (db *DB) DeleteFileByPath(ctx context.Context, path string) error {
	_, err := db.Exec(ctx, `DELETE FROM files WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// InsertChunk inserts a chunk record.
func (db *DB) InsertChunk(ctx context.Context, chunk *models.Chunk, fileID int64) error {
	_, err := db.Exec(ctx, `
		INSERT OR REPLACE INTO chunks (id, file_id, level, name, start_line, end_line, content, content_hash, parent_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, fileID, string(chunk.Level), chunk.Name, chunk.StartLine, chunk.EndLine, chunk.Content, chunk.ContentHash, chunk.ParentID)
	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}
	return nil
}

// DeleteChunksByFileID deletes all chunks for a file ID.
func (db *DB) DeleteChunksByFileID(ctx context.Context, fileID int64) error {
	_, err := db.Exec(ctx, `DELETE FROM chunks WHERE file_id = ?`, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}
	return nil
}

// DeleteChunksByFile deletes all chunks associated with a file path.
func (db *DB) DeleteChunksByFile(ctx context.Context, filePath string) error {
	_, err := db.Exec(ctx, `
		DELETE FROM chunks WHERE file_id IN (
			SELECT id FROM files WHERE path = ?
		)
	`, filePath)
	if err != nil {
		return fmt.Errorf("failed to delete chunks by file path: %w", err)
	}
	return nil
}

// GetChunkIDsByFile returns all chunk IDs for a given file path.
func (db *DB) GetChunkIDsByFile(ctx context.Context, filePath string) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT c.id FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE f.path = ?
	`, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunk IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan chunk ID: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunk IDs: %w", err)
	}

	return ids, nil
}

// GetChunkIDsByFileID returns all chunk IDs for a given file ID.
func (db *DB) GetChunkIDsByFileID(ctx context.Context, fileID int64) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT id FROM chunks WHERE file_id = ?`, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunk IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan chunk ID: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunk IDs: %w", err)
	}

	return ids, nil
}

// ClearAll deletes all data from all tables.
func (db *DB) ClearAll(ctx context.Context) error {
	// Delete from chunk_embeddings first (no FK constraints but good practice)
	if _, err := db.Exec(ctx, `DELETE FROM chunk_embeddings`); err != nil {
		return fmt.Errorf("failed to clear chunk_embeddings: %w", err)
	}

	// Delete from chunks (has FK to files)
	if _, err := db.Exec(ctx, `DELETE FROM chunks`); err != nil {
		return fmt.Errorf("failed to clear chunks: %w", err)
	}

	// Delete from files
	if _, err := db.Exec(ctx, `DELETE FROM files`); err != nil {
		return fmt.Errorf("failed to clear files: %w", err)
	}

	return nil
}

// ChunkCount returns the total number of chunks stored.
func (db *DB) ChunkCount(ctx context.Context) (int64, error) {
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count chunks: %w", err)
	}
	return count, nil
}

// FileCount returns the total number of files stored.
func (db *DB) FileCount(ctx context.Context) (int64, error) {
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM files`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count files: %w", err)
	}
	return count, nil
}

// IndexedFile represents a file in the index with its metadata.
type IndexedFile struct {
	Path       string
	ModifiedAt time.Time
}

// ListFiles returns all indexed files with their modification times.
func (db *DB) ListFiles(ctx context.Context) ([]IndexedFile, error) {
	rows, err := db.Query(ctx, `SELECT path, modified_at FROM files`)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []IndexedFile
	for rows.Next() {
		var f IndexedFile
		if err := rows.Scan(&f.Path, &f.ModifiedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		files = append(files, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	return files, nil
}

// GetChunkByID retrieves a chunk by its ID.
func (db *DB) GetChunkByID(ctx context.Context, id string) (*models.Chunk, error) {
	var chunk models.Chunk
	var parentID sql.NullString
	var filePath string
	var language string

	err := db.QueryRow(ctx, `
		SELECT c.id, f.path, f.language, c.start_line, c.end_line, c.level, c.name, c.content, c.content_hash, c.parent_id
		FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE c.id = ?
	`, id).Scan(&chunk.ID, &filePath, &language, &chunk.StartLine, &chunk.EndLine, &chunk.Level, &chunk.Name, &chunk.Content, &chunk.ContentHash, &parentID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	chunk.FilePath = filePath
	chunk.Language = language
	if parentID.Valid {
		chunk.ParentID = &parentID.String
	}

	return &chunk, nil
}

// GetChunksByIDs retrieves multiple chunks by their IDs.
func (db *DB) GetChunksByIDs(ctx context.Context, ids []string) ([]*models.Chunk, error) {
	if len(ids) == 0 {
		return []*models.Chunk{}, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT c.id, f.path, f.language, c.start_line, c.end_line, c.level, c.name, c.content, c.content_hash, c.parent_id
		FROM chunks c
		JOIN files f ON c.file_id = f.id
		WHERE c.id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		var parentID sql.NullString
		var filePath string
		var language string

		if err := rows.Scan(&chunk.ID, &filePath, &language, &chunk.StartLine, &chunk.EndLine, &chunk.Level, &chunk.Name, &chunk.Content, &chunk.ContentHash, &parentID); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		chunk.FilePath = filePath
		chunk.Language = language
		if parentID.Valid {
			chunk.ParentID = &parentID.String
		}
		chunks = append(chunks, &chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunks: %w", err)
	}

	return chunks, nil
}
